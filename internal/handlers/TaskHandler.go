package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service" // новый код chat
	"strconv"                  // новый код chat
)

func GetAllTaskHandler(w http.ResponseWriter, r *http.Request) {
	var tasks []models.Task

	if err := database.DB.Preload("Attachments").Preload("Assignees.EmployeeCard").Preload("CompletedBy.EmployeeCard").Preload("ToUser.EmployeeCard").Preload("FromUser.EmployeeCard").Find(&tasks).Error; err != nil {
		msg := "[Error] Ошибка получения данных задач"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// --------- новый код chat: автор из JWT, события задачи сохраняются атомарно
func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	actor := notificationUser(w, r)
	if actor == 0 {
		return
	}
	saveTaskRequest(w, r, actor, false) // новый код chat
}
func UpdateTaskHandler(w http.ResponseWriter, r *http.Request) {
	actor := notificationUser(w, r)
	if actor == 0 {
		return
	}
	saveTaskRequest(w, r, actor, true) // новый код chat
}
func taskRequestID(w http.ResponseWriter, r *http.Request) uint {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "Некорректная задача", 400)
		return 0
	}
	return uint(id)
}
func UpdateStatusTaskHandler(w http.ResponseWriter, r *http.Request) {
	actor := notificationUser(w, r)
	if actor == 0 {
		return
	}
	id := taskRequestID(w, r)
	if id == 0 {
		return
	}
	input := models.Task{}
	input.ID = id
	result, err := service.UpdateTaskWithNotifications(database.DB, actor, input, true)
	notificationResponse(w, result, err)
}
func DeleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	actor := notificationUser(w, r)
	if actor == 0 {
		return
	}
	id := taskRequestID(w, r)
	if id == 0 {
		return
	}
	err := service.DeleteTaskWithNotifications(database.DB, actor, id)
	notificationResponse(w, models.Response{Message: "Задача и связанные уведомления удалены"}, err)
}

// --------- конец нового кода chat
