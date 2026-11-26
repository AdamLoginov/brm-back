// --------- новый код chat: индивидуальные выплаты и согласования
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"project/internal/database"
	"project/internal/middleware" // новый код chat: автор массового согласования
	"project/internal/service"
	"strconv"
)

func salaryEmployeeRequest(w http.ResponseWriter, r *http.Request) (uint, uint, *json.Decoder, bool) {
	a, e := strconv.ParseUint(r.PathValue("id"), 10, 32)
	m, me := strconv.ParseUint(r.PathValue("monthId"), 10, 32)
	if e != nil || me != nil || a == 0 || m == 0 {
		http.Error(w, "Некорректные данные запроса", 400)
		return 0, 0, nil, false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	d.DisallowUnknownFields()
	return uint(a), uint(m), d, true
}
func SalaryPaymentDraftHandler(w http.ResponseWriter, r *http.Request) {
	a, m, d, ok := salaryEmployeeRequest(w, r)
	if !ok {
		return
	}
	var input service.SalaryPaymentDraftInput
	if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные запроса", 400)
		return
	}
	result, err := service.SaveSalaryPaymentDraft(database.DB, a, m, input)
	salaryApprovalResponse(w, result, err)
}
func SalaryEmployeeApprovalHandler(w http.ResponseWriter, r *http.Request) {
	a, m, d, ok := salaryEmployeeRequest(w, r)
	if !ok {
		return
	}
	var input service.SalaryEmployeeApprovalInput
	if d.Decode(&input) != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные запроса", 400)
		return
	}
	input.ActorID = middleware.CurrentUserID(r) // новый код chat
	result, err := service.ApplySalaryEmployeeApprovals(database.DB, a, m, input)
	salaryApprovalResponse(w, result, err)
}

// --------- новый код chat: индивидуальные выплаты и согласования
