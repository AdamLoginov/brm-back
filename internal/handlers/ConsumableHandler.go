// --------- новый код chat: учёт расходников без истории движения
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var consumableMutex sync.Mutex
var errConsumableInput = errors.New("Некорректные данные расходника")
var errConsumableCategoryUsed = errors.New("Категория используется расходниками. Сначала измените их категорию")
var errConsumableCategoryDuplicate = errors.New("Категория с таким названием уже существует")

func consumableID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, errConsumableInput
	}
	return uint(id), nil
}
func decodeConsumable(w http.ResponseWriter, r *http.Request, input any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные запроса", 400)
		return false
	}
	return true
}
func consumableResponse(w http.ResponseWriter, result any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Запись, договор или категория не найдены", 404)
		case errors.Is(err, errConsumableInput):
			http.Error(w, err.Error(), 400)
		case errors.Is(err, errConsumableCategoryUsed), errors.Is(err, errConsumableCategoryDuplicate):
			http.Error(w, err.Error(), 409)
		default:
			log.Println("[Consumable]", err)
			http.Error(w, "Не удалось обработать расходники", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func ListConsumables(w http.ResponseWriter, r *http.Request) {
	rows := []models.Consumable{}
	query := database.DB
	if r.PathValue("id") != "" {
		id, err := consumableID(r.PathValue("id"))
		if err != nil {
			consumableResponse(w, nil, err)
			return
		}
		query = query.Where("agreement_id = ?", id)
	}
	err := query.Order("name, id").Find(&rows).Error
	consumableResponse(w, rows, err)
}
func SaveConsumable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		AddedDate   string `json:"added_date"`
		Quantity    int64  `json:"quantity"`
		AgreementID *uint  `json:"agreement_id"`
		CategoryID  *uint  `json:"category_id"`
	}
	if !decodeConsumable(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	date, err := time.Parse("2006-01-02", input.AddedDate)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 500 || input.Quantity <= 0 || input.Quantity > 9007199254740991 || err != nil || date.Year() < 1 {
		http.Error(w, "Укажите наименование (до 500 символов), корректную дату и положительное целое количество", 400)
		return
	}
	if input.AgreementID != nil && *input.AgreementID == 0 {
		input.AgreementID = nil
	}
	if input.CategoryID != nil && *input.CategoryID == 0 {
		input.CategoryID = nil
	}
	var id uint
	if r.PathValue("id") != "" {
		id, err = consumableID(r.PathValue("id"))
		if err != nil {
			consumableResponse(w, nil, err)
			return
		}
	}
	consumableMutex.Lock()
	defer consumableMutex.Unlock()
	var row models.Consumable
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if id != 0 {
			if err := tx.First(&row, id).Error; err != nil {
				return err
			}
		}
		if input.AgreementID != nil {
			if err := tx.First(&models.Agreement{}, *input.AgreementID).Error; err != nil {
				return err
			}
		}
		if input.CategoryID != nil {
			if err := tx.First(&models.ConsumableCategory{}, *input.CategoryID).Error; err != nil {
				return err
			}
		}
		row.Name, row.AddedDate, row.Quantity = input.Name, input.AddedDate, input.Quantity
		row.AgreementID, row.CategoryID = input.AgreementID, input.CategoryID
		return tx.Save(&row).Error
	})
	consumableResponse(w, row, err)
}
func DeleteConsumable(w http.ResponseWriter, r *http.Request) {
	id, err := consumableID(r.PathValue("id"))
	if err != nil {
		consumableResponse(w, nil, err)
		return
	}
	consumableMutex.Lock()
	defer consumableMutex.Unlock()
	result := database.DB.Delete(&models.Consumable{}, id)
	err = result.Error
	if err == nil && result.RowsAffected == 0 {
		err = gorm.ErrRecordNotFound
	}
	consumableResponse(w, map[string]bool{"deleted": err == nil}, err)
}

// --------- конец нового кода chat
