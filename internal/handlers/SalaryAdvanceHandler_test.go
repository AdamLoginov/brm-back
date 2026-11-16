// --------- новый код chat: проверка HTTP API авансов
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

func TestSalaryAdvanceAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "advances.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.SalaryAdvance{}); err != nil {
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
	call := func(method, id, advanceID, body string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		r.SetPathValue("advanceId", advanceID)
		w := httptest.NewRecorder()
		if authenticated {
			SalaryAdvanceHandler(w, r)
		} else {
			middleware.AuthMiddleware(SalaryAdvanceHandler)(w, r)
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
	var advance models.SalaryAdvance
	if err := json.Unmarshal(w.Body.Bytes(), &advance); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || advance.AmountKopecks != 1005 || advance.IsSettled {
		t.Fatalf("create: %s", w.Body.String())
	}
	advanceID := fmt.Sprint(advance.ID)
	if w := call(http.MethodPost, id, advanceID, strings.Replace(body, "10.05", "20.01", 1), true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(http.MethodGet, id, "", "", true)
	var rows []models.SalaryAdvance
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].AmountKopecks != 2001 {
		t.Fatalf("list: %s", w.Body.String())
	}
	if w := call(http.MethodDelete, id, advanceID, "", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(http.MethodGet, id, "", "", true); w.Body.String() != "[]\n" {
		t.Fatal("deleted advance listed")
	}
	if w := call(http.MethodDelete, id, advanceID, "", true); w.Code != 404 {
		t.Fatal("missing advance must return 404")
	}
}

// --------- конец нового кода chat
