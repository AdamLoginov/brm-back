// --------- новый код chat: проверка HTTP API алиментов
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

func TestSalaryAlimonyAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "alimonys.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	// новый код chat: история видимости общих алиментов
	if err := db.AutoMigrate(&models.SalaryAlimonyEmployeeHistory{}, &models.Agreement{}, &models.EmployeeCard{}, &models.SalaryAlimony{}, &models.TimeSheetMonth{}, &models.SalaryAccrual{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор", Number: "1"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID, HasAlimony: true}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	call := func(method, id, alimonyID, body string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		r.SetPathValue("alimonyId", alimonyID)
		w := httptest.NewRecorder()
		if authenticated {
			SalaryAlimonyHandler(w, r)
		} else {
			middleware.AuthMiddleware(SalaryAlimonyHandler)(w, r)
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
	var alimony models.SalaryAlimony
	if err := json.Unmarshal(w.Body.Bytes(), &alimony); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || alimony.AmountKopecks != 1005 || alimony.IsSettled {
		t.Fatalf("create: %s", w.Body.String())
	}
	alimonyID := fmt.Sprint(alimony.ID)
	// новый код chat: другой договор не открывает записи постороннего сотрудника
	other := models.Agreement{Name: "Посторонний договор"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	otherID := fmt.Sprint(other.ID)
	if w := call(http.MethodGet, otherID, "", "", true); w.Code != 200 || w.Body.String() != "[]\n" {
		t.Fatal("unrelated records exposed", w.Body.String())
	}
	if w := call(http.MethodPost, otherID, alimonyID, body, true); w.Code != 404 {
		t.Fatal("unrelated update allowed", w.Code)
	}
	if w := call(http.MethodDelete, otherID, alimonyID, "", true); w.Code != 404 {
		t.Fatal("unrelated delete allowed", w.Code)
	}
	// Перевод до создания табеля не лишает старую вкладку истории.
	if err := db.Model(&employee).Update("agreement_id", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if w := call(http.MethodGet, id, "", "", true); w.Code != 200 || w.Body.String() == "[]\n" {
		t.Fatal("history lost after transfer", w.Body.String())
	}
	if w := call(http.MethodPost, id, alimonyID, strings.Replace(body, "10.05", "20.01", 1), true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(http.MethodGet, id, "", "", true)
	var rows []models.SalaryAlimony
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0].AmountKopecks != 2001 {
		t.Fatalf("list: %s", w.Body.String())
	}
	if w := call(http.MethodDelete, id, alimonyID, "", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(http.MethodGet, id, "", "", true); w.Body.String() != "[]\n" {
		t.Fatal("deleted alimony listed")
	}
	if w := call(http.MethodDelete, id, alimonyID, "", true); w.Code != 404 {
		t.Fatal("missing alimony must return 404")
	}
}

// --------- конец нового кода chat
