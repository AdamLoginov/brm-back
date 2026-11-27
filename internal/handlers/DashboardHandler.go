// новый код chat: защищённая сводка главной страницы
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/service"
	"strconv"
	"time"
)

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	user := notificationUser(w, r)
	if user == 0 {
		return
	}
	zone := time.FixedZone("Москва", 3*60*60)
	now := time.Now().In(zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	month, err := time.ParseInLocation("2006-01", r.URL.Query().Get("month"), zone)
	if err != nil {
		http.Error(w, "Выберите месяц", 400)
		return
	}
	var agreement uint64
	if raw := r.URL.Query().Get("agreement"); raw != "" {
		agreement, err = strconv.ParseUint(raw, 10, 32)
		if err != nil {
			http.Error(w, "Некорректный договор", 400)
			return
		}
	}
	result, err := service.ReadDashboard(database.DB.WithContext(r.Context()), month, today, uint(agreement), user)
	if err != nil {
		log.Printf("dashboard: %v", err)
		http.Error(w, "Не удалось загрузить сводку", 500)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
