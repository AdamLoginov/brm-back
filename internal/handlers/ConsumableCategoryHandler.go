// --------- новый код chat: общий редактируемый справочник категорий расходников
package handlers

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strings"
	"unicode/utf8"
)

func ListConsumableCategories(w http.ResponseWriter, r *http.Request) {
	rows := []models.ConsumableCategory{}
	err := database.DB.Order("name_key").Find(&rows).Error
	consumableResponse(w, rows, err)
}
func SaveConsumableCategory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeConsumable(w, r, &input) {
		return
	}
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 100 {
		http.Error(w, "Название должно содержать от 1 до 100 символов", 400)
		return
	}
	var id uint
	var err error
	if r.PathValue("id") != "" {
		id, err = consumableID(r.PathValue("id"))
		if err != nil {
			consumableResponse(w, nil, err)
			return
		}
	}
	consumableMutex.Lock()
	defer consumableMutex.Unlock()
	var row models.ConsumableCategory
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if id != 0 {
			if err := tx.First(&row, id).Error; err != nil {
				return err
			}
		}
		row.Name, row.NameKey = input.Name, strings.ToLower(input.Name)
		var count int64
		if err := tx.Model(&models.ConsumableCategory{}).Where("name_key = ? AND id <> ?", row.NameKey, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errConsumableCategoryDuplicate
		}
		return tx.Save(&row).Error
	})
	if err != nil && (errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint")) {
		err = errConsumableCategoryDuplicate
	}
	consumableResponse(w, row, err)
}
func DeleteConsumableCategory(w http.ResponseWriter, r *http.Request) {
	id, err := consumableID(r.PathValue("id"))
	if err != nil {
		consumableResponse(w, nil, err)
		return
	}
	consumableMutex.Lock()
	defer consumableMutex.Unlock()
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var row models.ConsumableCategory
		if err := tx.First(&row, id).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.Consumable{}).Where("category_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errConsumableCategoryUsed
		}
		return tx.Delete(&row).Error
	})
	consumableResponse(w, map[string]bool{"deleted": err == nil}, err)
}

// --------- конец нового кода chat
