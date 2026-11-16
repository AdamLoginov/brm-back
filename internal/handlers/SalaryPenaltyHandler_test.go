// --------- новый код chat: проверка HTTP API штрафов
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
	"project/internal/middleware"
	"project/internal/models"
	"strings"
	"testing"
)

func TestSalaryPenaltyAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "penaltys.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.SalaryPenalty{}); err != nil {
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
	call := func(method, id, penaltyID, body string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		r.SetPathValue("penaltyId", penaltyID)
		w := httptest.NewRecorder()
		if authenticated {
			SalaryPenaltyHandler(w, r)
		} else {
			middleware.AuthMiddleware(SalaryPenaltyHandler)(w, r)
		}
		return w
	}
	id := fmt.Sprint(agreement.ID)
	body := fmt.Sprintf(`{"employee_card_id":%d,"date":"2026-01-10","amount":"10.05","comment":""}`, employee.ID)
	if w := call(http.MethodPost, id, "", body, false); w.Code != 401 {
		t.Fatal("anonymous write allowed")
	}
	if w := call(http.MethodGet, "bad", "", "", true); w.Code != 400 {
		t.Fatal("invalid ID allowed")
	}
	if w := call(http.MethodPost, id, "", strings.TrimSuffix(body, "}")+`,"is_settled":true}`, true); w.Code != 400 {
		t.Fatal("client can set settlement flag")
	}
	if w := call(http.MethodPost, id, "", body+body, true); w.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
	w := call(http.MethodPost, id, "", body, true)
	var penalty models.SalaryPenalty
	if err := json.Unmarshal(w.Body.Bytes(), &penalty); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || penalty.AmountKopecks != 1005 {
		t.Fatalf("create: %s", w.Body.String())
	}
	penaltyID := fmt.Sprint(penalty.ID)
	// --------- новый код chat: API статуса, дата и запрет ручного закрытия
	if penalty.Status != "active" || penalty.ClosedAt != nil {
		t.Fatal("new penalty status")
	}
	if w := call(http.MethodPatch, id, penaltyID, `{"status":"forgiven"}`, false); w.Code != 401 {
		t.Fatal("anonymous status change")
	}
	if w := call(http.MethodPatch, id, penaltyID, `{"status":"closed"}`, true); w.Code != 409 {
		t.Fatal("manual close allowed")
	}
	if w := call(http.MethodPost, id, penaltyID, strings.TrimSuffix(body, "}")+`,"status":"forgiven"}`, true); w.Code != 400 {
		t.Fatal("status through edit allowed")
	}
	for _, invalid := range []string{`{`, `{"status":"forgiven","closed_at":"2020-01-01"}`, `{"status":"forgiven"}{}`} {
		if w := call(http.MethodPatch, id, penaltyID, invalid, true); w.Code != 400 {
			t.Fatal("invalid status body allowed")
		}
	}
	w = call(http.MethodPatch, id, penaltyID, `{"status":"forgiven"}`, true)
	var forgiven models.SalaryPenalty
	if err := json.Unmarshal(w.Body.Bytes(), &forgiven); err != nil || w.Code != 200 || forgiven.Status != "forgiven" || forgiven.ClosedAt == nil {
		t.Fatalf("forgive: %s", w.Body.String())
	}
	w = call(http.MethodPatch, id, penaltyID, `{"status":"active"}`, true)
	var restored models.SalaryPenalty
	if err := json.Unmarshal(w.Body.Bytes(), &restored); err != nil || w.Code != 200 || restored.Status != "active" || restored.ClosedAt != nil {
		t.Fatalf("restore: %s", w.Body.String())
	}
	// --------- конец нового кода chat
	if w := call(http.MethodPost, id, penaltyID, strings.Replace(body, "10.05", "20.01", 1), true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(http.MethodGet, id, "", "", true)
	var rows []models.SalaryPenalty
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].AmountKopecks != 2001 {
		t.Fatalf("list: %s", w.Body.String())
	}
	if w := call(http.MethodDelete, id, penaltyID, "", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(http.MethodGet, id, "", "", true); w.Body.String() != "[]\n" {
		t.Fatal("deleted penalty listed")
	}
	if w := call(http.MethodDelete, id, penaltyID, "", true); w.Code != 404 {
		t.Fatal("missing penalty must return 404")
	}
}

// --------- конец нового кода chat
