// --------- новый код chat: история согласований по выбранному договору и месяцу
package handlers

import (
	"net/http"
	"project/internal/database"
	"project/internal/service"
)

func SalaryApprovalHistoryHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, monthID, _, ok := salaryEmployeeRequest(w, r)
	if !ok {
		return
	}
	result, err := service.GetSalaryApprovalHistory(database.DB, agreementID, monthID)
	salaryApprovalResponse(w, result, err)
}

// --------- конец нового кода chat
