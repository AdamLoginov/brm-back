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
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.DailyAllowanceAccrual{}, &models.Expenses{}, &models.SalaryBonus{}, &models.SalaryDebt{}, &models.SalaryApproval{}, &models.SalaryPaymentDraft{}, &models.SalaryApprovalBatch{}); err != nil { // новый код chat: авансы в тестовой БД
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
	// --------- новый код chat: проверка пустого ответа с новым итогом вычета
	var empty map[string]json.RawMessage
	if err := json.Unmarshal(before.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if before.Code != 200 || string(empty["accruals"]) != "[]" || string(empty["total_deduction_kopecks"]) != "0" || string(empty["total_payable_kopecks"]) != "0" {
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
			PenaltiesKopecks     int64 `json:"penalties_kopecks"`
			PayableKopecks       int64 `json:"payable_kopecks"`
			OfficiallyEmployed   bool  `json:"officially_employed"`    // новый код chat
			MonthlySalaryKopecks int64 `json:"monthly_salary_kopecks"` // новый код chat
			DeductionKopecks     int64 `json:"deduction_kopecks"`      // новый код chat
		} `json:"accruals"`
		TotalKopecks          int64 `json:"total_kopecks"`
		TotalPenaltiesKopecks int64 `json:"total_penalties_kopecks"`
		TotalPayableKopecks   int64 `json:"total_payable_kopecks"`
		TotalDeductionKopecks int64 `json:"total_deduction_kopecks"` // новый код chat
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(body.Accruals) != 1 || body.TotalKopecks != 500000 || body.TotalPenaltiesKopecks != 12345 || body.TotalPayableKopecks != 487655 || body.Accruals[0].PenaltiesKopecks != 12345 || body.Accruals[0].PayableKopecks != 487655 {
		t.Fatalf("calculation response: %s", response.Body.String())
	}
	// --------- новый код chat: 13% берутся от 35000, а не от начисленных 5000
	if err := db.Model(&employee).Updates(map[string]any{"officially_employed": true, "monthly_salary_kopecks": 3500000}).Error; err != nil {
		t.Fatal(err)
	}
	response = call(CalculateSalaryAccrualHandler, "POST", aID, mID)
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || len(body.Accruals) != 1 || !body.Accruals[0].OfficiallyEmployed || body.Accruals[0].MonthlySalaryKopecks != 3500000 || body.Accruals[0].DeductionKopecks != 455000 || body.TotalDeductionKopecks != 455000 || body.TotalPayableKopecks != 32655 || body.Accruals[0].PayableKopecks != 32655 {
		t.Fatalf("deduction response: %s", response.Body.String())
	}
	// --------- конец нового кода chat
	// --------- новый код chat: личный расход увеличивает итог, возмещённый исключается
	if err := db.AutoMigrate(&models.Expenses{}, &models.SalaryBonus{}, &models.SalaryDebt{}, &models.SalaryApproval{}, &models.SalaryPaymentDraft{}, &models.SalaryApprovalBatch{}); err != nil {
		t.Fatal(err)
	}
	expense := models.Expenses{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 100.25}
	if err := db.Create(&expense).Error; err != nil {
		t.Fatal(err)
	}
	response = call(CalculateSalaryAccrualHandler, "POST", aID, mID)
	var expenseBody struct {
		TotalExpensesKopecks int64 `json:"total_expenses_kopecks"`
		TotalPayableKopecks  int64 `json:"total_payable_kopecks"`
		Accruals             []struct {
			ExpensesKopecks int64 `json:"expenses_kopecks"`
			PayableKopecks  int64 `json:"payable_kopecks"`
		} `json:"accruals"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &expenseBody); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || expenseBody.TotalExpensesKopecks != 10025 || expenseBody.TotalPayableKopecks != 42680 || len(expenseBody.Accruals) != 1 || expenseBody.Accruals[0].ExpensesKopecks != 10025 || expenseBody.Accruals[0].PayableKopecks != 42680 {
		t.Fatalf("expenses: %s", response.Body.String())
	}
	// --------- конец нового кода chat
	// --------- новый код chat: премия прибавляется только пока не выплачена
	bonus := models.SalaryBonus{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Month: month.Month, Year: month.Year, AmountKopecks: 10000}
	if err := db.Create(&bonus).Error; err != nil {
		t.Fatal(err)
	}
	for _, paid := range []bool{false, true} {
		if err := db.Model(&bonus).Update("is_paid", paid).Error; err != nil {
			t.Fatal(err)
		}
		res := call(CalculateSalaryAccrualHandler, "POST", aID, mID)
		var result struct {
			TotalBonusesKopecks int64 `json:"total_bonuses_kopecks"`
			TotalPayableKopecks int64 `json:"total_payable_kopecks"`
			Accruals            []struct {
				BonusesKopecks int64 `json:"bonuses_kopecks"`
			} `json:"accruals"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		want := int64(10000)
		if paid {
			want = 0
		}
		if res.Code != 200 || result.TotalBonusesKopecks != want || result.TotalPayableKopecks != 42680+want || len(result.Accruals) != 1 || result.Accruals[0].BonusesKopecks != want {
			t.Fatalf("bonus totals: %s", res.Body.String())
		}
	}
	// --------- конец нового кода chat
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
