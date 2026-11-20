// --------- новый код chat: API склада договора
package handlers

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
)

func WarehouseHandler(w http.ResponseWriter, r *http.Request) {
	a := workAgreement(w, r)
	if a == 0 {
		return
	}
	id := workID(r, "materialId")
	if r.PathValue("materialId") != "" && id == 0 { // новый код chat: неверный ID не превращается в создание
		http.Error(w, "Некорректный материал", 400)
		return
	}
	if r.Method == "GET" && id == 0 {
		rows, err := service.LoadWarehouse(database.DB, a)
		if err != nil {
			workError(w, err)
			return
		}
		workJSON(w, rows)
		return
	}
	var material models.WarehouseMaterial
	if id != 0 {
		if err := database.DB.Where("id = ? AND agreement_id = ?", id, a).First(&material).Error; err != nil {
			workError(w, err)
			return
		}
	}
	if r.Method == "GET" {
		var receipts = []models.WarehouseReceipt{}
		var usages []service.WarehouseUsage
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("material_id = ?", id).Order("date DESC, id DESC").Find(&receipts).Error; err != nil {
				return err
			}
			var err error
			usages, err = service.WarehouseUsages(tx, id)
			return err
		})
		if err != nil {
			workError(w, err)
			return
		}
		workJSON(w, map[string]interface{}{"receipts": receipts, "usages": usages})
		return
	}
	if r.PathValue("action") == "receipts" {
		receiptID := workID(r, "receiptId")
		if id == 0 || ((r.Method == "PATCH" || r.Method == "DELETE") && receiptID == 0) {
			http.Error(w, "Не указано поступление", 400)
			return
		}
		var input service.WarehouseReceiptInput
		if !workDecode(w, r, &input) {
			return
		}
		if err := service.SaveWarehouseReceipt(database.DB, a, id, receiptID, input, r.Method == "DELETE"); err != nil {
			workError(w, err)
			return
		}
		workJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method == "DELETE" {
		if id == 0 {
			http.Error(w, "Не указан материал", 400)
			return
		}
		if err := service.DeleteWarehouseMaterial(database.DB, a, id); err != nil {
			workError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	var input workReferenceInput
	if !workDecode(w, r, &input) {
		return
	}
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	err := service.WorkTransaction(database.DB, a, func(tx *gorm.DB) error {
		if id != 0 {
			if err := tx.Where("id = ? AND agreement_id = ?", id, a).First(&material).Error; err != nil {
				return err
			}
		}
		if input.Name == "" || len(input.Name) > 500 || input.WorkshopID == 0 || input.UnitID == 0 {
			return errors.New("Укажите название, цех и единицу измерения")
		}
		var shop models.AgreementWorkshop
		var unit models.MeasurementUnit
		if err := tx.Where("id = ? AND agreement_id = ?", input.WorkshopID, a).First(&shop).Error; err != nil {
			return err
		}
		if err := tx.First(&unit, input.UnitID).Error; err != nil {
			return err
		}
		if id != 0 && (material.UnitID != input.UnitID || material.WorkshopID != input.WorkshopID) {
			var count int64
			for _, m := range []interface{}{&models.WarehouseReceipt{}, &models.WorkOrderMaterial{}} {
				if err := tx.Model(m).Where("material_id = ?", id).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return errors.New("Материал с движениями нельзя перенести или изменить его единицу измерения")
				}
			}
		}
		material.AgreementID = a
		material.Name = input.Name
		material.NameKey = strings.ToLower(input.Name)
		material.WorkshopID = input.WorkshopID
		material.UnitID = input.UnitID
		return tx.Save(&material).Error
	})
	if err != nil {
		workError(w, err)
		return
	}
	workJSON(w, material)
}

// --------- конец нового кода chat
