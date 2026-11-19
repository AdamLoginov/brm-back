// --------- новый код chat: общий справочник категорий расходов
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

func ListExpenseCategories(w http.ResponseWriter, r *http.Request) {
	rows := []models.ExpenseCategory{}
	if err := database.DB.Order("name_key").Find(&rows).Error; err != nil {
		http.Error(w, "Не удалось загрузить категории", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

func SaveExpenseCategory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные категории", 400)
		return
	}
	name := strings.Join(strings.Fields(input.Name), " ")
	if name == "" || utf8.RuneCountInString(name) > 100 {
		http.Error(w, "Введите название категории от 1 до 100 символов", 400)
		return
	}
	var category models.ExpenseCategory
	if r.PathValue("id") != "" {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Некорректная категория", 400)
			return
		}
		if err := database.DB.First(&category, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				http.Error(w, "Категория не найдена", 404)
			} else {
				http.Error(w, "Не удалось загрузить категорию", 500)
			}
			return
		}
	}
	category.Name, category.NameKey = name, strings.ToLower(name)
	var err error
	if category.ID == 0 {
		err = database.DB.Create(&category).Error
	} else {
		result := database.DB.Model(&models.ExpenseCategory{}).Where("id = ?", category.ID).Updates(map[string]interface{}{"name": category.Name, "name_key": category.NameKey})
		err = result.Error
		if err == nil && result.RowsAffected == 0 {
			http.Error(w, "Категория не найдена", 404)
			return
		}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
			http.Error(w, "Категория с таким названием уже существует", 409)
		} else {
			http.Error(w, "Не удалось сохранить категорию", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}

func DeleteExpenseCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || id == 0 {
		http.Error(w, "Некорректная категория", 400)
		return
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var category models.ExpenseCategory
		if err := tx.First(&category, id).Error; err != nil {
			return err
		}
		// Снимаем категорию во всех договорах, в том числе у мягко удалённых расходов.
		if err := tx.Unscoped().Model(&models.Expenses{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&category).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Категория не найдена", 404)
		} else {
			http.Error(w, "Не удалось удалить категорию", 500)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --------- конец нового кода chat
