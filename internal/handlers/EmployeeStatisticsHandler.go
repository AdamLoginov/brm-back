// --------- новый код chat: статистика сотрудника за выбранный год
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"project/internal/database"
	"project/internal/service"
	"strconv"
)

func EmployeeStatisticsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	year, yearErr := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || id == 0 || yearErr != nil || year < 1 || year > 9999 {
		http.Error(w, "Укажите сотрудника и корректный год", 400)
		return
	}
	result, err := service.GetEmployeeStatistics(database.DB, uint(id), year)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Сотрудник не найден", 404)
		} else {
			http.Error(w, "Не удалось рассчитать статистику сотрудника", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// --------- конец нового кода chat
