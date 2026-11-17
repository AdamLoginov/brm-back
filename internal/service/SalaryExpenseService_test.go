// --------- новый код chat: возмещение расходов без смешивания договоров
package service

import (
	"project/internal/models"
	"testing"
)

func TestSalaryExpenses(t *testing.T) {
	db := salaryTestDB(t)
	agreement, _, employee := salaryFixture(t, db)
	if err := db.AutoMigrate(&models.Expenses{}); err != nil {
		t.Fatal(err)
	}
	other := models.Agreement{Name: "Другой"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	rows := []models.Expenses{
		{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 100.25},
		{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 0.10},
		{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 500, IsReimbursed: true},
		{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "company", Price: 1000},
		{AgreementId: other.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 2000},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	// Расход из прошлого года тоже возмещается.
	if err := db.Model(&rows[0]).Update("created_at", "2020-01-01 00:00:00").Error; err != nil {
		t.Fatal(err)
	}
	summary := SalaryDeductionSummary{SalaryPenaltySummary: SalaryPenaltySummary{TotalPayableKopecks: 100000}, Accruals: []SalaryAccrualWithDeduction{{SalaryAccrualWithPenalties: SalaryAccrualWithPenalties{SalaryAccrualWithAdvances: SalaryAccrualWithAdvances{SalaryAccrual: models.SalaryAccrual{EmployeeCardID: employee.ID}, PayableKopecks: 100000}}}}}
	assert := func(want int64) {
		t.Helper()
		result, err := AddSalaryExpenses(db, agreement.ID, summary)
		if err != nil {
			t.Fatal(err)
		}
		if result.TotalExpensesKopecks != want || result.TotalPayableKopecks != 100000+want || result.Accruals[0].ExpensesKopecks != want || result.Accruals[0].PayableKopecks != 100000+want {
			t.Fatalf("unexpected: %+v", result)
		}
		if summary.Accruals[0].PayableKopecks != 100000 {
			t.Fatal("input changed")
		}
	}
	assert(10035)
	db.Model(&rows[0]).Update("is_reimbursed", true)
	assert(10)
	db.Model(&rows[0]).Update("is_reimbursed", false)
	assert(10035)
	db.Delete(&rows[0])
	assert(10)
	empty, err := AddSalaryExpenses(db, agreement.ID, SalaryDeductionSummary{})
	if err != nil || len(empty.Accruals) != 0 || empty.TotalExpensesKopecks != 0 {
		t.Fatal("empty summary", err)
	}
}

// --------- конец нового кода chat
