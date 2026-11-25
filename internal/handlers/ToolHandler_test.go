// --------- новый код chat: назначение между договорами, даты и перевод сотрудника
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

func TestToolAssignmentAndInspection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tools.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Tool{}, &models.Ppe{}); err != nil { // новый код chat: перевод снимает также СИЗ
		t.Fatal(err)
	}
	a, b := models.Agreement{Name: "A"}, models.Agreement{Name: "B"}
	db.Create(&a)
	db.Create(&b)
	employee := models.EmployeeCard{Name: "Иван", AgreementID: b.ID}
	db.Create(&employee)
	call := func(handler http.HandlerFunc, id uint, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", fmt.Sprint(id))
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != want {
			t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
		}
		return w
	}
	payload := func(date, next string) string {
		return fmt.Sprintf(`{"agreement_id":%d,"employee_card_id":%d,"name":"Дрель","status":"good","inspection_date":%q,"next_inspection_date":%q}`, a.ID, employee.ID, date, next)
	}
	call(CreateToolHandler, 0, payload("2026-02-30", ""), 400)
	call(CreateToolHandler, 0, payload("2026-11-25", "2026-11-24"), 400)
	w := call(CreateToolHandler, 0, payload("2026-11-20", "2026-11-25"), 200)
	var tool models.Tool
	if err := json.Unmarshal(w.Body.Bytes(), &tool); err != nil {
		t.Fatal(err)
	}
	if tool.EmployeeCardID == nil || *tool.EmployeeCardID != employee.ID || tool.NextInspectionDate != "2026-11-25" {
		t.Fatalf("saved: %+v", tool)
	}
	w = call(GetAgreementAllToolHandler, a.ID, "", 200)
	var list []models.Tool
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].EmployeeCard == nil || list[0].EmployeeCard.Name != "Иван" {
		t.Fatalf("list: %s", w.Body.String())
	}
	call(PutAgreementEmployeeCard, employee.ID, fmt.Sprintf(`{"agreement_id":%d}`, b.ID), 200)
	var unchanged models.Tool
	db.First(&unchanged, tool.ID)
	if unchanged.EmployeeCardID == nil {
		t.Fatal("same agreement must retain assignment")
	}
	call(PutAgreementEmployeeCard, employee.ID, fmt.Sprintf(`{"agreement_id":%d}`, a.ID), 200)
	var detached models.Tool
	db.First(&detached, tool.ID)
	if detached.EmployeeCardID != nil {
		t.Fatal("transfer must detach")
	}
	call(UpdateToolHandler, tool.ID, payload("", ""), 200)
	var edited models.Tool
	db.First(&edited, tool.ID)
	if edited.EmployeeCardID == nil || edited.InspectionDate != "" || edited.NextInspectionDate != "" {
		t.Fatalf("edit: %+v", edited)
	}
	call(UpdateToolHandler, tool.ID, fmt.Sprintf(`{"agreement_id":%d,"name":"Дрель","status":"new","employee_card_id":null}`, a.ID), 200)
	var cleared models.Tool
	db.First(&cleared, tool.ID)
	if cleared.EmployeeCardID != nil {
		t.Fatal("manual detach failed")
	}
	db.Model(&employee).Update("status", true)
	call(UpdateToolHandler, tool.ID, payload("", ""), 400)
}

// --------- конец нового кода chat
