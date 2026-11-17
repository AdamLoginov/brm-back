// --------- новый код chat: ручные долги, стороны долга и изоляция договоров
package service

import (
	"errors"
	"project/internal/models"
	"testing"
)

func TestSalaryDebtOperations(t *testing.T) {
	db, a, m, e := approvalFixture(t)
	input := SalaryDebtInput{EmployeeCardID: e.ID, Date: "2025-01-01", Side: "company", Amount: "10.25", Comment: " Долг "}
	debt, err := SaveSalaryDebt(db, a.ID, 0, input)
	if err != nil || debt.IsClosed || debt.AmountKopecks != 1025 || debt.Comment != "Долг" {
		t.Fatal("create", err)
	}
	input.Side = "employee"
	input.Amount = "5"
	employeeDebt, err := SaveSalaryDebt(db, a.ID, 0, input)
	if err != nil {
		t.Fatal(err)
	}
	check := func(company, employee int64) {
		t.Helper()
		s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if s.TotalCompanyDebtKopecks != company || s.TotalEmployeeDebtKopecks != employee || s.TotalPayableKopecks != 500000+company-employee {
			t.Fatalf("debt totals %+v", s)
		}
	}
	check(1025, 500)
	if _, err := SetSalaryDebtClosed(db, a.ID, debt.ID, true); err != nil {
		t.Fatal(err)
	}
	check(0, 500)
	input.Side = "company"
	input.Amount = "20"
	changed, err := SaveSalaryDebt(db, a.ID, debt.ID, input)
	if err != nil || !changed.IsClosed {
		t.Fatal("closed edit", err)
	}
	if _, err := SetSalaryDebtClosed(db, a.ID, debt.ID, false); err != nil {
		t.Fatal(err)
	}
	check(2000, 500)
	if err := DeleteSalaryDebt(db, a.ID, employeeDebt.ID); err != nil {
		t.Fatal(err)
	}
	check(2000, 0)
	other := models.Agreement{Name: "Другой"}
	db.Create(&other)
	if _, err := SaveSalaryDebt(db, other.ID, 0, input); !errors.Is(err, ErrSalaryDebtInput) {
		t.Fatal("foreign employee", err)
	}
	for _, bad := range []SalaryDebtInput{{EmployeeCardID: e.ID, Date: "bad", Side: "company", Amount: "1"}, {EmployeeCardID: e.ID, Date: "2026-01-01", Side: "wrong", Amount: "1"}, {EmployeeCardID: e.ID, Date: "2026-01-01", Side: "company", Amount: "0"}} {
		if _, err := SaveSalaryDebt(db, a.ID, 0, bad); !errors.Is(err, ErrSalaryDebtInput) {
			t.Fatal("invalid", err)
		}
	}
}

// --------- конец нового кода chat
