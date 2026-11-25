// --------- новый код chat: API таблицы и выдач суточных
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

func dailyAllowanceID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, service.ErrDailyAllowanceInput
	}
	return uint(id), nil
}
func dailyAllowanceResponse(w http.ResponseWriter, result any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор, табель или выдача не найдены", 404)
		case errors.Is(err, service.ErrDailyAllowanceInput), errors.Is(err, service.ErrSalaryAmountOverflow):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[DailyAllowance]", err)
			http.Error(w, "Не удалось обработать суточные. Повторите запрос", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func decodeDailyAllowance(w http.ResponseWriter, r *http.Request, input any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные запроса", 400)
		return false
	}
	return true
}
func DailyAllowanceTableHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := dailyAllowanceID(r.PathValue("id"))
	if err != nil {
		dailyAllowanceResponse(w, nil, err)
		return
	}
	monthID, err := dailyAllowanceID(r.URL.Query().Get("month_id"))
	if err != nil {
		dailyAllowanceResponse(w, nil, err)
		return
	}
	result, err := service.GetDailyAllowanceTable(database.DB, agreementID, monthID, r.URL.Query().Get("through"), service.DailyAllowanceToday())
	dailyAllowanceResponse(w, result, err)
}

// --------- новый код chat: сохранение состава суточных для всех пользователей
func DailyAllowanceListHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := dailyAllowanceID(r.PathValue("id"))
	if err != nil {
		dailyAllowanceResponse(w, nil, err)
		return
	}
	var input struct {
		MonthID    uint   `json:"month_id"`
		EnabledIDs []uint `json:"enabled_ids"`
	}
	if !decodeDailyAllowance(w, r, &input) {
		return
	}
	if input.MonthID == 0 || input.EnabledIDs == nil {
		dailyAllowanceResponse(w, nil, service.ErrDailyAllowanceInput)
		return
	}
	err = service.SaveDailyAllowanceList(database.DB, agreementID, input.MonthID, input.EnabledIDs)
	dailyAllowanceResponse(w, map[string]bool{"saved": err == nil}, err)
}

// --------- конец нового кода chat
// --------- новый код chat: удаление всех выдач за день одной транзакцией
func DailyAllowancePaymentDateHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := dailyAllowanceID(r.PathValue("id"))
	if err != nil {
		dailyAllowanceResponse(w, nil, err)
		return
	}
	count, err := service.DeleteDailyAllowancePaymentsByDate(database.DB, agreementID, r.PathValue("date"))
	dailyAllowanceResponse(w, map[string]int64{"deleted_count": count}, err)
}

// --------- конец нового кода chat

func DailyAllowancePaymentHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := dailyAllowanceID(r.PathValue("id"))
	if err != nil {
		dailyAllowanceResponse(w, nil, err)
		return
	}
	var paymentID uint
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		paymentID, err = dailyAllowanceID(r.PathValue("paymentId"))
		if err != nil {
			dailyAllowanceResponse(w, nil, err)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		result, err = service.GetDailyAllowancePayments(database.DB, agreementID)
	case http.MethodPost:
		var input service.DailyAllowanceBatchInput
		if !decodeDailyAllowance(w, r, &input) {
			return
		}
		result, err = service.CreateDailyAllowancePayments(database.DB, agreementID, input)
	case http.MethodPut:
		var input service.DailyAllowancePaymentUpdate
		if !decodeDailyAllowance(w, r, &input) {
			return
		}
		result, err = service.UpdateDailyAllowancePayment(database.DB, agreementID, paymentID, input)
	case http.MethodDelete:
		err = service.DeleteDailyAllowancePayment(database.DB, agreementID, paymentID)
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	dailyAllowanceResponse(w, result, err)
}

// --------- конец нового кода chat
