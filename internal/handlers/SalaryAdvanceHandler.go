// --------- новый код chat: API авансов
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

func SalaryAdvanceHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный ID договора", 400)
		return
	}
	var advanceID uint64
	if value := r.PathValue("advanceId"); value != "" {
		advanceID, err = strconv.ParseUint(value, 10, 32)
		if err != nil || advanceID == 0 {
			http.Error(w, "Некорректный ID аванса", 400)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		result, err = service.GetSalaryAdvances(database.DB, uint(agreementID))
	case http.MethodPost:
		var input service.SalaryAdvanceInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			http.Error(w, "Некорректные данные аванса", 400)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные аванса", 400)
			return
		}
		result, err = service.SaveSalaryAdvance(database.DB, uint(agreementID), uint(advanceID), input)
	case http.MethodDelete:
		err = service.DeleteSalaryAdvance(database.DB, uint(agreementID), uint(advanceID))
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или аванс не найден", 404)
		case errors.Is(err, service.ErrSalaryAdvanceInput):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[SalaryAdvance]", err)
			http.Error(w, "Не удалось обработать аванс", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
