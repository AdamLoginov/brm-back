// --------- новый код chat: возмещение личных расходов в начислениях
package service

import (
	"gorm.io/gorm"
	"math"
	"project/internal/models"
)

type SalaryAccrualWithExpenses struct {
	SalaryAccrualWithDeduction
	ExpensesKopecks int64 `json:"expenses_kopecks"`
}
type SalaryExpenseSummary struct {
	SalaryDeductionSummary
	Accruals             []SalaryAccrualWithExpenses `json:"accruals"`
	TotalExpensesKopecks int64                       `json:"total_expenses_kopecks"`
}

// Старое поле Price хранит рубли в float64; складываем расходы уже в копейках.
func ExpenseKopecks(price float64) (int64, error) {
	cents := math.Round(price * 100)
	if math.IsNaN(cents) || math.IsInf(cents, 0) || price < 0 || cents > float64(maxSalaryKopecks) {
		return 0, ErrSalaryAmountOverflow
	}
	return int64(cents), nil
}

func AddSalaryExpenses(db *gorm.DB, agreementID uint, summary SalaryDeductionSummary) (SalaryExpenseSummary, error) {
	result := SalaryExpenseSummary{SalaryDeductionSummary: summary, Accruals: []SalaryAccrualWithExpenses{}}
	ids := []uint{}
	for _, row := range summary.Accruals {
		ids = append(ids, row.EmployeeCardID)
	}
	var expenses []models.Expenses
	if len(ids) > 0 {
		if err := db.Where("agreement_id = ? AND employee_card_id IN ? AND account = ? AND is_reimbursed = ?", agreementID, ids, "employee", false).Find(&expenses).Error; err != nil {
			return result, err
		}
	}
	totals := map[uint]int64{}
	for _, expense := range expenses {
		cents, err := ExpenseKopecks(expense.Price)
		if err != nil {
			return result, err
		}
		if cents > maxSalaryKopecks-result.TotalExpensesKopecks {
			return result, ErrSalaryAmountOverflow
		}
		totals[expense.EmployeeCardId] += cents
		result.TotalExpensesKopecks += cents
	}
	if result.TotalPayableKopecks > maxSalaryKopecks-result.TotalExpensesKopecks {
		return result, ErrSalaryAmountOverflow
	}
	result.TotalPayableKopecks += result.TotalExpensesKopecks
	for _, row := range summary.Accruals {
		amount := totals[row.EmployeeCardID]
		if row.PayableKopecks > maxSalaryKopecks-amount {
			return result, ErrSalaryAmountOverflow
		}
		row.PayableKopecks += amount
		result.Accruals = append(result.Accruals, SalaryAccrualWithExpenses{row, amount})
	}
	return result, nil
}

// --------- конец нового кода chat
