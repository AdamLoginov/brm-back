// --------- новый код chat: долги договора и включение в начисления
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
	"time"
)

var ErrSalaryDebtInput = errors.New("Проверьте сотрудника, дату, сторону долга и положительную сумму долги до копеек")

type SalaryDebtInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Date           string `json:"date"`
	Side           string `json:"side"`
	Amount         string `json:"amount"`
	Comment        string `json:"comment"`
}

func GetSalaryDebts(db *gorm.DB, agreementID uint) ([]models.SalaryDebt, error) {
	rows := []models.SalaryDebt{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("agreement_id = ?", agreementID).Order("date DESC, id DESC").Find(&rows).Error
	return rows, err
}
func SaveSalaryDebt(db *gorm.DB, agreementID, debtID uint, input SalaryDebtInput) (models.SalaryDebt, error) {
	row := models.SalaryDebt{}
	amount := salaryRateKopecks(input.Amount)
	date, dateErr := time.Parse("2006-01-02", input.Date)
	if dateErr != nil || date.Year() < 1 || (input.Side != "company" && input.Side != "employee") || input.EmployeeCardID == 0 || amount <= 0 {
		return row, ErrSalaryDebtInput
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if debtID != 0 {
			if err := tx.Where("agreement_id = ?", agreementID).First(&row, debtID).Error; err != nil {
				return err
			}
		}
		// При редактировании сохраняем возможность работать с долгом переведённого сотрудника.
		if debtID == 0 || row.EmployeeCardID != input.EmployeeCardID {
			var employee models.EmployeeCard
			if err := tx.Where("agreement_id = ?", agreementID).First(&employee, input.EmployeeCardID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrSalaryDebtInput
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
		row.Date, row.Side, row.AmountKopecks, row.Comment = input.Date, input.Side, amount, strings.TrimSpace(input.Comment)
		return tx.Save(&row).Error
	})
	return row, err
}
func SetSalaryDebtClosed(db *gorm.DB, agreementID, debtID uint, closed bool) (models.SalaryDebt, error) {
	var row models.SalaryDebt
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, debtID).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Update("is_closed", closed).Error; err != nil {
			return err
		}
		row.IsClosed = closed
		return nil
	})
	return row, err
}
func DeleteSalaryDebt(db *gorm.DB, agreementID, debtID uint) error {
	var row models.SalaryDebt
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, debtID).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

type SalaryAccrualWithDebts struct {
	// --------- новый код chat: состояние сотрудника
	Approved     bool   `json:"approved"`
	ApprovalID   uint   `json:"approval_id"`
	ApprovalDate string `json:"approval_date"`
	Revision     string `json:"revision"`
	HasDraft     bool   `json:"has_draft"`
	DraftReset   bool   `json:"draft_reset"`
	// --------- конец нового кода chat
	SalaryAccrualWithBonuses
	CompanyDebtKopecks     int64                `json:"company_debt_kopecks"`
	EmployeeDebtKopecks    int64                `json:"employee_debt_kopecks"`
	PaymentKopecks         int64                `json:"payment_kopecks"`
	ExtraBonusKopecks      int64                `json:"extra_bonus_kopecks"`
	NewCompanyDebtKopecks  int64                `json:"new_company_debt_kopecks"`
	NewEmployeeDebtKopecks int64                `json:"new_employee_debt_kopecks"`
	Detail                 SalaryApprovalDetail `json:"detail"`
}
type SalaryApprovalDetail struct {
	Alimonies  []models.SalaryAlimony `json:"alimonies"` // новый код chat: удержания сотрудника всех договоров
	TimeSheets []models.TimeSheet     `json:"time_sheets"`
	Advances   []models.SalaryAdvance `json:"advances"`
	Penalties  []models.SalaryPenalty `json:"penalties"`
	Expenses   []models.Expenses      `json:"expenses"`
	Bonuses    []models.SalaryBonus   `json:"bonuses"`
	Debts      []models.SalaryDebt    `json:"debts"`
}
type SalaryDebtSummary struct {
	CalendarMonth uint `json:"calendar_month"` // новый код chat: месяц сохранённого календаря
	CalendarYear  uint `json:"calendar_year"`  // новый код chat
	SalaryBonusSummary
	Accruals                 []SalaryAccrualWithDebts `json:"accruals"`
	TotalCompanyDebtKopecks  int64                    `json:"total_company_debt_kopecks"`
	TotalEmployeeDebtKopecks int64                    `json:"total_employee_debt_kopecks"`
	TotalPaymentKopecks      int64                    `json:"total_payment_kopecks"`
	Approved                 bool                     `json:"approved"`
	ApprovalID               uint                     `json:"approval_id"`
	ApprovalDate             string                   `json:"approval_date"`
	Revision                 string                   `json:"revision"`
}

func AddSalaryDebts(db *gorm.DB, agreementID uint, summary SalaryBonusSummary) (SalaryDebtSummary, error) {
	result := SalaryDebtSummary{SalaryBonusSummary: summary, Accruals: []SalaryAccrualWithDebts{}}
	ids := []uint{}
	for _, row := range summary.Accruals {
		ids = append(ids, row.EmployeeCardID)
	}
	var debts []models.SalaryDebt
	if len(ids) > 0 {
		if err := db.Where("agreement_id = ? AND employee_card_id IN ? AND is_closed = ?", agreementID, ids, false).Order("id").Find(&debts).Error; err != nil {
			return result, err
		}
	}
	company, employee := map[uint]int64{}, map[uint]int64{}
	for _, debt := range debts {
		if debt.AmountKopecks < 0 {
			return result, ErrSalaryAmountOverflow
		}
		if debt.Side == "company" {
			if debt.AmountKopecks > maxSalaryKopecks-result.TotalCompanyDebtKopecks {
				return result, ErrSalaryAmountOverflow
			}
			company[debt.EmployeeCardID] += debt.AmountKopecks
			result.TotalCompanyDebtKopecks += debt.AmountKopecks
		} else if debt.Side == "employee" {
			if debt.AmountKopecks > maxSalaryKopecks-result.TotalEmployeeDebtKopecks {
				return result, ErrSalaryAmountOverflow
			}
			employee[debt.EmployeeCardID] += debt.AmountKopecks
			result.TotalEmployeeDebtKopecks += debt.AmountKopecks
		}
	}
	if result.TotalPayableKopecks > maxSalaryKopecks-result.TotalCompanyDebtKopecks {
		return result, ErrSalaryAmountOverflow
	}
	result.TotalPayableKopecks += result.TotalCompanyDebtKopecks
	if result.TotalPayableKopecks < -maxSalaryKopecks+result.TotalEmployeeDebtKopecks {
		return result, ErrSalaryAmountOverflow
	}
	result.TotalPayableKopecks -= result.TotalEmployeeDebtKopecks
	for _, row := range summary.Accruals {
		c, e := company[row.EmployeeCardID], employee[row.EmployeeCardID]
		if row.PayableKopecks > maxSalaryKopecks-c {
			return result, ErrSalaryAmountOverflow
		}
		row.PayableKopecks += c
		if row.PayableKopecks < -maxSalaryKopecks+e {
			return result, ErrSalaryAmountOverflow
		}
		row.PayableKopecks -= e
		payment := row.PayableKopecks
		if payment < 0 {
			payment = 0
		}
		if payment > maxSalaryKopecks-result.TotalPaymentKopecks {
			return result, ErrSalaryAmountOverflow
		}
		result.TotalPaymentKopecks += payment
		result.Accruals = append(result.Accruals, SalaryAccrualWithDebts{SalaryAccrualWithBonuses: row, CompanyDebtKopecks: c, EmployeeDebtKopecks: e, PaymentKopecks: payment})
	}
	return result, nil
}

// --------- конец нового кода chat
