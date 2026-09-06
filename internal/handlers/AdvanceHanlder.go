package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllAgreementAdvanceHandler(w http.ResponseWriter, r *http.Request) {
	var agreementID = r.PathValue("id")
	var advance []models.Advance

	if err := database.DB.Where("agreement_id = ?", agreementID).Find(&advance).Error; err != nil {
		msg := "[Error] Ошибка при получении данных из базы"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advance)
}
