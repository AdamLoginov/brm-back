package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
)

func GetAllSupler(w http.ResponseWriter, r *http.Request) {
	var suplers []models.Suplers

	if err := database.DB.Preload("ManagerCards").Preload("CategorySuppliers").Find(&suplers).Error; err != nil {
		http.Error(w, "Не удалось создать поставщика", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suplers)
}

func GetDetailSupplier(w http.ResponseWriter, r *http.Request) {
	var idSupler = r.PathValue("id")
	var supler models.Suplers

	if err := database.DB.Preload("ManagerCards").First(&supler, idSupler).Error; err != nil {
		http.Error(w, "Не удалось найти поставщика!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(supler)
}

func CreateSupler(w http.ResponseWriter, r *http.Request) {
	var category []models.CategorySuppliers

	var input struct {
		Name       string `json:"name"`
		Link       string `json:"link"`
		CategoryId []uint `json:"category_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Println("[ERROR] Ошибка в получении данных при создании компании")
		http.Error(w, "[ERROR] Ошибка в получении данных при создании компании", http.StatusBadRequest)
		return
	}

	if err := database.DB.Where("id IN ?", input.CategoryId).Find(&category).Error; err != nil {
		log.Println("[ERROR] Ошибка при поиске категорий")
		http.Error(w, "[ERROR] Ошибка при поиске категорий", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&models.Suplers{
		Name:              input.Name,
		Link:              input.Link,
		CategorySuppliers: category,
	}).Error; err != nil {
		log.Println("[ERROR] Ошибка при сохранении данных компании")
		http.Error(w, "[ERROR] Ошибка при сохранении данных компании", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Поставщик успешно создан!"})
}

// func UpdateSupler(w http.ResponseWriter, r *http.Request) {
// 	var idSupler = r.PathValue("id")
// 	var supler models.Suplers
// 	var input struct {
// 		Name           string `json:"name"`
// 		Specialization string `json:"specialization"`
// 		Email          string `json:"email"`
// 	}

// 	if err := database.DB.First(&supler, idSupler).Error; err != nil {
// 		http.Error(w, "Не удалось найти поставщика", http.StatusInternalServerError)
// 		return
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// 		http.Error(w, "Не верно введенные данные", http.StatusInternalServerError)
// 		return
// 	}

// 	if err := database.DB.Model(&supler).Updates(models.Suplers{
// 		Name:           input.Name,
// 		Specialization: input.Specialization,
// 		Email:          input.Email,
// 	}).Error; err != nil {
// 		http.Error(w, "Ошибка при внессении изменений в базу данных", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(models.Response{Message: "Сотрудник успешно удален!"})
// }

func DeleteSupplierHandler(w http.ResponseWriter, r *http.Request) {
	var idSupler = r.PathValue("id")
	if err := database.DB.Delete(&models.Suplers{}, idSupler).Error; err != nil {
		http.Error(w, "Не удалось удалить сотрудника", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Сотрудник успешно удален!"})
}

// ------------------------------------------------------------------------------------------

func GetAllCategorySupplierHandler(w http.ResponseWriter, r *http.Request) {
	var categoryes []models.CategorySuppliers

	if err := database.DB.Find(&categoryes).Error; err != nil {
		log.Println("[ERROR] Ошибка в получении категорий")
		http.Error(w, "Ошибка в получении категорий", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categoryes)
}

func CreateCategorySupplierHandler(w http.ResponseWriter, r *http.Request) {
	var category models.CategorySuppliers

	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		log.Println("[ERROR Ошибка в полученных данных категории]")
		http.Error(w, "Ошибка в получении данных категории", http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&category).Error; err != nil {
		log.Println("[ERROR Ошибка при сохранени данных категории]")
		http.Error(w, "Ошибка при сохранени данных категории", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Успешное создание категории"})
}

func DeleteCategorySupplierHandler(w http.ResponseWriter, r *http.Request) {
	var categoryId = r.PathValue("id")

	if err := database.DB.Delete(&models.CategorySuppliers{}, categoryId).Error; err != nil {
		log.Println("[ERROR] Ошибка при удалении категории")
		http.Error(w, "Ошибка при удалении категории", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Категория успешно удалена"})
}
