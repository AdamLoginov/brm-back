// --------- новый код chat: премии договора и включение в начисления
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
)

var ErrSalaryBonusInput = errors.New("Проверьте сотрудника, месяц, год и положительную сумму премии до копеек")

type SalaryBonusInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Month          uint   `json:"month"`
	Year           uint   `json:"year"`
	Amount         string `json:"amount"`
	Comment        string `json:"comment"`
}

func GetSalaryBonuses(db *gorm.DB, agreementID uint) ([]models.SalaryBonus, error) {
	rows := []models.SalaryBonus{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("agreement_id = ?", agreementID).Order("year DESC, month DESC, id DESC").Find(&rows).Error
	return rows, err
}
func SaveSalaryBonus(db *gorm.DB, agreementID, bonusID uint, input SalaryBonusInput) (models.SalaryBonus, error) {
	row := models.SalaryBonus{}
	amount := salaryRateKopecks(input.Amount)
	if input.Month < 1 || input.Month > 12 || input.Year < 1 || input.Year > 9999 || input.EmployeeCardID == 0 || amount <= 0 {
		return row, ErrSalaryBonusInput
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if bonusID != 0 {
			if err := tx.Where("agreement_id = ?", agreementID).First(&row, bonusID).Error; err != nil {
				return err
			}
		}
		// При редактировании сохраняем возможность работать с премией переведённого сотрудника.
		if bonusID == 0 || row.EmployeeCardID != input.EmployeeCardID {
			var employee models.EmployeeCard
			if err := tx.Where("agreement_id = ?", agreementID).First(&employee, input.EmployeeCardID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSalaryBonusInput
				}
				return err
			}
			parts := []string{}
			if employee.Surname != nil {
				parts = append(parts, *employee.Surname)
			}
			parts = append(parts, employee.Name)
			if employee.MiddleName != nil {
				parts = append(parts, *employee.MiddleName)
			}
			row.EmployeeName = strings.TrimSpace(strings.Join(parts, " "))
		}
		row.AgreementID, row.EmployeeCardID = agreementID, input.EmployeeCardID
		row.Month, row.Year, row.AmountKopecks, row.Comment = input.Month, input.Year, amount, strings.TrimSpace(input.Comment)
		return tx.Save(&row).Error
	})
	return row, err
}
func SetSalaryBonusPaid(db *gorm.DB, agreementID, bonusID uint, paid bool) (models.SalaryBonus, error) {
	var row models.SalaryBonus
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, bonusID).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Update("is_paid", paid).Error; err != nil {
			return err
		}
		row.IsPaid = paid
		return nil
	})
	return row, err
}
func DeleteSalaryBonus(db *gorm.DB, agreementID, bonusID uint) error {
	var row models.SalaryBonus
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, bonusID).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

type SalaryAccrualWithBonuses struct {
	SalaryAccrualWithExpenses
	BonusesKopecks int64 `json:"bonuses_kopecks"`
}
type SalaryBonusSummary struct {
	SalaryExpenseSummary
	Accruals            []SalaryAccrualWithBonuses `json:"accruals"`
	TotalBonusesKopecks int64                      `json:"total_bonuses_kopecks"`
}

func AddSalaryBonuses(db *gorm.DB, agreementID, monthID uint, summary SalaryExpenseSummary) (SalaryBonusSummary, error) {
	result := SalaryBonusSummary{SalaryExpenseSummary: summary, Accruals: []SalaryAccrualWithBonuses{}}
	var month models.TimeSheetMonth
	if err := db.Where("agreement_id = ?", agreementID).First(&month, monthID).Error; err != nil {
		return result, err
	}
	ids := []uint{}
	for _, row := range summary.Accruals {
		ids = append(ids, row.EmployeeCardID)
	}
	var bonuses []models.SalaryBonus
	if len(ids) > 0 {
		if err := db.Where("agreement_id = ? AND employee_card_id IN ? AND month = ? AND year = ? AND is_paid = ?", agreementID, ids, month.Month, month.Year, false).Find(&bonuses).Error; err != nil {
			return result, err
		}
	}
	totals := map[uint]int64{}
	for _, row := range bonuses {
		if row.AmountKopecks < 0 || row.AmountKopecks > maxSalaryKopecks-result.TotalBonusesKopecks {
			return result, ErrSalaryAmountOverflow
		}
		totals[row.EmployeeCardID] += row.AmountKopecks
		result.TotalBonusesKopecks += row.AmountKopecks
	}
	if result.TotalPayableKopecks > maxSalaryKopecks-result.TotalBonusesKopecks {
		return result, ErrSalaryAmountOverflow
	}
	result.TotalPayableKopecks += result.TotalBonusesKopecks
	for _, row := range summary.Accruals {
		amount := totals[row.EmployeeCardID]
		if row.PayableKopecks > maxSalaryKopecks-amount {
			return result, ErrSalaryAmountOverflow
		}
		row.PayableKopecks += amount
		result.Accruals = append(result.Accruals, SalaryAccrualWithBonuses{row, amount})
	}
	return result, nil
}

// --------- конец нового кода chat
