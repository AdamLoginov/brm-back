// --------- новый код chat: проверки основания, округления и итогов вычета
package service

import (
	"errors"
	"project/internal/models"
	"testing"
)

func TestSalaryDeductions(t *testing.T) {
	db := salaryTestDB(t)
	_, _, employee := salaryFixture(t, db)
	for _, tc := range []struct {
		name                                       string
		official                                   bool
		salary, gross, advance, penalty, deduction int64
	}{
		{"official 35000 earned 120000", true, 3500000, 12000000, 0, 0, 455000},
		{"other deductions", true, 3500000, 12000000, 100000, 200000, 455000},
		{"unofficial", false, 3500000, 12000000, 0, 0, 0},
		{"zero salary", true, 0, 12000000, 0, 0, 0},
		{"zero earned", true, 3500000, 0, 0, 0, 455000},
		{"round down", true, 3, 100, 0, 0, 0},
		{"round up", true, 4, 100, 0, 0, 1},
		{"half kopeck", true, 50, 100, 0, 0, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Model(&employee).Updates(map[string]any{"officially_employed": tc.official, "monthly_salary_kopecks": tc.salary}).Error; err != nil {
				t.Fatal(err)
			}
			payable := tc.gross - tc.advance - tc.penalty
			summary := SalaryPenaltySummary{Accruals: []SalaryAccrualWithPenalties{{SalaryAccrualWithAdvances: SalaryAccrualWithAdvances{SalaryAccrual: models.SalaryAccrual{EmployeeCardID: employee.ID, AmountKopecks: tc.gross}, AdvancesKopecks: tc.advance, PayableKopecks: payable}, PenaltiesKopecks: tc.penalty}}, TotalKopecks: tc.gross, TotalAdvancesKopecks: tc.advance, TotalPenaltiesKopecks: tc.penalty, TotalPayableKopecks: payable}
			result, err := AddSalaryDeductions(db, summary)
			if err != nil {
				t.Fatal(err)
			}
			row := result.Accruals[0]
			if row.DeductionKopecks != tc.deduction || result.TotalDeductionKopecks != tc.deduction || row.PayableKopecks != payable-tc.deduction || result.TotalPayableKopecks != payable-tc.deduction || row.AmountKopecks != tc.gross {
				t.Fatalf("unexpected result: %+v", result)
			}
			if !tc.official && row.MonthlySalaryKopecks != 0 {
				t.Fatal("unofficial salary exposed")
			}
			if summary.Accruals[0].PayableKopecks != payable {
				t.Fatal("input summary mutated")
			}
		})
	}
	if err := db.Model(&employee).Updates(map[string]any{"officially_employed": true, "monthly_salary_kopecks": 100}).Error; err != nil {
		t.Fatal(err)
	}
	summary := SalaryPenaltySummary{Accruals: []SalaryAccrualWithPenalties{{SalaryAccrualWithAdvances: SalaryAccrualWithAdvances{SalaryAccrual: models.SalaryAccrual{EmployeeCardID: employee.ID}, PayableKopecks: -maxSalaryKopecks}}}, TotalPayableKopecks: -maxSalaryKopecks}
	if _, err := AddSalaryDeductions(db, summary); !errors.Is(err, ErrSalaryAmountOverflow) {
		t.Fatalf("overflow: %v", err)
	}
}

// --------- конец нового кода chat
