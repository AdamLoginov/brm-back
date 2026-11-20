// --------- новый код chat: поступления и использование — единственный источник остатка
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"time"
)

func WarehouseBalance(db *gorm.DB, materialID uint) (int64, error) {
	var received, used int64
	if err := db.Model(&models.WarehouseReceipt{}).Select("COALESCE(SUM(quantity_thousandths),0)").Where("material_id = ?", materialID).Scan(&received).Error; err != nil {
		return 0, err
	}
	if err := db.Model(&models.WorkOrderMaterial{}).Select("COALESCE(SUM(quantity_thousandths),0)").Where("material_id = ?", materialID).Scan(&used).Error; err != nil {
		return 0, err
	}
	return received - used, nil
}

type WarehouseRow struct {
	models.WarehouseMaterial
	Balance  int64 `json:"balance"`
	Received int64 `json:"received"`
	Used     int64 `json:"used"`
}
type WarehouseUsage struct {
	models.WorkOrderMaterial
	TaskName       string `json:"task_name"`
	ShiftID        uint   `json:"shift_id"`
	BookID         uint   `json:"book_id"`
	Date           string `json:"date"`
	Number         int    `json:"number"`
	PositionNumber string `json:"position_number"`
	PositionName   string `json:"position_name"`
	BlockName      string `json:"block_name"`
}

func LoadWarehouse(db *gorm.DB, agreementID uint) ([]WarehouseRow, error) {
	rows := []WarehouseRow{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var materials []models.WarehouseMaterial
		if err := tx.Where("agreement_id = ?", agreementID).Order("name").Find(&materials).Error; err != nil {
			return err
		}
		for _, m := range materials {
			r := WarehouseRow{WarehouseMaterial: m}
			if err := tx.Model(&models.WarehouseReceipt{}).Select("COALESCE(SUM(quantity_thousandths),0)").Where("material_id = ?", m.ID).Scan(&r.Received).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.WorkOrderMaterial{}).Select("COALESCE(SUM(quantity_thousandths),0)").Where("material_id = ?", m.ID).Scan(&r.Used).Error; err != nil {
				return err
			}
			r.Balance = r.Received - r.Used
			rows = append(rows, r)
		}
		return nil
	})
	return rows, err
}
func WarehouseUsages(db *gorm.DB, materialID uint) ([]WarehouseUsage, error) {
	rows := []WarehouseUsage{}
	err := db.Model(&models.WorkOrderMaterial{}).Select("work_order_materials.*, t.name AS task_name, t.shift_id, d.book_id, d.date, s.number, p.number AS position_number, p.name AS position_name, b.name AS block_name").
		Joins("JOIN work_order_tasks t ON t.id = work_order_materials.task_id JOIN work_order_shifts s ON s.id = t.shift_id JOIN work_order_days d ON d.id = s.day_id LEFT JOIN agreement_work_positions p ON p.id = t.position_id LEFT JOIN agreement_work_blocks b ON b.id = COALESCE(NULLIF(t.block_id, 0), p.block_id)"). // новый код chat: история включает работы без позиции и блока
		Where("material_id = ?", materialID).Order("d.date, s.number, work_order_materials.id").Scan(&rows).Error
	return rows, err
}

// При удалении использования увеличиваем версию затронутых смен: открытая старая форма не вернёт списания.
func removeWarehouseUsages(tx *gorm.DB, materialID uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var usages []models.WorkOrderMaterial
	if err := tx.Where("material_id = ? AND id IN ?", materialID, ids).Find(&usages).Error; err != nil {
		return err
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if len(usages) != len(seen) {
		return errors.New("Выбранные списания изменились. Обновите историю")
	}
	tasks := []uint{}
	for _, row := range usages {
		tasks = append(tasks, row.TaskID)
	}
	if err := tx.Model(&models.WorkOrderShift{}).Where("id IN (?)", tx.Model(&models.WorkOrderTask{}).Select("shift_id").Where("id IN ?", tasks)).UpdateColumn("version", gorm.Expr("version + 1")).Error; err != nil {
		return err
	}
	return tx.Where("material_id = ? AND id IN ?", materialID, ids).Delete(&models.WorkOrderMaterial{}).Error
}

type WarehouseReceiptInput struct {
	Date           string `json:"date"`
	Quantity       string `json:"quantity"`
	RemoveUsageIDs []uint `json:"remove_usage_ids"`
}

func SaveWarehouseReceipt(db *gorm.DB, agreementID, materialID, receiptID uint, input WarehouseReceiptInput, remove bool) error {
	return WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		var material models.WarehouseMaterial
		if err := tx.Where("id = ? AND agreement_id = ?", materialID, agreementID).First(&material).Error; err != nil {
			return err
		}
		row := models.WarehouseReceipt{MaterialID: materialID}
		if receiptID != 0 {
			if err := tx.Where("id = ? AND material_id = ?", receiptID, materialID).First(&row).Error; err != nil {
				return err
			}
		}
		if remove && receiptID == 0 {
			return errors.New("Не указано поступление")
		}
		if !remove {
			date, err := time.Parse("2006-01-02", input.Date)
			if err != nil || date.Year() < 1900 || date.Year() > 2200 {
				return errors.New("Некорректная дата поступления")
			}
			amount, err := ParseWorkQuantity(input.Quantity, 3, false)
			if err != nil {
				return err
			}
			row.Date = input.Date
			row.QuantityThousandths = amount
		}
		if err := removeWarehouseUsages(tx, materialID, input.RemoveUsageIDs); err != nil {
			return err
		}
		if remove {
			if err := tx.Delete(&row).Error; err != nil {
				return err
			}
		} else if err := tx.Save(&row).Error; err != nil {
			return err
		}
		balance, err := WarehouseBalance(tx, materialID)
		if err != nil {
			return err
		}
		if balance < 0 {
			return errors.New("Недостаточно остатка: выберите списания для удаления из работ")
		}
		return nil
	})
}
func DeleteWarehouseMaterial(db *gorm.DB, agreementID, materialID uint) error {
	return WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		var material models.WarehouseMaterial
		if err := tx.Where("id = ? AND agreement_id = ?", materialID, agreementID).First(&material).Error; err != nil {
			return err
		}
		var ids []uint
		if err := tx.Model(&models.WorkOrderMaterial{}).Where("material_id = ?", materialID).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if err := removeWarehouseUsages(tx, materialID, ids); err != nil {
			return err
		}
		if err := tx.Where("material_id = ?", materialID).Delete(&models.WarehouseReceipt{}).Error; err != nil {
			return err
		}
		return tx.Delete(&material).Error
	})
}

// --------- конец нового кода chat
