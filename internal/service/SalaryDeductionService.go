// --------- новый код chat: вычет 13% только от официальной месячной зарплаты
package service

import (
	"gorm.io/gorm"
	"project/internal/models"
)

type SalaryAccrualWithDeduction struct {
	SalaryAccrualWithPenalties
	OfficiallyEmployed   bool  `json:"officially_employed"`
	MonthlySalaryKopecks int64 `json:"monthly_salary_kopecks"`
	DeductionKopecks     int64 `json:"deduction_kopecks"`
}

type SalaryDeductionSummary struct {
	SalaryPenaltySummary
	Accruals              []SalaryAccrualWithDeduction `json:"accruals"`
	TotalDeductionKopecks int64                        `json:"total_deduction_kopecks"`
}

func AddSalaryDeductions(db *gorm.DB, summary SalaryPenaltySummary) (SalaryDeductionSummary, error) {
	result := SalaryDeductionSummary{SalaryPenaltySummary: summary, Accruals: []SalaryAccrualWithDeduction{}}
	ids := []uint{}
	for _, row := range summary.Accruals {
		ids = append(ids, row.EmployeeCardID)
	}
	if len(ids) == 0 {
		return result, nil
	}
	var employees []models.EmployeeCard
	// Сотрудник мог перейти в другой договор или быть удалён после создания табеля.
	if err := db.Unscoped().Select("id", "officially_employed", "monthly_salary_kopecks").Where("id IN ?", ids).Find(&employees).Error; err != nil {
		return result, err
	}
	byID := map[uint]models.EmployeeCard{}
	for _, employee := range employees {
		byID[employee.ID] = employee
	}
	for _, row := range summary.Accruals {
		employee, ok := byID[row.EmployeeCardID]
		if !ok {
			return result, gorm.ErrRecordNotFound
		}
		item := SalaryAccrualWithDeduction{SalaryAccrualWithPenalties: row, OfficiallyEmployed: employee.OfficiallyEmployed}
		if employee.OfficiallyEmployed {
			amount := employee.MonthlySalaryKopecks
			if amount < 0 || amount > maxSalaryKopecks {
				return result, ErrSalaryAmountOverflow
			}
			item.MonthlySalaryKopecks = amount
			// Целочисленное округление до копейки: половина копейки округляется вверх.
			item.DeductionKopecks = (amount*13 + 50) / 100
		}
		deduction := item.DeductionKopecks
		if deduction > maxSalaryKopecks-result.TotalDeductionKopecks || item.PayableKopecks < -maxSalaryKopecks+deduction || result.TotalPayableKopecks < -maxSalaryKopecks+deduction {
			return result, ErrSalaryAmountOverflow
		}
		item.PayableKopecks -= deduction
		result.TotalDeductionKopecks += deduction
		result.TotalPayableKopecks -= deduction
		result.Accruals = append(result.Accruals, item)
	}
	return result, nil
}

// --------- конец нового кода chat
