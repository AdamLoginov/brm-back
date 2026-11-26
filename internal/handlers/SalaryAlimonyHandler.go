// --------- новый код chat: API алиментов
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

// новый код chat: отдельный маршрут выбора сотрудников
func SalaryAlimonyEmployeesHandler(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("employees", "employees")
	SalaryAlimonyHandler(w, r)
}

func SalaryAlimonyHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный ID договора", 400)
		return
	}
	var alimonyID uint64
	if value := r.PathValue("alimonyId"); value != "" {
		alimonyID, err = strconv.ParseUint(value, 10, 32)
		if err != nil || alimonyID == 0 {
			http.Error(w, "Некорректный ID алиментов", 400)
			return
		}
	}
	var result any
	switch r.Method {
	case http.MethodGet:
		if r.PathValue("employees") == "employees" {
			result, err = service.GetSalaryAlimonyEmployees(database.DB, uint(agreementID))
		} else {
			result, err = service.GetSalaryAlimonies(database.DB, uint(agreementID))
		}
	case http.MethodPost:
		var input service.SalaryAlimonyInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil {
			http.Error(w, "Некорректные данные алиментов", 400)
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			http.Error(w, "Некорректные данные алиментов", 400)
			return
		}
		result, err = service.SaveSalaryAlimony(database.DB, uint(agreementID), uint(alimonyID), input)
	case http.MethodDelete:
		err = service.DeleteSalaryAlimony(database.DB, uint(agreementID), uint(alimonyID))
		result = map[string]bool{"deleted": err == nil}
	default:
		http.Error(w, "Метод не поддерживается", 405)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Договор или запись алиментов не найдены", 404)
		case errors.Is(err, service.ErrSalaryAlimonyInput):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[SalaryAlimony]", err)
			http.Error(w, "Не удалось обработать алименты", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
