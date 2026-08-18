package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAgreementAllToolHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var tools []models.Tool

	if err := database.DB.Where("agreement_id = ?", agreementId).Find(&tools).Error; err != nil {
		msg := "[Error] Ошибка при получении данных инструмента привзяанного к договору"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tools)
}

func CreateToolHandler(w http.ResponseWriter, r *http.Request) {
	var tool models.Tool

	if err := json.NewDecoder(r.Body).Decode(&tool); err != nil {
		msg := "[Error] Ошибка при получении данных при создании инструмента"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&tool).Error; err != nil {
		msg := "[Error] Ошибка при сохранении данных при создании инструмента"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Инструмент успешно создан!"})
}

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
