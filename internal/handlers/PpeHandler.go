// --------- новый код chat: учёт СИЗ с точной денежной суммой и ответственным
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
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ppePricePattern = regexp.MustCompile(`^[0-9]+([.,][0-9]{1,2})?$`)
var errPpeInput = errors.New("Некорректные данные СИЗ")
var errPpeDismissed = errors.New("Уволенного сотрудника нельзя назначить ответственным")

func ppeID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, errPpeInput
	}
	return uint(id), nil
}
func ppePrice(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if !ppePricePattern.MatchString(value) {
		return 0, errPpeInput
	}
	parts := strings.Split(strings.ReplaceAll(value, ",", "."), ".")
	cents := "00"
	if len(parts) == 2 {
		cents = parts[1]
		if len(cents) == 1 {
			cents += "0"
		}
	}
	result, err := strconv.ParseInt(strings.TrimLeft(parts[0]+cents, "0"), 10, 64)
	if strings.Trim(parts[0]+cents, "0") == "" {
		return 0, nil
	}
	if err != nil || result > 9007199254740991 {
		return 0, errPpeInput
	}
	return result, nil
}
func ppeResponse(w http.ResponseWriter, result any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "СИЗ, договор или сотрудник не найдены", 404)
		case errors.Is(err, errPpeInput), errors.Is(err, errPpeDismissed):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[Ppe]", err)
			http.Error(w, "Не удалось обработать СИЗ", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func ListPpe(w http.ResponseWriter, r *http.Request) {
	rows := []models.Ppe{}
	query := database.DB.Preload("EmployeeCard")
	if r.PathValue("id") != "" {
		id, err := ppeID(r.PathValue("id"))
		if err != nil {
			ppeResponse(w, nil, err)
			return
		}
		query = query.Where("agreement_id = ?", id)
	}
	err := query.Order("name, id").Find(&rows).Error
	ppeResponse(w, rows, err)
}
func SavePpe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string `json:"name"`
		Size           string `json:"size"`
		Category       string `json:"category"`
		Price          string `json:"price"`
		AgreementID    uint   `json:"agreement_id"`
		EmployeeCardID *uint  `json:"employee_card_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные СИЗ", 400)
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Size = strings.Join(strings.Fields(input.Size), " ")
	price, err := ppePrice(input.Price)
	if err != nil {
		http.Error(w, "Цена должна быть неотрицательной суммой до копеек", 400)
		return
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 500 || utf8.RuneCountInString(input.Size) > 100 || input.AgreementID == 0 {
		http.Error(w, "Укажите наименование (до 500 символов), договор и размер не длиннее 100 символов", 400)
		return
	}
	switch input.Category {
	case "summer", "winter", "demiseason":
	default:
		http.Error(w, "Выберите категорию: летняя, зимняя или демисезонная", 400)
		return
	}
	var id uint
	if r.PathValue("id") != "" {
		id, err = ppeID(r.PathValue("id"))
		if err != nil {
			ppeResponse(w, nil, err)
			return
		}
	}
	if input.EmployeeCardID != nil && *input.EmployeeCardID == 0 {
		input.EmployeeCardID = nil
	}
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	var row models.Ppe
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if id != 0 {
			if err := tx.First(&row, id).Error; err != nil {
				return err
			}
		}
		if err := tx.First(&models.Agreement{}, input.AgreementID).Error; err != nil {
			return err
		}
		if input.EmployeeCardID != nil {
			var employee models.EmployeeCard
			if err := tx.First(&employee, *input.EmployeeCardID).Error; err != nil {
				return err
			}
			if employee.Status {
				return errPpeDismissed
			}
		}
		row.Name, row.Size, row.Category = input.Name, input.Size, input.Category
		row.PriceKopecks, row.AgreementID, row.EmployeeCardID = price, input.AgreementID, input.EmployeeCardID
		return tx.Omit("EmployeeCard").Save(&row).Error
	})
	ppeResponse(w, row, err)
}
func DeletePpe(w http.ResponseWriter, r *http.Request) {
	id, err := ppeID(r.PathValue("id"))
	if err != nil {
		ppeResponse(w, nil, err)
		return
	}
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	result := database.DB.Delete(&models.Ppe{}, id)
	err = result.Error
	if err == nil && result.RowsAffected == 0 {
		err = gorm.ErrRecordNotFound
	}
	ppeResponse(w, map[string]bool{"deleted": err == nil}, err)
}

// --------- конец нового кода chat
