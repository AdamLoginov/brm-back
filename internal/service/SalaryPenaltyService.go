// --------- новый код chat: штрафы договора
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
	"time"
)

var ErrSalaryPenaltyInput = errors.New("Проверьте сотрудника, дату и положительную сумму штрафа (до двух знаков после запятой)")

// --------- новый код chat: допустимые статусы штрафа
const (
	SalaryPenaltyActive   = "active"
	SalaryPenaltyClosed   = "closed"
	SalaryPenaltyForgiven = "forgiven"
)

var ErrSalaryPenaltyStatus = errors.New("Допустимы только переходы между актуальным и прощённым штрафом")

func ChangeSalaryPenaltyStatus(db *gorm.DB, agreementID, penaltyID uint, status string) (models.SalaryPenalty, error) {
	var row models.SalaryPenalty
	if penaltyID == 0 || (status != SalaryPenaltyActive && status != SalaryPenaltyForgiven) {
		return row, ErrSalaryPenaltyStatus
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, penaltyID).Error; err != nil {
			return err
		}
		previous := row.Status
		if !((previous == SalaryPenaltyActive && status == SalaryPenaltyForgiven) || (previous == SalaryPenaltyForgiven && status == SalaryPenaltyActive)) {
			return ErrSalaryPenaltyStatus
		}
		var closedAt *time.Time
		if status == SalaryPenaltyForgiven {
			now := time.Now().UTC()
			closedAt = &now
		}
		result := tx.Model(&row).Where("status = ?", previous).Updates(map[string]any{"status": status, "closed_at": closedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrSalaryPenaltyStatus
		}
		return tx.First(&row, row.ID).Error
	})
	return row, err
}

// --------- конец нового кода chat

type SalaryPenaltyInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Date           string `json:"date"`
	Amount         string `json:"amount"`
	Comment        string `json:"comment"`
}

func GetSalaryPenalties(db *gorm.DB, agreementID uint) ([]models.SalaryPenalty, error) {
	rows := []models.SalaryPenalty{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("agreement_id = ?", agreementID).Order("date DESC, id DESC").Find(&rows).Error
	return rows, err
}

func SaveSalaryPenalty(db *gorm.DB, agreementID, penaltyID uint, input SalaryPenaltyInput) (models.SalaryPenalty, error) {
	row := models.SalaryPenalty{}
	amount := salaryRateKopecks(input.Amount)
	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil || date.Year() < 1 || amount <= 0 || input.EmployeeCardID == 0 {
		return row, ErrSalaryPenaltyInput
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if penaltyID != 0 {
			if err := tx.Where("agreement_id = ?", agreementID).First(&row, penaltyID).Error; err != nil {
				return err
			}
		}
		if penaltyID == 0 || row.EmployeeCardID != input.EmployeeCardID {
			var employee models.EmployeeCard
			if err := tx.Where("agreement_id = ?", agreementID).First(&employee, input.EmployeeCardID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSalaryPenaltyInput
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

func DeleteSalaryPenalty(db *gorm.DB, agreementID, penaltyID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var row models.SalaryPenalty
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, penaltyID).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

// --------- новый код chat: штрафы только за месяц выбранного табеля
type SalaryAccrualWithPenalties struct {
	SalaryAccrualWithAdvances
	PenaltiesKopecks int64 `json:"penalties_kopecks"`
}

type SalaryPenaltySummary struct {
	Accruals              []SalaryAccrualWithPenalties `json:"accruals"`
	TotalKopecks          int64                        `json:"total_kopecks"`
	TotalAdvancesKopecks  int64                        `json:"total_advances_kopecks"`
	TotalPayableKopecks   int64                        `json:"total_payable_kopecks"`
	TotalPenaltiesKopecks int64                        `json:"total_penalties_kopecks"`
}

func AddSalaryPenalties(db *gorm.DB, agreementID, monthID uint, summary SalaryAccrualSummary) (SalaryPenaltySummary, error) {
	result := SalaryPenaltySummary{Accruals: []SalaryAccrualWithPenalties{}, TotalKopecks: summary.TotalKopecks, TotalAdvancesKopecks: summary.TotalAdvancesKopecks, TotalPayableKopecks: summary.TotalPayableKopecks}
	var month models.TimeSheetMonth
	if err := db.Where("agreement_id = ?", agreementID).First(&month, monthID).Error; err != nil {
		return result, err
	}
	ids := []uint{}
	for _, row := range summary.Accruals {
		ids = append(ids, row.EmployeeCardID)
	}
	penalties := []models.SalaryPenalty{}
	if len(ids) > 0 {
		prefix := fmt.Sprintf("%04d-%02d-", month.Year, month.Month)
		if err := db.Where("agreement_id = ? AND employee_card_id IN ? AND date LIKE ? AND status IN ?", agreementID, ids, prefix+"%", []string{SalaryPenaltyActive, SalaryPenaltyClosed}).Find(&penalties).Error; err != nil { // новый код chat: прощённые штрафы не вычитаются
			return result, err
		}
	}
	totals := map[uint]int64{}
	for _, row := range penalties {
		if row.AmountKopecks < 0 || row.AmountKopecks > maxSalaryKopecks-result.TotalPenaltiesKopecks {
			return result, ErrSalaryAmountOverflow
		}
		result.TotalPenaltiesKopecks += row.AmountKopecks
		totals[row.EmployeeCardID] += row.AmountKopecks
	}
	// Итоги должны оставаться точными и при отрицательном остатке в JavaScript.
	if result.TotalPayableKopecks < -maxSalaryKopecks+result.TotalPenaltiesKopecks {
		return result, ErrSalaryAmountOverflow
	}
	result.TotalPayableKopecks -= result.TotalPenaltiesKopecks
	for _, row := range summary.Accruals {
		amount := totals[row.EmployeeCardID]
		if row.PayableKopecks < -maxSalaryKopecks+amount {
			return result, ErrSalaryAmountOverflow
		}
		row.PayableKopecks -= amount
		result.Accruals = append(result.Accruals, SalaryAccrualWithPenalties{row, amount})
	}
	return result, nil
}

// --------- конец нового кода chat
