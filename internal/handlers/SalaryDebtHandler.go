// --------- новый код chat: API долгов
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

func SalaryDebtHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный ID договора", 400)
		return
	}
	var debtID uint64
	if value := r.PathValue("debtId"); value != "" {
		debtID, err = strconv.ParseUint(value, 10, 32)
		if err != nil || debtID == 0 {
			http.Error(w, "Некорректный ID долга", 400)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		result, err = service.GetSalaryDebts(database.DB, uint(agreementID))
	case http.MethodPost:
		var input service.SalaryDebtInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			http.Error(w, "Некорректные данные долга", 400)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные долга", 400)
			return
		}
		result, err = service.SaveSalaryDebt(database.DB, uint(agreementID), uint(debtID), input)
		// --------- новый код chat: ручная смена статуса выплаты
	case http.MethodPatch:
		var input struct {
			IsClosed *bool `json:"is_closed"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if debtID == 0 || decoder.Decode(&input) != nil || input.IsClosed == nil || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Укажите статус выплаты долга", 400)
			return
		}
		result, err = service.SetSalaryDebtClosed(database.DB, uint(agreementID), uint(debtID), *input.IsClosed)
	// --------- конец нового кода chat
	case http.MethodDelete:
		err = service.DeleteSalaryDebt(database.DB, uint(agreementID), uint(debtID))
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или долг не найден", 404)
		case errors.Is(err, service.ErrSalaryDebtInput):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[SalaryDebt]", err)
			http.Error(w, "Не удалось обработать долг", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
