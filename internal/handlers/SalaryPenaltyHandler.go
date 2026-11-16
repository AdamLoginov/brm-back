// --------- новый код chat: API штрафов
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

func SalaryPenaltyHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный ID договора", 400)
		return
	}
	var penaltyID uint64
	if value := r.PathValue("penaltyId"); value != "" {
		penaltyID, err = strconv.ParseUint(value, 10, 32)
		if err != nil || penaltyID == 0 {
			http.Error(w, "Некорректный ID штрафа", 400)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		result, err = service.GetSalaryPenalties(database.DB, uint(agreementID))
	// --------- новый код chat: отдельный запрос изменения статуса
	case http.MethodPatch:
		var input struct {
			Status string `json:"status"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || penaltyID == 0 {
			http.Error(w, "Некорректные данные статуса штрафа", 400)
			return
		}
		result, err = service.ChangeSalaryPenaltyStatus(database.DB, uint(agreementID), uint(penaltyID), input.Status)
	// --------- конец нового кода chat
	case http.MethodPost:
		var input service.SalaryPenaltyInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			http.Error(w, "Некорректные данные штрафа", 400)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные штрафа", 400)
			return
		}
		result, err = service.SaveSalaryPenalty(database.DB, uint(agreementID), uint(penaltyID), input)
	case http.MethodDelete:
		err = service.DeleteSalaryPenalty(database.DB, uint(agreementID), uint(penaltyID))
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или штраф не найден", 404)
		case errors.Is(err, service.ErrSalaryPenaltyInput):
			http.Error(w, err.Error(), 400)
		case errors.Is(err, service.ErrSalaryPenaltyStatus): // новый код chat: запрещённый переход
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			log.Println("[SalaryPenalty]", err)
			http.Error(w, "Не удалось обработать штраф", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
