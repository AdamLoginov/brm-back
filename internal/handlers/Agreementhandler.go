package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var agreements []models.Agreement

	if err := database.DB.Find(&agreements).Error; err != nil {
		http.Error(w, "Не удалось получить данные", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agreements)
}

func GetDetailAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var idAgreement = r.PathValue("id")
	var agreement models.Agreement
	if err := database.DB.Preload("Expenses.EmployeeCard").Preload("Order.ManagerCard.Suplers").Preload("Tool").Preload("Estimate").First(&agreement, idAgreement).Error; err != nil {
		http.Error(w, "ошибка в получении договора", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agreement)
}

func CreateAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var agreement models.Agreement

	if err := json.NewDecoder(r.Body).Decode(&agreement); err != nil {
		msg := "[Error] Ошибка чтения данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&agreement).Error; err != nil {
		msg := "[Error] Ошибка при создании договора"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Договор успешно создан!"})
}

func UpdateAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var agreement models.Agreement

	if err := json.NewDecoder(r.Body).Decode(&agreement); err != nil {
		msg := "[Error] Ошибка чтения данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Model(&models.Agreement{}).Where("id = ?", agreement.ID).Select("Name", "ShortName", "Number", "Customer", "Address", "Price", "DateEnd", "Status").Updates(agreement).Error; err != nil {
		msg := "[Error] Ошибка при обновлении данных договора"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Договор успешно создан!"})
}

func DeleteAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var idAgreement = r.PathValue("id")

	if err := database.DB.Delete(&models.Agreement{}, idAgreement).Error; err != nil {
		http.Error(w, "Не удалось удалить договор!", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Успешно удален договор!"})
}
