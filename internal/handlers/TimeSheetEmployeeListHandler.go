// --------- новый код chat: управление видимостью сотрудников месячного табеля
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"io"
	"net/http"
	"project/internal/database"
	"project/internal/service"
)

func TimeSheetEmployeeListHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, monthID, decoder, ok := salaryEmployeeRequest(w, r)
	if !ok {
		return
	}
	var result any
	var err error
	if r.Method == http.MethodGet {
		result, err = service.GetTimeSheetEmployeeList(database.DB, agreementID, monthID)
	} else {
		var input struct {
			EnabledIDs *[]uint `json:"enabled_ids"`
		}
		if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.EnabledIDs == nil {
			http.Error(w, "Укажите список сотрудников", 400)
			return
		}
		err = service.SaveTimeSheetEmployeeList(database.DB, agreementID, monthID, *input.EnabledIDs)
		result = map[string]bool{"saved": err == nil}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Табель не найден", 404)
		} else if errors.Is(err, service.ErrDailyAllowanceInput) {
			http.Error(w, err.Error(), 400)
		} else {
			http.Error(w, "Не удалось обновить список сотрудников", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
