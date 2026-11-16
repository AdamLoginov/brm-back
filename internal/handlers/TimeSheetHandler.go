package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service" // новый код chat: синхронизация штрафов
)

func GetAllTimeSheethandler(w http.ResponseWriter, r *http.Request) {
	var timeSheets []models.TimeSheet

	if err := database.DB.Find(&timeSheets).Error; err != nil {
		msg := "[Error] Ошибка при получении данных из базы"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}

func GetMonthAgreementTimeSheethandler(w http.ResponseWriter, r *http.Request) {
	agreementID := r.PathValue("id")
	month := r.URL.Query().Get("month")

	var timeSheets []models.TimeSheet

	if len(month) == 1 {
		month = "0" + month
	}

	if err := database.DB.Preload("EmployeeCard").Where("agreement_id = ?", agreementID).Where("date LIKE ?", "%-"+month+"-%").Find(&timeSheets).Error; err != nil {
		msg := "[Error] Ошибка при получении данных из базы"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}

func GetAllAgreementTimeSheethandler(w http.ResponseWriter, r *http.Request) {
	var agreementID = r.PathValue("id")
	var timeSheets []models.TimeSheet

	if err := database.DB.Where("agreement_id = ?", agreementID).Find(&timeSheets).Error; err != nil {
		msg := "[Error] Ошибка при получении данных из базы"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}

// func UpdateTimeSheetHandler(w http.ResponseWriter, r *http.Request) {
// 	var timeSheets []models.TimeSheet

// 	if err := json.NewDecoder(r.Body).Decode(&timeSheets); err != nil {
// 		msg := "[Error] Ошибка при чтении данных"
// 		log.Println(msg)
// 		http.Error(w, msg, http.StatusBadRequest)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(timeSheets)
// }

func CreateTimeSheetHandler(w http.ResponseWriter, r *http.Request) {
	var timeSheet []models.TimeSheet

	if err := json.NewDecoder(r.Body).Decode(&timeSheet); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := service.ChangeTimeSheetEntries(database.DB, timeSheet, "create"); err != nil { // новый код chat: отметки и штрафы в транзакции
		msg := "[Error] Ошибка при записи данных в базу"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheet)
}

func UpdateTimeSheetHandler(w http.ResponseWriter, r *http.Request) {
	var timeSheets []models.TimeSheet

	if err := json.NewDecoder(r.Body).Decode(&timeSheets); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	// --------- новый код chat: смена отметок и удаление / создание штрафов вместе
	if err := service.ChangeTimeSheetEntries(database.DB, timeSheets, "update"); err != nil {
		log.Println(err)
		http.Error(w, "Не удалось обновить табель и штрафы", http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}

func DeleteTimeSheetHandler(w http.ResponseWriter, r *http.Request) {
	var timeSheets []models.TimeSheet

	if err := json.NewDecoder(r.Body).Decode(&timeSheets); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// --------- новый код chat: очистка ячеек удаляет связанные штрафы любого статуса
	if err := service.ChangeTimeSheetEntries(database.DB, timeSheets, "delete"); err != nil {
		log.Println(err)
		http.Error(w, "Не удалось удалить отметки и штрафы", http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}
