package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var timeSheetMonth []models.TimeSheetMonth

	if err := database.DB.Where("agreement_id = ?", agreementId).Find(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка получения данных табелей"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func GetDetailTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	var timeSheetMonthId = r.PathValue("id")
	var timeSheetMonth models.TimeSheetMonth

	if err := database.DB.Preload("Employees").Preload("TimeSheets").Where("id = ?", timeSheetMonthId).Find(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка получения данных табелей"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func CreateTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Month        uint   `json:"month"`
		Year         uint   `json:"year"`
		AgreementId  uint   `json:"agreement_id"`
		EmployeesIds []uint `json:"employee_card_id"`
	}

	var employees []models.EmployeeCard

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Where("id IN ?", input.EmployeesIds).Find(&employees).Error; err != nil {
		msg := "[Error] Ошибка при поиске сотрудников по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
	}

	timeSheetMonth := models.TimeSheetMonth{
		Month:       input.Month,
		Year:        input.Year,
		AgreementID: input.AgreementId,
		Employees:   employees,
	}

	if err := database.DB.Create(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка при записи данных в базу"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func UpdateTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		Month            uint   `json:"month"`
		Year             uint   `json:"year"`
		EmployeesIds     []uint `json:"employee_card_id"`
		TimeSheetMonthId uint   `json:"timesheet_month_id"`
	}
	var timeSheetMonth models.TimeSheetMonth
	var employees []models.EmployeeCard

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.First(&timeSheetMonth, input.TimeSheetMonthId).Error; err != nil {
		msg := "[Error] Ошибка при поиске табеля по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
	}

	if err := database.DB.Where("id IN ?", input.EmployeesIds).Find(&employees).Error; err != nil {
		msg := "[Error] Ошибка при поиске сотрудников по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
	}

	if err := database.DB.Model(&timeSheetMonth).Updates(map[string]interface{}{"month": input.Month, "year": input.Year}).Error; err != nil {
		msg := "[Error] Ошибка при обновлении табеля"
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	if err := database.DB.Model(&timeSheetMonth).Association("Employees").Replace(&employees); err != nil {
		msg := "[Error] Ошибка при обновлении сотрудников табеля"
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Данные успешно обновлены!"})
}

func DeleteTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	timeSheetMonthID := r.PathValue("id")

	var timeSheetMonth models.TimeSheetMonth

	// Находим табель
	if err := database.DB.First(&timeSheetMonth, timeSheetMonthID).Error; err != nil {

		msg := "[Error] Табель не найден"
		log.Println(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	// Soft delete TimeSheets + самого TimeSheetMonth
	if err := database.DB.Select("TimeSheets").Delete(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка при удалении данных"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Табель успешно удален"})
}
