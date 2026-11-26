// --------- новый код chat: состав рабочего табеля и отдельная видимость строк
package service

import (
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"time"
)

func TimeSheetEmployeeHidden(month models.TimeSheetMonth, id uint) bool {
	for _, hidden := range month.HiddenEmployeeIDs {
		if hidden == id {
			return true
		}
	}
	return false
}
func timeSheetEmployeeEligible(employee models.EmployeeCard, year, month uint) bool {
	if !employee.Status {
		return true
	}
	_, err := time.Parse("2006-01-02", employee.DismissalDate)
	return err == nil && employee.DismissalDate >= fmt.Sprintf("%04d-%02d-01", year, month)
}
func GetTimeSheetEmployeeList(db *gorm.DB, agreementID, monthID uint) ([]models.EmployeeCard, error) {
	var month models.TimeSheetMonth
	if err := db.Where("agreement_id = ?", agreementID).Preload("Employees").First(&month, monthID).Error; err != nil {
		return nil, err
	}
	ids := []uint{}
	for _, employee := range month.Employees {
		ids = append(ids, employee.ID)
	}
	var candidates []models.EmployeeCard
	err := db.Where("id IN ? OR (agreement_id = ? AND status = ?) OR (status = ? AND dismissal_date >= ?)", ids, agreementID, false, true, fmt.Sprintf("%04d-%02d-01", month.Year, month.Month)).Order("surname, name, id").Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	result := []models.EmployeeCard{}
	members := map[uint]bool{}
	for _, id := range ids {
		members[id] = true
	}
	for _, employee := range candidates {
		if members[employee.ID] || timeSheetEmployeeEligible(employee, month.Year, month.Month) {
			result = append(result, employee)
		}
	}
	return result, nil
}
func SaveTimeSheetEmployeeList(db *gorm.DB, agreementID, monthID uint, enabledIDs []uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var month models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).Preload("Employees").First(&month, monthID).Error; err != nil {
			return err
		}
		candidates, err := GetTimeSheetEmployeeList(tx, agreementID, monthID)
		if err != nil {
			return err
		}
		allowed := map[uint]models.EmployeeCard{}
		for _, employee := range candidates {
			allowed[employee.ID] = employee
		}
		members := map[uint]bool{}
		for _, employee := range month.Employees {
			members[employee.ID] = true
		}
		enabled := map[uint]bool{}
		added := []models.EmployeeCard{}
		for _, id := range enabledIDs {
			employee, ok := allowed[id]
			if !ok || enabled[id] {
				return dailyError("Проверьте список сотрудников табеля")
			}
			if !members[id] && !timeSheetEmployeeEligible(employee, month.Year, month.Month) {
				return dailyError("Сотрудник уволен до выбранного месяца")
			}
			enabled[id] = true
			if !members[id] {
				added = append(added, employee)
			}
		}
		if len(added) > 0 {
			if err := tx.Model(&month).Association("Employees").Append(&added); err != nil {
				return err
			}
		}
		// Association.Append также обновляет month.Employees; перечитываем полный состав.
		if err := tx.Preload("Employees").First(&month, month.ID).Error; err != nil {
			return err
		}
		month.HiddenEmployeeIDs = []uint{}
		for _, employee := range month.Employees {
			if !enabled[employee.ID] {
				month.HiddenEmployeeIDs = append(month.HiddenEmployeeIDs, employee.ID)
			}
		}
		if err := tx.Model(&month).Select("HiddenEmployeeIDs").Updates(&month).Error; err != nil {
			return err
		}
		return SyncDailyAllowanceAccruals(tx, month.ID)
	})
}

// --------- конец нового кода chat
