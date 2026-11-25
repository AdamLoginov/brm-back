// --------- новый код chat: состав суточных хранится в месячном табеле
package service

import (
	"gorm.io/gorm"
	"project/internal/models"
)

func dailyAllowanceDisabled(month models.TimeSheetMonth, employeeID uint) bool {
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
			if !members[id] || enabled[id] {
				return dailyError("Проверьте список сотрудников табеля")
			}
			enabled[id] = true
		}
		month.DailyAllowanceDisabledIDs = []uint{}
		for _, employee := range month.Employees {
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
