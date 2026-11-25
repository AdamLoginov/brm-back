package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"time"
)

func GetAgreementAllToolHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var tools []models.Tool

	// новый код chat: общий список МТР и прежний список конкретного договора
	query := database.DB.Preload("EmployeeCard")
	if agreementId != "" {
		query = query.Where("agreement_id = ?", agreementId)
	}
	if err := query.Find(&tools).Error; err != nil {
		msg := "[Error] Ошибка при получении данных инструмента привзяанного к договору"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tools)
}

// --------- новый код chat: единая валидация создания и редактирования
func CreateToolHandler(w http.ResponseWriter, r *http.Request) { saveTool(w, r, false) }
func UpdateToolHandler(w http.ResponseWriter, r *http.Request) { saveTool(w, r, true) }
func saveTool(w http.ResponseWriter, r *http.Request, updating bool) {
	var input struct {
		AgreementID        uint   `json:"agreement_id"`
		Name               string `json:"name"`
		InvNumber          string `json:"inv_number"`
		SerialNumber       string `json:"serial_number"`
		Status             string `json:"status"`
		EmployeeCardID     *uint  `json:"employee_card_id"`
		InspectionDate     string `json:"inspection_date"`
		NextInspectionDate string `json:"next_inspection_date"`
	}
	if !decodeDailyAllowance(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || input.AgreementID == 0 {
		http.Error(w, "Укажите наименование и договор", 400)
		return
	}
	switch input.Status {
	case "new", "good", "satisfactory", "broken":
	default:
		http.Error(w, "Некорректный статус инструмента", 400)
		return
	}
	for _, date := range []string{input.InspectionDate, input.NextInspectionDate} {
		if date != "" {
			parsed, err := time.Parse("2006-01-02", date)
			if err != nil || parsed.Year() < 1 {
				http.Error(w, "Укажите корректные даты проверки", 400)
				return
			}
		}
	}
	if input.InspectionDate != "" && input.NextInspectionDate != "" && input.NextInspectionDate < input.InspectionDate {
		http.Error(w, "Следующая проверка не может быть раньше проведённой", 400)
		return
	}
	var id uint64
	if updating {
		var err error
		id, err = strconv.ParseUint(r.PathValue("id"), 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Некорректный инструмент", 400)
			return
		}
	}
	if input.EmployeeCardID != nil && *input.EmployeeCardID == 0 {
		input.EmployeeCardID = nil
	}
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	var tool models.Tool
	inputError := errors.New("Сотрудник уволен: выберите другого ответственного")
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, input.AgreementID).Error; err != nil {
			return err
		}
		if updating {
			if err := tx.Where("agreement_id = ?", input.AgreementID).First(&tool, id).Error; err != nil {
				return err
			}
		}
		if input.EmployeeCardID != nil {
			var employee models.EmployeeCard
			if err := tx.First(&employee, *input.EmployeeCardID).Error; err != nil {
				return err
			}
			if employee.Status {
				return inputError
			}
		}
		tool.AgreementID = input.AgreementID
		tool.Name, tool.InvNumber, tool.SerialNumber, tool.Status = input.Name, input.InvNumber, input.SerialNumber, input.Status
		tool.EmployeeCardID = input.EmployeeCardID
		tool.InspectionDate, tool.NextInspectionDate = input.InspectionDate, input.NextInspectionDate
		return tx.Omit("EmployeeCard").Save(&tool).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Инструмент, договор или сотрудник не найдены", 404)
		} else if errors.Is(err, inputError) {
			http.Error(w, err.Error(), 400)
		} else {
			log.Println("[Tool]", err)
			http.Error(w, "Не удалось сохранить инструмент", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tool)
}

// --------- конец нового кода chat

func DeleteToolHandler(w http.ResponseWriter, r *http.Request) {
	var toolId = r.PathValue("id")

	if err := database.DB.Delete(&models.Tool{}, toolId).Error; err != nil {
		msg := "[Error] Ошибка при удалении данных инструмента"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Инструмент успешно удален!"})
}
