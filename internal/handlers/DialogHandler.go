package handlers

import (
	"encoding/json"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllDialogHandler(w http.ResponseWriter, r *http.Request) {
	var dialogs []models.Dialog
	if err := database.DB.Preload("Emails").Find(&dialogs).Error; err != nil {
		http.Error(w, "Ошибка при получения данных из базы", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialogs)
}

func GetDetailDialogHandler(w http.ResponseWriter, r *http.Request) {
	var dialogId = r.PathValue("id")
	var dialog models.Dialog

	if err := database.DB.Preload("Emails").First(&dialog, dialogId).Error; err != nil {
		http.Error(w, "Ошибка при поиске диалога", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dialog)
}

func CreateDialogHandler(w http.ResponseWriter, r *http.Request) {
	var dialog models.Dialog
	if err := json.NewDecoder(r.Body).Decode(&dialog); err != nil {
		http.Error(w, "Ошибка в данных", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&dialog).Error; err != nil {
		http.Error(w, "Ошибка при сохранении данных", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Диалог успешно создан!"})
}
