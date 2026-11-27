// --------- новый код chat: расчёт начислений по сохранённым часам табеля
package service

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	"project/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxSalaryKopecks int64 = 9007199254740991 // точное целое также в JavaScript

var salaryDecimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)
var ErrSalaryAmountOverflow = errors.New("сумма начисления слишком велика")

// Денежные значения не проходят через float64, округляется только месячный итог.
func parseSalaryDecimal(value string) *big.Rat {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	if !salaryDecimalPattern.MatchString(value) {
		return new(big.Rat)
	}
	result, ok := new(big.Rat).SetString(value)
	if !ok {
		return new(big.Rat)
	}
	return result
}

func salaryRateKopecks(value string) int64 {
	value = strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "").Replace(value)
	rate := parseSalaryDecimal(value)
	rate.Mul(rate, big.NewRat(100, 1))
	if !rate.Num().IsInt64() || rate.Num().Int64() > maxSalaryKopecks {
		return 0
	}
	return rate.Num().Int64()
}

// новый код chat: деление на точную дробную норму табеля
func salaryAmountKopecks(hours *big.Rat, rate int64, shiftHours string) (int64, error) {
	shift := parseSalaryDecimal(shiftHours)
	if shift.Sign() <= 0 || shift.Cmp(big.NewRat(24, 1)) > 0 {
		return 0, ErrTimeSheetSettings
	}
	amount := new(big.Rat).Mul(hours, big.NewRat(rate, 1))
	amount.Quo(amount, shift)
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(amount.Num(), amount.Denom(), remainder)
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(amount.Denom()) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	if !whole.IsInt64() || whole.Int64() > maxSalaryKopecks {
		return 0, ErrSalaryAmountOverflow
	}
	return whole.Int64(), nil
}

func findSalaryTimeSheet(db *gorm.DB, agreementID, monthID uint, month *models.TimeSheetMonth) error {
	var agreement models.Agreement
	if err := db.First(&agreement, agreementID).Error; err != nil {
		return err
	}
	return db.Where("agreement_id = ?", agreementID).First(month, monthID).Error
}

func GetSalaryAccruals(db *gorm.DB, agreementID, monthID uint) ([]models.SalaryAccrual, error) {
	var month models.TimeSheetMonth
	rows := make([]models.SalaryAccrual, 0)
	if err := findSalaryTimeSheet(db, agreementID, monthID, &month); err != nil {
		return rows, err
	}
	err := db.Where("time_sheet_month_id = ? AND agreement_id = ?", monthID, agreementID).
		Order("employee_name, employee_card_id").Find(&rows).Error
	return rows, err
}

func CalculateSalaryAccruals(db *gorm.DB, agreementID, monthID uint) ([]models.SalaryAccrual, error) {
	rows := make([]models.SalaryAccrual, 0)
	err := db.Transaction(func(tx *gorm.DB) error {
		var month models.TimeSheetMonth
		if err := findSalaryTimeSheet(tx, agreementID, monthID, &month); err != nil {
			return err
		}
		if err := tx.Preload("Employees").Preload("TimeSheets").First(&month, month.ID).Error; err != nil {
			return err
		}
		calculated, err := salaryRowsForMonth(month) // новый код chat: тот же расчёт для сводки
		if err != nil {
			return err
		}
		employeeIDs := make([]uint, 0, len(calculated))
		for _, row := range calculated {
			employeeIDs = append(employeeIDs, row.EmployeeCardID)
			// Уникальная пара табель + сотрудник сохраняет ID при повторном расчёте.
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "time_sheet_month_id"}, {Name: "employee_card_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"employee_name": row.EmployeeName, "total_hours": row.TotalHours,
					"shift_hours": row.ShiftHours, "salary_rate_kopecks": row.SalaryRateKopecks, "amount_kopecks": row.AmountKopecks,
					"agreement_id": agreementID, "updated_at": time.Now(), "deleted_at": nil,
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		removed := tx.Unscoped().Where("time_sheet_month_id = ?", month.ID)
		if len(employeeIDs) > 0 {
			removed = removed.Where("employee_card_id NOT IN ?", employeeIDs)
		}
		if err := removed.Delete(&models.SalaryAccrual{}).Error; err != nil {
			return err
		}
		return tx.Where("time_sheet_month_id = ?", month.ID).Order("employee_name, employee_card_id").Find(&rows).Error
	})
	return rows, err
}

// новый код chat: расчёт в памяти без изменения ведомости
func salaryRowsForMonth(month models.TimeSheetMonth) ([]models.SalaryAccrual, error) {
	hoursByEmployee := make(map[uint]*big.Rat)
	for _, entry := range month.TimeSheets {
		hours := parseSalaryDecimal(entry.Status)
		if hours.Cmp(big.NewRat(24, 1)) > 0 {
			continue // некорректное количество часов в одной дневной записи
		}
		if hoursByEmployee[entry.EmployeeCardID] == nil {
			hoursByEmployee[entry.EmployeeCardID] = new(big.Rat)
		}
		hoursByEmployee[entry.EmployeeCardID].Add(hoursByEmployee[entry.EmployeeCardID], hours)
	}
	rows := make([]models.SalaryAccrual, 0, len(month.Employees))
	var total int64
	for _, employee := range month.Employees {
		hours := hoursByEmployee[employee.ID]
		if hours == nil {
			hours = new(big.Rat)
		}
		rate := salaryRateKopecks(employee.SalaryRate)
		amount, err := salaryAmountKopecks(hours, rate, month.ShiftHours)
		if err != nil {
			return nil, err
		} // новый код chat: ошибка нормы смены
		if total > maxSalaryKopecks-amount {
			return nil, ErrSalaryAmountOverflow
		}
		total += amount
		nameParts := []string{}
		if employee.Surname != nil {
			nameParts = append(nameParts, *employee.Surname)
		}
		nameParts = append(nameParts, employee.Name)
		if employee.MiddleName != nil {
			nameParts = append(nameParts, *employee.MiddleName)
		}
		row := models.SalaryAccrual{
			TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, AgreementID: month.AgreementID,
			EmployeeName: strings.TrimSpace(strings.Join(nameParts, " ")),
			TotalHours:   hours.FloatString(2), ShiftHours: month.ShiftHours,
			SalaryRateKopecks: rate, AmountKopecks: amount,
		}
		rows = append(rows, row)
	}
	return rows, nil
}
