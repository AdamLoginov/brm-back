// --------- новый код chat: проверка реальных маршрутов, CORS и API суточных
package app

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
	"testing"
	"time"
)

func TestDailyAllowanceRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "daily-api.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousKey := database.DB, config.JWTKey
	database.DB = db
	config.JWTKey = []byte("daily-allowance-router-test-key")
	t.Cleanup(func() { database.DB = previousDB; config.JWTKey = previousKey; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryPenalty{}, &models.DailyAllowanceAccrual{}, &models.DailyAllowancePayment{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Первый", Number: "1"}
	other := models.Agreement{Name: "Второй", Number: "2"}
	for _, row := range []*models.Agreement{&agreement, &other} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	yesterday := service.DailyAllowanceToday().AddDate(0, 0, -1)
	month := models.TimeSheetMonth{AgreementID: agreement.ID, Year: uint(yesterday.Year()), Month: uint(yesterday.Month()), DailyAllowanceKopecks: 50000, Employees: []models.EmployeeCard{employee}}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TimeSheet{AgreementID: agreement.ID, TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: yesterday.Format("2006-01-02"), Status: "1"}).Error; err != nil {
		t.Fatal(err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString(config.JWTKey)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter()
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost:5173")
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	base := fmt.Sprintf("/api/agreements/%d/daily-allowance", agreement.ID)
	path := base + "/payments"
	body := fmt.Sprintf(`{"request_key":"api-daily-idempotent-001","month_id":%d,"date":"%s","payments":[{"employee_card_id":%d,"amount":"1000"}]}`, month.ID, yesterday.Format("2006-01-02"), employee.ID)
	if w := call("POST", path, body, false); w.Code != 401 {
		t.Fatal("anonymous issue")
	}
	if w := call("OPTIONS", path, "", false); w.Code != 200 || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatal("preflight")
	}
	for _, payload := range []string{body + body, strings.TrimSuffix(body, "}") + `,"unexpected":1}`, strings.Replace(body, `"1000"`, `"750"`, 1)} {
		if w := call("POST", path, payload, true); w.Code != 400 {
			t.Fatal("invalid input accepted", w.Code, w.Body.String())
		}
	}
	w := call("POST", path, body, true)
	var payments []models.DailyAllowancePayment
	if err := json.Unmarshal(w.Body.Bytes(), &payments); err != nil || w.Code != 200 || len(payments) != 1 {
		t.Fatal(w.Body.String(), err)
	}
	if w := call("POST", path, body, true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call("GET", path, "", true)
	var history []models.DailyAllowancePayment
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history) != 1 {
		t.Fatal("duplicate issue", w.Body.String())
	}
	w = call("GET", fmt.Sprintf("%s?month_id=%d", base, month.ID), "", true)
	var table service.DailyAllowanceTable
	if err := json.Unmarshal(w.Body.Bytes(), &table); err != nil || w.Code != 200 || len(table.Employees) != 1 || table.Employees[0].BalanceKopecks != 50000 || table.Employees[0].Cells[yesterday.Day()-1].State != "paid" {
		t.Fatal("calendar", w.Body.String(), err)
	}
	paymentPath := fmt.Sprintf("%s/%d", path, payments[0].ID)
	foreign := fmt.Sprintf("/api/agreements/%d/daily-allowance/payments/%d", other.ID, payments[0].ID)
	update := fmt.Sprintf(`{"date":"%s","amount":"1500"}`, yesterday.Format("2006-01-02"))
	if w := call("PUT", foreign, update, true); w.Code != 404 {
		t.Fatal("cross agreement update")
	}
	if w := call("DELETE", foreign, "", true); w.Code != 404 {
		t.Fatal("cross agreement delete")
	}
	if w := call("PUT", paymentPath, update, true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	// Удаление месячного табеля удаляет только начисления, а не фактическую выдачу.
	if w := call("DELETE", fmt.Sprintf("/api/agreements/timesheetmonth/delete/%d", month.ID), "", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call("GET", path, "", true); w.Code != 200 || !strings.Contains(w.Body.String(), `"amount_kopecks":150000`) {
		t.Fatal("payment deleted with month", w.Body.String())
	}
	if w := call("PUT", paymentPath, update, true); w.Code != 200 {
		t.Fatal("history of deleted month", w.Body.String())
	}
	if w := call("DELETE", paymentPath, "", true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call("GET", path, "", true); w.Body.String() != "[]\n" {
		t.Fatal("payment not deleted", w.Body.String())
	}
}

// --------- конец нового кода chat
