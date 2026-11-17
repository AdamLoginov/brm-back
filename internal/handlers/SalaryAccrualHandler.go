// --------- новый код chat: API начислений
package handlers

import (
	"net/http"
	"project/internal/database"

	"project/internal/service"
	"strconv"
)

func GetSalaryAccrualHandler(w http.ResponseWriter, r *http.Request) {
	salaryAccrualResponse(w, r, false)
}

func CalculateSalaryAccrualHandler(w http.ResponseWriter, r *http.Request) {
	salaryAccrualResponse(w, r, true)
}

func salaryAccrualResponse(w http.ResponseWriter, r *http.Request, calculate bool) {
	agreementID, errAgreement := strconv.ParseUint(r.PathValue("id"), 10, 32)
	monthID, errMonth := strconv.ParseUint(r.PathValue("monthId"), 10, 32)
	if errAgreement != nil || errMonth != nil || agreementID == 0 || monthID == 0 {
		http.Error(w, "Некорректный ID договора или табеля", http.StatusBadRequest)
		return
	}
	// --------- новый код chat: единый расчёт с долгами или сохранённый снимок
	result, err := service.GetSalaryCalculation(database.DB, uint(agreementID), uint(monthID), calculate)
	salaryApprovalResponse(w, result, err)
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
