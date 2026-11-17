// --------- новый код chat: уведомления доступны только пользователю из JWT
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/service"
	"strconv"
)

func notificationUser(w http.ResponseWriter, r *http.Request) uint {
	id := middleware.CurrentUserID(r)
	if id == 0 {
		http.Error(w, "Требуется вход в систему", 401)
	}
	return id
}
func notificationResponse(w http.ResponseWriter, result any, err error) {
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			http.Error(w, "Запись не найдена", 404)
		case errors.Is(err, service.ErrTaskUser):
			http.Error(w, err.Error(), 400)
		default:
			log.Println("[Notifications]", err)
			http.Error(w, "Не удалось сохранить или получить данные", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(result)
}
func GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	id := notificationUser(w, r)
	if id == 0 {
		return
	}
	page := 1
	if text := r.URL.Query().Get("page"); text != "" {
		v, err := strconv.Atoi(text)
		if err != nil || v < 1 {
			http.Error(w, "Некорректная страница", 400)
			return
		}
		page = v
	}
	result, err := service.ListNotifications(database.DB, id, page)
	notificationResponse(w, result, err)
}
func ReadNotificationHandler(w http.ResponseWriter, r *http.Request) {
	userID := notificationUser(w, r)
	if userID == 0 {
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "Некорректное уведомление", 400)
		return
	}
	err = service.ReadNotifications(database.DB, userID, uint(id), false)
	notificationResponse(w, map[string]bool{"read": err == nil}, err)
}
func ReadAllNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	id := notificationUser(w, r)
	if id == 0 {
		return
	}
	err := service.ReadNotifications(database.DB, id, 0, true)
	notificationResponse(w, map[string]bool{"read": err == nil}, err)
}

// --------- конец нового кода chat
