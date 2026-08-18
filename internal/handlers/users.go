package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetUserHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")

	var user models.User
	if err := database.DB.Preload("EmployeeCard").First(&user, idStr).Error; err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func GetAllUserHandler(w http.ResponseWriter, r *http.Request) {
	var users []models.User

	if err := database.DB.Preload("EmployeeCard").Find(&users).Error; err != nil {
		http.Error(w, "Ошибка получения списка", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var user models.User

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		log.Println("[ERROR] Ошибка при получении данных в процессе создания пользователя")
		http.Error(w, "Ошибка при получении данных", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&user).Error; err != nil {
		log.Println("[ERROR] Ошибка при создании пользователя")
		http.Error(w, "Ошибка при создании пользователя", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Пользователь успешно создан"})
}

func DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	var userId = r.PathValue("id")

	if err := database.DB.Delete(&models.User{}, userId).Error; err != nil {
		log.Println("[ERROR] Ошибка при удалении данных")
		http.Error(w, "Ошибк при удалении данных", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Пользователь успешно удалены"})
}
