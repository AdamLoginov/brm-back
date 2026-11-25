// --------- новый код chat: хранение и применение алиментов без изменения заработка
package service

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"project/internal/models"
	"regexp"
	"strings"
)

var salaryAlimonyPattern = regexp.MustCompile(`^\d+(?:\.\d{1,2})?$`)

func saveSalaryAlimony(db *gorm.DB, row SalaryAccrualWithDebts, text string) error {
	text = strings.ReplaceAll(strings.TrimSpace(text), ",", ".")
	if !salaryAlimonyPattern.MatchString(text) {
		return ErrSalaryApprovalInput
	}
	amount, err := salaryPaymentAmount(text)
	if err != nil {
		return err
	}
	if !row.HasAlimony && amount != 0 {
		return ErrSalaryApprovalInput
	}
	record := models.SalaryAlimony{AgreementID: row.AgreementID, TimeSheetMonthID: row.TimeSheetMonthID, EmployeeCardID: row.EmployeeCardID, AmountKopecks: amount}
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agreement_id"}, {Name: "time_sheet_month_id"}, {Name: "employee_card_id"}}, DoUpdates: clause.AssignmentColumns([]string{"amount_kopecks", "updated_at", "deleted_at"})}).Create(&record).Error
}

func salaryAlimonyAmount(db *gorm.DB, row SalaryAccrualWithDeduction) (int64, error) {
	query := db.Where("agreement_id = ? AND time_sheet_month_id = ? AND employee_card_id = ?", row.AgreementID, row.TimeSheetMonthID, row.EmployeeCardID)
	if !row.HasAlimony {
		// Согласованный снимок хранится отдельно и не меняется при отключении отметки.
		return 0, query.Model(&models.SalaryAlimony{}).Update("amount_kopecks", 0).Error
	}
	var record models.SalaryAlimony
	err := query.First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if record.AmountKopecks < 0 || record.AmountKopecks > maxSalaryKopecks {
		return 0, ErrSalaryAmountOverflow
	}
	return record.AmountKopecks, nil
}

// --------- конец нового кода chat
