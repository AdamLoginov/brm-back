// --------- новый код chat: согласование и отмена расчёта всего табеля
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

func salaryApprovalResponse(w http.ResponseWriter, result any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или табель не найден", 404)
		case errors.Is(err, service.ErrSalaryApprovalConflict):
			http.Error(w, err.Error(), 409)
		case errors.Is(err, service.ErrSalaryApprovalInput), errors.Is(err, service.ErrSalaryAmountOverflow):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[SalaryApproval]", err)
			http.Error(w, "Не удалось обработать согласование", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func SalaryApprovalHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	monthID, monthErr := strconv.ParseUint(r.PathValue("monthId"), 10, 32)
	if err != nil || monthErr != nil || agreementID == 0 || monthID == 0 {
		http.Error(w, "Некорректный договор или табель", 400)
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	if r.Method == http.MethodPost {
		var input service.SalaryApprovalInput
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные согласования", 400)
			return
		}
		result, err := service.ApproveSalary(database.DB, uint(agreementID), uint(monthID), input)
		salaryApprovalResponse(w, result, err)
	} else if r.Method == http.MethodDelete {
		var input struct {
			ApprovalID uint `json:"approval_id"`
		}
		if decoder.Decode(&input) != nil || input.ApprovalID == 0 || decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Укажите согласование", 400)
			return
		}
		err := service.CancelSalaryApproval(database.DB, uint(agreementID), uint(monthID), input.ApprovalID)
		salaryApprovalResponse(w, map[string]bool{"cancelled": err == nil}, err)
	} else {
		http.Error(w, "Метод не поддерживается", 405)
	}
}

// --------- конец нового кода chat
