// --------- новый код chat: премии по месяцу, статусу и договору
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func TestSalaryBonuses(t *testing.T) {
	db := salaryTestDB(t)
	agreement, month, employee := salaryFixture(t, db)
	if err := db.AutoMigrate(&models.SalaryBonus{}); err != nil {
		t.Fatal(err)
	}
	input := SalaryBonusInput{EmployeeCardID: employee.ID, Month: month.Month, Year: month.Year, Amount: "1234,56", Comment: " Спасибо "}
	bonus, err := SaveSalaryBonus(db, agreement.ID, 0, input)
	if err != nil || bonus.IsPaid || bonus.AmountKopecks != 123456 || bonus.Comment != "Спасибо" {
		t.Fatalf("create: %+v %v", bonus, err)
	}
	summary := SalaryExpenseSummary{}
	summary.TotalPayableKopecks = 100000
	row := SalaryAccrualWithExpenses{}
	row.EmployeeCardID = employee.ID
	row.PayableKopecks = 100000
	summary.Accruals = []SalaryAccrualWithExpenses{row}
	check := func(want int64) {
		t.Helper()
		result, err := AddSalaryBonuses(db, agreement.ID, month.ID, summary)
		if err != nil {
			t.Fatal(err)
		}
		if result.TotalBonusesKopecks != want || result.TotalPayableKopecks != 100000+want || result.Accruals[0].BonusesKopecks != want || result.Accruals[0].PayableKopecks != 100000+want {
			t.Fatalf("summary: %+v", result)
		}
		if summary.Accruals[0].PayableKopecks != 100000 {
			t.Fatal("input mutated")
		}
	}
	check(123456)
	other := input
	other.Year++
	if _, err := SaveSalaryBonus(db, agreement.ID, 0, other); err != nil {
		t.Fatal(err)
	}
	other = input
	other.Month = month.Month%12 + 1
	if _, err := SaveSalaryBonus(db, agreement.ID, 0, other); err != nil {
		t.Fatal(err)
	}
	secondAgreement := models.Agreement{Name: "Другой договор"}
	db.Create(&secondAgreement)
	db.Create(&models.SalaryBonus{AgreementID: secondAgreement.ID, EmployeeCardID: employee.ID, Month: month.Month, Year: month.Year, AmountKopecks: 999})
	check(123456)
	if _, err := SetSalaryBonusPaid(db, agreement.ID, bonus.ID, true); err != nil {
		t.Fatal(err)
	}
	check(0)
	input.Amount = "2000.01"
	updated, err := SaveSalaryBonus(db, agreement.ID, bonus.ID, input)
	if err != nil || !updated.IsPaid || updated.AmountKopecks != 200001 {
		t.Fatal("paid edit", err)
	}
	if _, err := SetSalaryBonusPaid(db, agreement.ID, bonus.ID, false); err != nil {
		t.Fatal(err)
	}
	check(200001)
	if _, err := SetSalaryBonusPaid(db, secondAgreement.ID, bonus.ID, true); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross-contract status", err)
	}
	if err := DeleteSalaryBonus(db, secondAgreement.ID, bonus.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross-contract delete", err)
	}
	db.Model(&employee).Update("agreement_id", secondAgreement.ID)
	if _, err := SaveSalaryBonus(db, agreement.ID, 0, input); !errors.Is(err, ErrSalaryBonusInput) {
		t.Fatal("transferred employee allowed", err)
	}
	if _, err := SaveSalaryBonus(db, agreement.ID, bonus.ID, input); err != nil {
		t.Fatal("edit transferred", err)
	}
	if err := DeleteSalaryBonus(db, agreement.ID, bonus.ID); err != nil {
		t.Fatal(err)
	}
	check(0)
	for _, invalid := range []SalaryBonusInput{
		{EmployeeCardID: employee.ID, Month: 0, Year: 2026, Amount: "1"},
		{EmployeeCardID: employee.ID, Month: 13, Year: 2026, Amount: "1"},
		{EmployeeCardID: employee.ID, Month: 1, Year: 0, Amount: "1"},
		{EmployeeCardID: employee.ID, Month: 1, Year: 2026, Amount: "0"},
		{EmployeeCardID: employee.ID, Month: 1, Year: 2026, Amount: "-1"},
		{EmployeeCardID: employee.ID, Month: 1, Year: 2026, Amount: "1.001"},
	} {
		if _, err := SaveSalaryBonus(db, agreement.ID, 0, invalid); !errors.Is(err, ErrSalaryBonusInput) {
			t.Fatal("invalid input", err)
		}
	}
}

// --------- конец нового кода chat
