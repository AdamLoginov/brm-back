// --------- новый код chat: API начислений
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
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
	var rows []models.SalaryAccrual
	var err error
	if calculate {
		rows, err = service.CalculateSalaryAccruals(database.DB, uint(agreementID), uint(monthID))
	} else {
		rows, err = service.GetSalaryAccruals(database.DB, uint(agreementID), uint(monthID))
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "Договор или табель не найден", http.StatusNotFound)
		return
	}
	if errors.Is(err, service.ErrSalaryAmountOverflow) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Println("[SalaryAccrual]", err)
		http.Error(w, "Не удалось обработать начисления", http.StatusInternalServerError)
		return
	}
	// --------- новый код chat: вычет всех неучтённых авансов без изменения начислений
	summary, err := service.AddSalaryAdvances(database.DB, uint(agreementID), rows)
	if err != nil {
		if errors.Is(err, service.ErrSalaryAmountOverflow) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Println("[SalaryAccrual advances]", err)
			http.Error(w, "Не удалось получить авансы", http.StatusInternalServerError)
		}
		return
	}
	// --------- новый код chat: штрафы за месяц и год табеля
	penaltySummary, err := service.AddSalaryPenalties(database.DB, uint(agreementID), uint(monthID), summary)
	if err != nil {
		if errors.Is(err, service.ErrSalaryAmountOverflow) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Println("[SalaryAccrual penalties]", err)
			http.Error(w, "Не удалось получить штрафы", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(penaltySummary)
	// --------- конец нового кода chat
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
