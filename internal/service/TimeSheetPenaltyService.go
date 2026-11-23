// --------- новый код chat: настройки табеля и синхронизация штрафов за прогул
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"math/big"
	"project/internal/models"
	"strings"
	"time"
)

var ErrTimeSheetSettings = errors.New("Укажите часы в смене больше 0 и не больше 24, штраф не меньше 0; допускается до двух знаков после запятой")

// --------- новый код chat: суточные в копейках, без участия в расчёте зарплаты
func ParseTimeSheetDailyAllowance(value *string, current int64) (int64, error) {
	_, amount, err := ParseTimeSheetSettings(nil, value, "12", current)
	if err != nil {
		return 0, errors.New("Суточные должны быть не меньше 0, с точностью до копеек и в допустимом диапазоне")
	}
	return amount, nil
}

// --------- конец нового кода chat

// Строки сохраняют точные десятичные значения; nil поддерживает прежние клиенты API.
func ParseTimeSheetSettings(hours, penalty *string, currentHours string, currentPenalty int64) (string, int64, error) {
	if hours != nil {
		currentHours = strings.ReplaceAll(strings.TrimSpace(*hours), ",", ".")
	}
	shift := parseSalaryDecimal(currentHours)
	if shift.Sign() <= 0 || shift.Cmp(big.NewRat(24, 1)) > 0 {
		return "", 0, ErrTimeSheetSettings
	}
	if penalty != nil {
		value := strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "").Replace(strings.ReplaceAll(strings.TrimSpace(*penalty), ",", "."))
		if !salaryDecimalPattern.MatchString(value) {
			return "", 0, ErrTimeSheetSettings
		}
		exact := parseSalaryDecimal(value)
		currentPenalty = salaryRateKopecks(value)
		if exact.Sign() > 0 && currentPenalty == 0 {
			return "", 0, ErrTimeSheetSettings
		}
	}
	if currentPenalty < 0 || currentPenalty > maxSalaryKopecks {
		return "", 0, ErrTimeSheetSettings
	}
	return currentHours, currentPenalty, nil
}

// Вызывать внутри транзакции, в которой сохраняются отметки или настройки.
func SyncTimeSheetPenalties(tx *gorm.DB, monthID uint) error {
	var month models.TimeSheetMonth
	if err := tx.Preload("Employees").Preload("TimeSheets").First(&month, monthID).Error; err != nil {
		return err
	}
	if month.AbsencePenaltyKopecks < 0 || month.AbsencePenaltyKopecks > maxSalaryKopecks {
		return ErrTimeSheetSettings
	}
	members := map[uint]models.EmployeeCard{}
	for _, employee := range month.Employees {
		members[employee.ID] = employee
	}
	keep := []uint{}
	seen := map[string]bool{}
	if month.AbsencePenaltyKopecks > 0 {
		for _, entry := range month.TimeSheets {
			employee, ok := members[entry.EmployeeCardID]
			if !ok || entry.Status != "П" {
				continue
			}
			if _, err := time.Parse("2006-01-02", entry.Date); err != nil {
				return errors.New("Некорректная дата прогула")
			}
			key := fmt.Sprintf("%d:%d:%s", month.ID, employee.ID, entry.Date)
			if seen[key] {
				continue
			}
			seen[key] = true
			var row models.SalaryPenalty
			err := tx.Unscoped().Where("source_time_sheet_key = ?", key).First(&row).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if errors.Is(err, gorm.ErrRecordNotFound) || row.DeletedAt.Valid {
				row.Status = SalaryPenaltyActive
				row.ClosedAt = nil
				row.DeletedAt = gorm.DeletedAt{}
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
			row.AgreementID = month.AgreementID
			row.EmployeeCardID = employee.ID
			row.Date = entry.Date
			row.AmountKopecks = month.AbsencePenaltyKopecks
			row.Comment = "Прогул"
			row.SourceTimeSheetMonthID = &month.ID
			row.SourceTimeSheetID = &entry.ID
			row.SourceTimeSheetKey = &key
			if err := tx.Unscoped().Save(&row).Error; err != nil {
				return err
			}
			keep = append(keep, row.ID)
		}
	}
	removed := tx.Where("source_time_sheet_month_id = ?", month.ID)
	if len(keep) > 0 {
		removed = removed.Where("id NOT IN ?", keep)
	}
	return removed.Delete(&models.SalaryPenalty{}).Error
}

func DeleteTimeSheetPenalties(tx *gorm.DB, monthID uint) error {
	return tx.Where("source_time_sheet_month_id = ?", monthID).Delete(&models.SalaryPenalty{}).Error
}

// Сохранение пачки дневных отметок и штрафов атомарно; источник берётся из БД.
func ChangeTimeSheetEntries(db *gorm.DB, entries []models.TimeSheet, action string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		months := map[uint]bool{}
		for i, input := range entries {
			var row models.TimeSheet
			if action == "create" {
				var month models.TimeSheetMonth
				if err := tx.Preload("Employees").First(&month, input.TimeSheetMonthID).Error; err != nil {
					return err
				}
				member := false
				for _, employee := range month.Employees {
					if employee.ID == input.EmployeeCardID {
						member = true
						// новый код chat: нельзя создать отметку начиная с увольнения
						if employee.Status && employee.DismissalDate != "" && input.Date >= employee.DismissalDate {
							return errors.New("Нельзя заполнять табель с даты увольнения сотрудника")
						}
						break
					}
				}
				if !member {
					return errors.New("Сотрудник не включён в табель")
				}
				if _, err := time.Parse("2006-01-02", input.Date); err != nil {
					return errors.New("Некорректная дата отметки")
				}
				err := tx.Where("time_sheet_month_id = ? AND employee_card_id = ? AND date = ?", month.ID, input.EmployeeCardID, input.Date).First(&row).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					row = models.TimeSheet{TimeSheetMonthID: month.ID, AgreementID: month.AgreementID, EmployeeCardID: input.EmployeeCardID, Date: input.Date}
				}
				row.Status = input.Status
				if err := tx.Omit("EmployeeCard").Save(&row).Error; err != nil {
					return err
				}
			} else {
				if input.ID == 0 {
					return errors.New("Не указан ID отметки")
				}
				if err := tx.First(&row, input.ID).Error; err != nil {
					return err
				}
				switch action {
				case "update":
					// --------- новый код chat: проверка по реальному сотруднику и дате записи
					var employee models.EmployeeCard
					if err := tx.First(&employee, row.EmployeeCardID).Error; err != nil {
						return err
					}
					if employee.Status && employee.DismissalDate != "" && row.Date >= employee.DismissalDate {
						return errors.New("Нельзя заполнять табель с даты увольнения сотрудника")
					}
					// --------- конец нового кода chat
					row.Status = input.Status
					if err := tx.Model(&row).Update("status", row.Status).Error; err != nil {
						return err
					}
				case "delete":
					if err := tx.Delete(&row).Error; err != nil {
						return err
					}
				default:
					return errors.New("Неизвестное действие с табелем")
				}
			}
			months[row.TimeSheetMonthID] = true
			entries[i] = row
		}
		for monthID := range months {
			if err := SyncTimeSheetPenalties(tx, monthID); err != nil {
				return err
			}
			if err := SyncDailyAllowanceAccruals(tx, monthID); err != nil {
				return err
			} // новый код chat: суточные вместе с отметками
		}
		return nil
	})
}

// --------- новый код chat: очистка отметок и производных начислений после увольнения
// Вызывается внутри транзакции сохранения карточки сотрудника.
func ClearDismissedEmployeeTimeSheets(tx *gorm.DB, employee models.EmployeeCard) error {
	if !employee.Status || employee.DismissalDate == "" {
		return nil
	}
	var entries []models.TimeSheet
	if err := tx.Where("employee_card_id = ? AND date >= ?", employee.ID, employee.DismissalDate).Find(&entries).Error; err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return ChangeTimeSheetEntries(tx, entries, "delete")
}

// --------- конец нового кода chat

// --------- конец нового кода chat
