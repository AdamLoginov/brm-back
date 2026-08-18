package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAgreementExpensesHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var agreement models.Agreement

	if err := database.DB.Preload("Expenses.EmployeeCard").First(&agreement, agreementId).Error; err != nil {
		log.Println("[Error] Ошибка в получении данных")
		http.Error(w, "Ошибка при получении данных", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agreement)
}

func DeleteExpensesHandleer(w http.ResponseWriter, r *http.Request) {
	var expensesId = r.PathValue("id")

	if err := database.DB.Delete(&models.Expenses{}, expensesId).Error; err != nil {
		log.Println("[ERROR] Ошибка при удалении элемента расхода")
		http.Error(w, "Ошибка при удалении", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "элемент расходов успешно удален!"})
}

func CreateExpensesHandler(w http.ResponseWriter, r *http.Request) {
	var expenses models.Expenses

	if err := json.NewDecoder(r.Body).Decode(&expenses); err != nil {
		log.Println("[ERROR] Ошибка в получении данных при создании затрат")
		http.Error(w, "Ошибка в полученных данных", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&expenses).Error; err != nil {
		log.Println("[ERROR] Ошибка при сохранении данных данных при создании затрат")
		http.Error(w, "Ошибка при создании расхода", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "элемент трат успешно создан!"})
}
