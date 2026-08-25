package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
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

	if err := database.DB.Create(&timeSheet).Error; err != nil {
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
	for _, timeSheet := range timeSheets {
		if err := database.DB.Model(&models.TimeSheet{}).Where("id = ?", timeSheet.ID).Update("status", timeSheet.Status).Error; err != nil {
			msg := "[Error] Ошибка при записи данных в базу"
			log.Println(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
	}

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

	var ids []uint

	for _, timeSheet := range timeSheets {
		ids = append(ids, timeSheet.ID)
	}

	if err := database.DB.Where("id IN ?", ids).Delete(&models.TimeSheet{}).Error; err != nil {
		msg := "[Error] Ошибка при удалении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheets)
}
