// --------- новый код chat: API премий
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/service"
	"strconv"
)

func SalaryBonusHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный ID договора", 400)
		return
	}
	var bonusID uint64
	if value := r.PathValue("bonusId"); value != "" {
		bonusID, err = strconv.ParseUint(value, 10, 32)
		if err != nil || bonusID == 0 {
			http.Error(w, "Некорректный ID премии", 400)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		result, err = service.GetSalaryBonuses(database.DB, uint(agreementID))
	case http.MethodPost:
		var input service.SalaryBonusInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			http.Error(w, "Некорректные данные премии", 400)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные премии", 400)
			return
		}
		result, err = service.SaveSalaryBonus(database.DB, uint(agreementID), uint(bonusID), input)
		// --------- новый код chat: ручная смена статуса выплаты
	case http.MethodPatch:
		var input struct {
			IsPaid *bool `json:"is_paid"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if bonusID == 0 || decoder.Decode(&input) != nil || input.IsPaid == nil || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Укажите статус выплаты премии", 400)
			return
		}
		result, err = service.SetSalaryBonusPaid(database.DB, uint(agreementID), uint(bonusID), *input.IsPaid)
	// --------- конец нового кода chat
	case http.MethodDelete:
		err = service.DeleteSalaryBonus(database.DB, uint(agreementID), uint(bonusID))
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или премия не найден", 404)
		case errors.Is(err, service.ErrSalaryBonusInput):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[SalaryBonus]", err)
			http.Error(w, "Не удалось обработать премия", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
