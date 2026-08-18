package handlers

import (
	"encoding/json"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"time"
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
	var input struct {
		Name     string  `json:"name"`
		Number   string  `json:"number"`
		Customer string  `json:"customer"`
		Address  string  `json:"address"`
		Price    float64 `json:"price"`
		DueDate  string  `json:"due_date"`
	}

	json.NewDecoder(r.Body).Decode(&input)

	DueDateTime, _ := time.Parse("2006-01-02", input.DueDate)

	if err := database.DB.Create(&models.Agreement{
		Name:     input.Name,
		Number:   input.Number,
		Customer: input.Customer,
		Address:  input.Address,
		Price:    input.Price,
		DueDate:  DueDateTime,
	}).Error; err != nil {
		http.Error(w, "Не удалось создать договор", http.StatusInternalServerError)
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
