// --------- новый код chat: выплаченные авансы договора
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
	"time"
)

var ErrSalaryAdvanceInput = errors.New("Проверьте сотрудника, дату и положительную сумму аванса (до двух знаков после запятой)")

type SalaryAdvanceInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Date           string `json:"date"`
	Amount         string `json:"amount"`
	Comment        string `json:"comment"`
}

func GetSalaryAdvances(db *gorm.DB, agreementID uint) ([]models.SalaryAdvance, error) {
	rows := []models.SalaryAdvance{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("agreement_id = ?", agreementID).Order("date DESC, id DESC").Find(&rows).Error
	return rows, err
}

func SaveSalaryAdvance(db *gorm.DB, agreementID, advanceID uint, input SalaryAdvanceInput) (models.SalaryAdvance, error) {
	row := models.SalaryAdvance{}
	amount := salaryRateKopecks(input.Amount)
	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil || date.Year() < 1 || amount <= 0 || input.EmployeeCardID == 0 {
		return row, ErrSalaryAdvanceInput
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if advanceID != 0 {
			if err := tx.Where("agreement_id = ?", agreementID).First(&row, advanceID).Error; err != nil {
				return err
			}
		}
		if advanceID == 0 || row.EmployeeCardID != input.EmployeeCardID {
			var employee models.EmployeeCard
			if err := tx.Where("agreement_id = ?", agreementID).First(&employee, input.EmployeeCardID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSalaryAdvanceInput
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
		row.Date, row.AmountKopecks, row.Comment = input.Date, amount, strings.TrimSpace(input.Comment)
		return tx.Save(&row).Error
	})
	return row, err
}

func DeleteSalaryAdvance(db *gorm.DB, agreementID, advanceID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var row models.SalaryAdvance
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, advanceID).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

type SalaryAccrualWithAdvances struct {
	models.SalaryAccrual
	AdvancesKopecks int64 `json:"advances_kopecks"`
	PayableKopecks  int64 `json:"payable_kopecks"`
}

type SalaryAccrualSummary struct {
	Accruals             []SalaryAccrualWithAdvances `json:"accruals"`
	TotalKopecks         int64                       `json:"total_kopecks"`
	TotalAdvancesKopecks int64                       `json:"total_advances_kopecks"`
	TotalPayableKopecks  int64                       `json:"total_payable_kopecks"`
}

func AddSalaryAdvances(db *gorm.DB, agreementID uint, rows []models.SalaryAccrual) (SalaryAccrualSummary, error) {
	result := SalaryAccrualSummary{Accruals: []SalaryAccrualWithAdvances{}}
	ids := []uint{}
	for _, row := range rows {
		ids = append(ids, row.EmployeeCardID)
	}
	advances := []models.SalaryAdvance{}
	if len(ids) > 0 {
		if err := db.Where("agreement_id = ? AND employee_card_id IN ? AND is_settled = ?", agreementID, ids, false).Find(&advances).Error; err != nil {
			return result, err
		}
	}
	totals := map[uint]int64{}
	for _, advance := range advances {
		if advance.AmountKopecks < 0 || advance.AmountKopecks > maxSalaryKopecks-result.TotalAdvancesKopecks {
			return result, ErrSalaryAmountOverflow
		}
		totals[advance.EmployeeCardID] += advance.AmountKopecks
		result.TotalAdvancesKopecks += advance.AmountKopecks
	}
	for _, row := range rows {
		if row.AmountKopecks > maxSalaryKopecks-result.TotalKopecks {
			return result, ErrSalaryAmountOverflow
		}
		result.TotalKopecks += row.AmountKopecks
		result.Accruals = append(result.Accruals, SalaryAccrualWithAdvances{row, totals[row.EmployeeCardID], row.AmountKopecks - totals[row.EmployeeCardID]})
	}
	result.TotalPayableKopecks = result.TotalKopecks - result.TotalAdvancesKopecks
	return result, nil
}

// --------- конец нового кода chat
