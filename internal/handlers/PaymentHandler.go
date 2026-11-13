package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"

	"gorm.io/gorm"
)

func GetAllPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var payments []models.Payment

	if err := database.DB.Preload("EmployeeCard").Find(&payments).Error; err != nil {
		msg := "[Error PH-3]:"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func CreatePaymentHandler(w http.ResponseWriter, r *http.Request) {
	var payment models.Payment

	if err := json.NewDecoder(r.Body).Decode(&payment); err != nil {
		msg := "[Error PH-1]:"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&payment).Error; err != nil {
			log.Println("[Error PH-2]:")
			return err
		}
		log.Println("[Sucsess PH-2]:")
		return nil
	}); err != nil {
		msg := "[Error PH-2]:"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
}
