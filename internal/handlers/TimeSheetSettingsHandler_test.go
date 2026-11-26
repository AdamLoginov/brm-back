// --------- новый код chat: настройки табеля через API и каскадное удаление штрафов
package handlers

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strings"
	"testing"
)

func TestTimeSheetSettingsAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryPenalty{}, &models.DailyAllowanceAccrual{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор", Number: "1"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, body, id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	create := fmt.Sprintf(`{"month":1,"year":2026,"agreement_id":%d,"employee_card_id":[%d],"shift_hours":"11.5","absence_penalty":"123.45","daily_allowance":"700.55"}`, agreement.ID, employee.ID)
	if w := call(CreateTimeSheetMonthHandler, strings.Replace(create, `"11.5"`, `"0"`, 1), ""); w.Code != 400 {
		t.Fatal("invalid shift accepted", w.Body.String())
	}
	w := call(CreateTimeSheetMonthHandler, create, "")
	var month models.TimeSheetMonth
	if err := json.Unmarshal(w.Body.Bytes(), &month); err != nil || w.Code != 200 || month.ShiftHours != "11.5" || month.AbsencePenaltyKopecks != 12345 || month.DailyAllowanceKopecks != 70055 {
		t.Fatalf("create: %s", w.Body.String())
	}
	rows := fmt.Sprintf(`[{"time_sheet_month_id":%d,"employee_card_id":%d,"date":"2026-01-15","status":"П"}]`, month.ID, employee.ID)
	w = call(CreateTimeSheetHandler, rows, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var entries []models.TimeSheet
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	check := func(amount int64) {
		t.Helper()
		var penalties []models.SalaryPenalty
		if err := db.Where("source_time_sheet_month_id = ?", month.ID).Find(&penalties).Error; err != nil {
			t.Fatal(err)
		}
		if amount == 0 {
			if len(penalties) != 0 {
				t.Fatal("automatic penalty remains")
			}
			return
		}
		if len(penalties) != 1 || penalties[0].AmountKopecks != amount {
			t.Fatalf("penalties: %+v", penalties)
		}
	}
	check(12345)
	update := fmt.Sprintf(`{"month":1,"year":2026,"timesheet_month_id":%d,"employee_card_id":[%d],"shift_hours":"9.5","absence_penalty":"500"}`, month.ID, employee.ID)
	if w := call(UpdateTimeSheetMonthHandler, update, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(50000)
	// --------- новый код chat: отсутствие поля сохраняет значение, изменение и ноль записываются
	if err := db.First(&month, month.ID).Error; err != nil {
		t.Fatal(err)
	}
	if month.DailyAllowanceKopecks != 70055 {
		t.Fatal("omitted allowance reset")
	}
	for _, item := range []struct {
		value string
		want  int64
	}{{"125.10", 12510}, {"0", 0}} {
		payload := strings.TrimSuffix(update, "}") + `,"daily_allowance":"` + item.value + `"}`
		if w := call(UpdateTimeSheetMonthHandler, payload, ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if err := db.First(&month, month.ID).Error; err != nil {
			t.Fatal(err)
		}
		if month.DailyAllowanceKopecks != item.want {
			t.Fatal("allowance not saved")
		}
	}
	for _, value := range []string{"-1", "1.234", "abc", "999999999999999999999"} {
		payload := strings.TrimSuffix(update, "}") + `,"daily_allowance":"` + value + `"}`
		if w := call(UpdateTimeSheetMonthHandler, payload, ""); w.Code != 400 {
			t.Fatal("invalid allowance accepted", value)
		}
	}
	// --------- конец нового кода chat

	if w := call(UpdateTimeSheetHandler, fmt.Sprintf(`[{"ID":%d,"status":"Б"}]`, entries[0].ID), ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(0)
	if w := call(UpdateTimeSheetHandler, fmt.Sprintf(`[{"ID":%d,"status":"П"}]`, entries[0].ID), ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(50000)
	if w := call(UpdateTimeSheetMonthHandler, strings.Replace(update, fmt.Sprintf(`[%d]`, employee.ID), `[]`, 1), ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(50000) // новый код chat: скрытие сотрудника сохраняет отметки и штрафы
	if w := call(UpdateTimeSheetMonthHandler, update, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(50000)
	manual := models.SalaryPenalty{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-15", AmountKopecks: 100}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.SalaryPenalty{}).Where("source_time_sheet_month_id = ?", month.ID).Update("status", "closed").Error; err != nil {
		t.Fatal(err)
	}
	if w := call(DeleteTimeSheetMonthHandler, "", fmt.Sprint(month.ID)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	check(0)
	if err := db.First(&manual, manual.ID).Error; err != nil {
		t.Fatal("manual penalty deleted", err)
	}
	var count int64
	db.Model(&models.TimeSheet{}).Where("time_sheet_month_id = ?", month.ID).Count(&count)
	if count != 0 {
		t.Fatal("entries left after deleting month")
	}
}

// --------- конец нового кода chat
