// --------- новый код chat: проверка HTTP API и удаления начислений с табелем
package handlers

import (
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"strconv"
	"testing"
)

func TestSalaryAccrualAPIAndMonthDeletion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "api.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	previousDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previousDB; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.DailyAllowanceAccrual{}); err != nil { // новый код chat: авансы в тестовой БД
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор", Number: "1"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", SalaryRate: "5000", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	month := models.TimeSheetMonth{AgreementID: agreement.ID, Year: 2026, Month: 1, Employees: []models.EmployeeCard{employee}}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TimeSheet{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Status: "12"}).Error; err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, method, agreementID, monthID string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", nil)
		r.SetPathValue("id", agreementID)
		r.SetPathValue("monthId", monthID)
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	aID, mID := strconv.Itoa(int(agreement.ID)), strconv.Itoa(int(month.ID))
	if w := call(middleware.AuthMiddleware(GetSalaryAccrualHandler), "GET", aID, mID); w.Code != 401 {
		t.Fatal("anonymous access allowed")
	}
	if w := call(GetSalaryAccrualHandler, "GET", "bad", mID); w.Code != 400 {
		t.Fatal("bad ID must return 400")
	}
	if w := call(GetSalaryAccrualHandler, "GET", "99999", mID); w.Code != 404 {
		t.Fatal("unknown agreement must return 404")
	}
	before := call(GetSalaryAccrualHandler, "GET", aID, mID)
	if before.Code != 200 || before.Body.String() != "{\"accruals\":[],\"total_kopecks\":0,\"total_advances_kopecks\":0,\"total_payable_kopecks\":0,\"total_penalties_kopecks\":0}\n" { // новый код chat: итоги с авансами
		t.Fatalf("empty response: %s", before.Body.String())
	}
	// --------- новый код chat: API возвращает штрафы и остаток за месяц
	if err := db.Create(&models.SalaryPenalty{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-31", AmountKopecks: 12345}).Error; err != nil {
		t.Fatal(err)
	}
	response := call(CalculateSalaryAccrualHandler, "POST", aID, mID)
	var body struct {
		Accruals []struct {
			models.SalaryAccrual
			PenaltiesKopecks int64 `json:"penalties_kopecks"`
			PayableKopecks   int64 `json:"payable_kopecks"`
		} `json:"accruals"`
		TotalKopecks          int64 `json:"total_kopecks"`
		TotalPenaltiesKopecks int64 `json:"total_penalties_kopecks"`
		TotalPayableKopecks   int64 `json:"total_payable_kopecks"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(body.Accruals) != 1 || body.TotalKopecks != 500000 || body.TotalPenaltiesKopecks != 12345 || body.TotalPayableKopecks != 487655 || body.Accruals[0].PenaltiesKopecks != 12345 || body.Accruals[0].PayableKopecks != 487655 {
		t.Fatalf("calculation response: %s", response.Body.String())
	}
	deleted := call(DeleteTimeSheetMonthHandler, "DELETE", mID, "")
	if deleted.Code != 200 {
		t.Fatalf("delete: %s", deleted.Body.String())
	}
	var count int64
	db.Model(&models.SalaryAccrual{}).Count(&count)
	if count != 0 {
		t.Fatal("month handler left live accruals")
	}
	if w := call(CalculateSalaryAccrualHandler, "POST", aID, mID); w.Code != 404 {
		t.Fatal("deleted month must return 404")
	}
}

// --------- конец нового кода chat
