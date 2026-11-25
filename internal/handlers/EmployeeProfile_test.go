// --------- новый код chat: справочник, связи и несколько телефонов
package handlers

import (
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"testing"
)

func TestEmployeeProfileAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "profile.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.EmployeeCard{}, &models.Tool{}, &models.Ppe{}); err != nil { // новый код chat: инструменты и СИЗ
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, id, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", id)
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != status {
			t.Fatalf("status=%d want=%d body=%s", res.Code, status, res.Body.String())
		}
		return res
	}
	created := call(SaveEmployeeSpecialization, "", `{"name":"  Монтажник  "}`, 200)
	var spec models.EmployeeSpecialization
	if err := json.Unmarshal(created.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(int(spec.ID))
	call(SaveEmployeeSpecialization, "", `{"name":"монтажник"}`, 409)
	call(SaveEmployeeSpecialization, "", `{"name":" "}`, 400)
	call(CreateEmployeeCardHandler, "", `{"name":"Иван","phone":"старый","phones":[" +7 111 ","+7 222"," "],"specialization_ids":[`+sid+`,`+sid+`]}`, 200)
	var employee models.EmployeeCard
	if err := db.Preload("Specializations").First(&employee).Error; err != nil {
		t.Fatal(err)
	}
	eid := strconv.Itoa(int(employee.ID))
	if len(employee.Phones) != 2 || employee.Phones[0] != "+7 111" || employee.Phone == nil || *employee.Phone != "+7 111" || len(employee.Specializations) != 1 {
		t.Fatalf("saved: %+v", employee)
	}
	call(DeleteEmployeeSpecialization, sid, "", 409)
	call(SaveEmployeeSpecialization, sid, `{"name":"Монтажник оборудования"}`, 200)
	var renamed models.EmployeeCard
	db.Preload("Specializations").First(&renamed, employee.ID)
	if len(renamed.Specializations) != 1 || renamed.Specializations[0].Name != "Монтажник оборудования" {
		t.Fatalf("renamed: %+v", renamed)
	}
	// Отсутствующий ключ связей сохраняет назначения при обновлении старым клиентом.
	call(UpdateEmployeeCardHandler, eid, `{"name":"Иван","phone":"+7 333"}`, 200)
	var legacy models.EmployeeCard
	db.Preload("Specializations").First(&legacy, employee.ID)
	if len(legacy.Specializations) != 1 || len(legacy.Phones) != 1 || legacy.Phones[0] != "+7 333" {
		t.Fatalf("legacy: %+v", legacy)
	}
	// Несуществующий справочник не должен частично сохранить карточку.
	input := models.EmployeeCard{Name: "Не сохранять", SpecializationIDs: []uint{999999}}
	if err := saveEmployeeProfile(&input, eid); err == nil {
		t.Fatal("invalid specialization accepted")
	}
	var unchanged models.EmployeeCard
	db.First(&unchanged, employee.ID)
	if unchanged.Name != "Иван" {
		t.Fatal("partial update")
	}
	call(UpdateEmployeeCardHandler, eid, `{"name":"Иван","phones":[],"specialization_ids":[]}`, 200)
	var cleared models.EmployeeCard
	db.Preload("Specializations").First(&cleared, employee.ID)
	if len(cleared.Specializations) != 0 || len(cleared.Phones) != 0 || cleared.Phone != nil {
		t.Fatalf("clear: %+v", cleared)
	}
	call(DeleteEmployeeSpecialization, sid, "", 204)
	call(DeleteEmployeeSpecialization, sid, "", 404)
	call(CreateEmployeeCardHandler, "", `{"name":"Анна","phone":"+7 444"}`, 200)
	var oldPhone models.EmployeeCard
	db.Where("name = ?", "Анна").First(&oldPhone)
	if len(oldPhone.Phones) != 1 || oldPhone.Phones[0] != "+7 444" {
		t.Fatal("old phone lost")
	}
}

// --------- конец нового кода chat

// --------- новый код chat: увольнение сбрасывает только текущее закрепление
func TestDismissedEmployeeAgreement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dismissal.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.Tool{}, &models.Ppe{}); err != nil { // новый код chat: очистка табеля, инструментов и СИЗ
		t.Fatal(err)
	}
	agreement := models.Agreement{}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	month := models.TimeSheetMonth{}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&employee).Association("TimeSheetMonths").Append(&month); err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(int(employee.ID))
	call := func(handler http.HandlerFunc, body string, want int) {
		t.Helper()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", id)
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != want {
			t.Fatalf("status=%d want=%d body=%s", res.Code, want, res.Body.String())
		}
	}
	aid := strconv.Itoa(int(agreement.ID))
	// новый код chat: увольнение снимает закрепление инструмента
	tool := models.Tool{Name: "Дрель", AgreementID: agreement.ID, EmployeeCardID: &employee.ID}
	ppe := models.Ppe{Name: "Куртка", AgreementID: agreement.ID, EmployeeCardID: &employee.ID} // новый код chat
	if err := db.Create(&ppe).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&tool).Error; err != nil {
		t.Fatal(err)
	}
	// --------- новый код chat: дата сохраняется и проверяется
	call(UpdateEmployeeCardHandler, `{"name":"Иван","status":true,"dismissal_date":"2026-02-30"}`, 400)
	call(UpdateEmployeeCardHandler, `{"name":"Иван","status":true}`, 400)
	call(UpdateEmployeeCardHandler, `{"name":"Иван","status":true,"dismissal_date":"2026-11-20","agreement_id":`+aid+`}`, 200)
	var saved models.EmployeeCard
	db.First(&saved, employee.ID)
	if !saved.Status || saved.AgreementID != 0 || saved.DismissalDate != "2026-11-20" {
		t.Fatalf("dismissed: %+v", saved)
	}
	var detachedTool models.Tool // новый код chat
	if err := db.First(&detachedTool, tool.ID).Error; err != nil {
		t.Fatal(err)
	}
	if detachedTool.EmployeeCardID != nil {
		t.Fatal("dismissal kept tool assignment")
	}
	var detachedPpe models.Ppe // новый код chat: увольнение снимает и СИЗ
	if err := db.First(&detachedPpe, ppe.ID).Error; err != nil {
		t.Fatal(err)
	}
	if detachedPpe.EmployeeCardID != nil {
		t.Fatal("dismissal kept PPE assignment")
	}
	if count := db.Model(&saved).Association("TimeSheetMonths").Count(); count != 1 {
		t.Fatal("historical timesheet link lost")
	}
	call(PutAgreementEmployeeCard, `{"agreement_id":`+aid+`}`, 400)
	call(PutAgreementEmployeeCard, `{"agreement_id":0}`, 200)
	call(CreateEmployeeCardHandler, `{"name":"Анна","status":true,"dismissal_date":"2026-11-21","agreement_id":`+aid+`}`, 200)
	var created models.EmployeeCard
	db.Where("name = ?", "Анна").First(&created)
	if !created.Status || created.AgreementID != 0 || created.DismissalDate != "2026-11-21" {
		t.Fatal("dismissed create kept agreement")
	}
	call(UpdateEmployeeCardHandler, `{"name":"Иван","status":false,"dismissal_date":"2026-11-20","agreement_id":0}`, 200)
	call(PutAgreementEmployeeCard, `{"agreement_id":`+aid+`}`, 200)
	var restored models.EmployeeCard
	db.First(&restored, employee.ID)
	if restored.Status || restored.AgreementID != agreement.ID || restored.DismissalDate != "" {
		t.Fatal("active employee assignment failed")
	}
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
