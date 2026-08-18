package handlers

import (
	"encoding/json"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllManageCard(w http.ResponseWriter, r *http.Request) {
	var managerCard []models.ManagerCard

	if err := database.DB.Find(&managerCard).Error; err != nil {
		http.Error(w, "Не удалось найти карточку менеджера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(managerCard)
}

func CreateManagerCrad(w http.ResponseWriter, r *http.Request) {
	var managerCard models.ManagerCard

	if err := json.NewDecoder(r.Body).Decode(&managerCard); err != nil {
		http.Error(w, "Ошибка в полученных данных", http.StatusInternalServerError)
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Ошибка инициализации транзакции", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := tx.Create(&managerCard).Error; err != nil {
		http.Error(w, "Не удалось найти карточку менеджера", http.StatusInternalServerError)
		return
	}

	dialog := models.Dialog{
		ManagerCardID: managerCard.ID,
		EmailTo:       managerCard.Email,
	}
	if err := tx.Create(&dialog).Error; err != nil {
		http.Error(w, "Не удалось создать диалог", http.StatusInternalServerError)
		return
	}

	err := tx.Model(&models.Email{}).
		Where("email_to = ? OR email_from = ?", managerCard.Email, managerCard.Email).
		Updates(map[string]interface{}{"dialog_id": dialog.ID}).Error

	if err != nil {
		http.Error(w, "Ошибка при привязке существующих писем к диалогу", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Не удалось зафиксировать изменения в БД", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка менеджера успешно создан!"})
}

func DeleteManagerCard(w http.ResponseWriter, r *http.Request) {
	var manageCardID = r.PathValue("id")

	if err := database.DB.Delete(&models.ManagerCard{}, manageCardID).Error; err != nil {
		http.Error(w, "Не удалось найти карточку менеджера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка менеджера успешно удалена!"})
}
