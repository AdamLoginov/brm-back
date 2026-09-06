package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllTaskHandler(w http.ResponseWriter, r *http.Request) {
	var tasks []models.Task

	if err := database.DB.Preload("ToUser.EmployeeCard").Preload("FromUser.EmployeeCard").Find(&tasks).Error; err != nil {
		msg := "[Error] Ошибка получения данных задач"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task models.Task

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		msg := "[Error] Ошибка чтения задачи"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&task).Error; err != nil {
		msg := "[Error] Ошибка записи задачи в базу данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func UpdateTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task models.Task

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		msg := "[Error] Ошибка при чтении данных обновления задачи"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Model(&models.Task{}).Where("id = ?", task.ID).Select("Message", "Status", "Priority", "ToUserID").Updates(task).Error; err != nil {
		msg := "[Error] Ошибка обновления данных задачи в базу"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func UpdateStatusTaskHandler(w http.ResponseWriter, r *http.Request) {
	var taskId = r.PathValue("id")

	if err := database.DB.Model(&models.Task{}).Where("id = ?", taskId).Update("Status", true).Error; err != nil {
		msg := "[Error] Ошибка обновления данных задачи в базу"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Успешно изменено"})
}

func DeleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	var taskId = r.PathValue("id")

	if err := database.DB.Delete(&models.Task{}, taskId).Error; err != nil {
		msg := "[Error] Ошибка при удалении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Данные успешно удалены"})
}
