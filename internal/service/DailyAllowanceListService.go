// --------- новый код chat: состав суточных хранится в месячном табеле
package service

import (
	"gorm.io/gorm"
	"project/internal/models"
)

func dailyAllowanceDisabled(month models.TimeSheetMonth, employeeID uint) bool {
	if TimeSheetEmployeeHidden(month, employeeID) {
		return true
	} // новый код chat: скрытые строки рабочего табеля не получают суточные
	for _, id := range month.DailyAllowanceDisabledIDs {
		if id == employeeID {
			return true
		}
	}
	return false
}

func SaveDailyAllowanceList(db *gorm.DB, agreementID, monthID uint, enabledIDs []uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var month models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).Preload("Employees").First(&month, monthID).Error; err != nil {
			return err
		}
		members := map[uint]bool{}
		for _, employee := range month.Employees {
			members[employee.ID] = true
		}
		enabled := map[uint]bool{}
		for _, id := range enabledIDs {
			if !members[id] || enabled[id] || TimeSheetEmployeeHidden(month, id) { // новый код chat: включение только через рабочий табель
				return dailyError("Проверьте список сотрудников табеля")
			}
			enabled[id] = true
		}
		previousDisabled := month.DailyAllowanceDisabledIDs // новый код chat: сохраняем независимую настройку скрытых строк
		month.DailyAllowanceDisabledIDs = []uint{}
		for _, employee := range month.Employees {
			// новый код chat: скрытие рабочего табеля не меняет отдельную настройку участия в суточных
			if TimeSheetEmployeeHidden(month, employee.ID) {
				for _, id := range previousDisabled {
					if id == employee.ID {
						month.DailyAllowanceDisabledIDs = append(month.DailyAllowanceDisabledIDs, id)
						break
					}
				}
				continue
			}
			if !enabled[employee.ID] {
				month.DailyAllowanceDisabledIDs = append(month.DailyAllowanceDisabledIDs, employee.ID)
			}
		}
		if err := tx.Model(&month).Select("DailyAllowanceDisabledIDs").Updates(&month).Error; err != nil {
			return err
		}
		return SyncDailyAllowanceAccruals(tx, month.ID)
	})
}

// --------- конец нового кода chat
