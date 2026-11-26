// --------- новый код chat: общий список алиментов сотрудника и учёт в любом договоре
package service

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause" // новый код chat: история видимости без дубликатов
	"project/internal/models"
	"regexp"
	"strings"
	"time"
)

var ErrSalaryAlimonyInput = errors.New("Проверьте сотрудника с отметкой «Алименты», дату и положительную сумму до копеек")
var salaryAlimonyPattern = regexp.MustCompile(`^\d+(?:\.\d{1,2})?$`)

type SalaryAlimonyInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Date           string `json:"date"`
	Amount         string `json:"amount"`
	Comment        string `json:"comment"`
}

// Договор определяет только список отображаемых сотрудников, а не владельца алиментов.
func salaryAlimonyEmployees(db *gorm.DB, agreementID uint) *gorm.DB {
	return db.Model(&models.EmployeeCard{}).Where(`agreement_id = ? OR id IN (
 SELECT employee_card_id FROM timesheet_month_employees
 JOIN time_sheet_months ON time_sheet_months.id = timesheet_month_employees.time_sheet_month_id
 WHERE time_sheet_months.agreement_id = ? AND time_sheet_months.deleted_at IS NULL
 ) OR id IN (SELECT employee_card_id FROM salary_accruals WHERE agreement_id = ? AND deleted_at IS NULL)
 OR id IN (SELECT employee_card_id FROM salary_alimony_employee_histories WHERE agreement_id = ?)`, agreementID, agreementID, agreementID, agreementID)
}
func GetSalaryAlimonyEmployees(db *gorm.DB, agreementID uint) ([]models.EmployeeCard, error) {
	rows := []models.EmployeeCard{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := salaryAlimonyEmployees(db, agreementID).Where("has_alimony = ?", true).Order("surname, name, id").Find(&rows).Error
	return rows, err
}
func GetSalaryAlimonies(db *gorm.DB, agreementID uint) ([]models.SalaryAlimony, error) {
	rows := []models.SalaryAlimony{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("employee_card_id IN (?)", salaryAlimonyEmployees(db.Unscoped(), agreementID).Select("id")).Order("date DESC, id DESC").Find(&rows).Error
	return rows, err
}
func SaveSalaryAlimony(db *gorm.DB, agreementID, id uint, input SalaryAlimonyInput) (models.SalaryAlimony, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	row := models.SalaryAlimony{}
	text := strings.ReplaceAll(strings.TrimSpace(input.Amount), ",", ".")
	amount := salaryRateKopecks(text)
	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil || date.Year() < 1 || !salaryAlimonyPattern.MatchString(text) || amount <= 0 || input.EmployeeCardID == 0 {
		return row, ErrSalaryAlimonyInput
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if id != 0 {
			if err := tx.Where("employee_card_id IN (?)", salaryAlimonyEmployees(tx.Unscoped(), agreementID).Select("id")).First(&row, id).Error; err != nil {
				return err
			}
		}
		// Для старой записи допускается редактирование после отключения отметки.
		// Смена владельца закрытого удержания запрещена: оно уже вошло в согласование.
		if row.IsSettled && row.EmployeeCardID != input.EmployeeCardID {
			return ErrSalaryAlimonyInput
		}
		if id == 0 || row.EmployeeCardID != input.EmployeeCardID {
			var employee models.EmployeeCard
			if err := salaryAlimonyEmployees(tx, agreementID).Where("id = ? AND has_alimony = ?", input.EmployeeCardID, true).First(&employee).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSalaryAlimonyInput
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
		row.EmployeeCardID = input.EmployeeCardID
		row.Date = input.Date
		row.AmountKopecks = amount
		row.Comment = strings.TrimSpace(input.Comment)
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		// новый код chat: только доступ к истории из этой вкладки, запись остаётся общей
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SalaryAlimonyEmployeeHistory{EmployeeCardID: row.EmployeeCardID, AgreementID: agreementID}).Error
	})
	return row, err
}
func DeleteSalaryAlimony(db *gorm.DB, agreementID, id uint) error {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var row models.SalaryAlimony
		if err := tx.Where("employee_card_id IN (?)", salaryAlimonyEmployees(tx.Unscoped(), agreementID).Select("id")).First(&row, id).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}
func salaryAlimonyAmount(db *gorm.DB, row SalaryAccrualWithDeduction) (int64, error) {
	// Уже назначенные алименты учитываются даже после отключения отметки в профиле.
	var records []models.SalaryAlimony
	if err := db.Where("employee_card_id = ? AND is_settled = ?", row.EmployeeCardID, false).Find(&records).Error; err != nil {
		return 0, err
	}
	var total int64
	for _, record := range records {
		if record.AmountKopecks <= 0 || record.AmountKopecks > maxSalaryKopecks-total {
			return 0, ErrSalaryAmountOverflow
		}
		total += record.AmountKopecks
	}
	return total, nil
}

// --------- конец нового кода chat
