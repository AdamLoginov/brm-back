// --------- новый код chat: надкатегории документов и назначение группы категории
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"sync" // новый код chat: последовательные изменения порядка
)

// --------- новый код chat: перестановка без нарушения уникального индекса
var employeeDocumentGroupOrderMutex sync.Mutex

func saveEmployeeDocumentGroupOrder(tx *gorm.DB, rows []models.EmployeeDocumentGroup) error {
	var stored []models.EmployeeDocumentGroup
	if err := tx.Find(&stored).Error; err != nil {
		return err
	}
	occupied := map[int]bool{}
	for _, row := range stored {
		occupied[row.Priority] = true
	}
	// Временные свободные положительные значения вне итогового диапазона 1..N.
	candidate := len(rows) + 1
	for i := range rows {
		for occupied[candidate] {
			candidate++
		}
		occupied[candidate] = true
		rows[i].Priority = candidate
		if rows[i].ID == 0 {
			if err := tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&rows[i]).Update("priority", candidate).Error; err != nil {
			return err
		}
		candidate++
	}
	for i := range rows {
		rows[i].Priority = i + 1
		if err := tx.Model(&rows[i]).Updates(map[string]interface{}{"name": rows[i].Name, "priority": i + 1}).Error; err != nil {
			return err
		}
	}
	return nil
}

// --------- конец нового кода chat

func EmployeeDocumentGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows := []models.EmployeeDocumentGroup{}
		if err := database.DB.Order("priority, name").Find(&rows).Error; err != nil {
			http.Error(w, "Не удалось загрузить надкатегории", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rows)
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if r.Method != http.MethodPost && (err != nil || id == 0) {
		http.Error(w, "Некорректная надкатегория", 400)
		return
	}
	// новый код chat: одновременно можно менять только один порядок списка
	employeeDocumentGroupOrderMutex.Lock()
	defer employeeDocumentGroupOrderMutex.Unlock()
	if r.Method == http.MethodDelete {
		err = database.DB.Transaction(func(tx *gorm.DB) error {
			var group models.EmployeeDocumentGroup
			if err := tx.First(&group, id).Error; err != nil {
				return err
			}
			// Снимаем связь также с ранее удалённых категорий.
			if err := tx.Unscoped().Model(&models.CategoryEmployeeDocument{}).Where("group_id = ?", id).Update("group_id", nil).Error; err != nil {
				return err
			}
			if err := tx.Delete(&group).Error; err != nil {
				return err
			}
			var rows []models.EmployeeDocumentGroup
			if err := tx.Order("priority, id").Find(&rows).Error; err != nil {
				return err
			}
			return saveEmployeeDocumentGroupOrder(tx, rows) // новый код chat: закрываем разрыв
		})
	} else {
		var input struct {
			Name     string `json:"name"`
			Priority int    `json:"priority"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || strings.TrimSpace(input.Name) == "" || input.Priority <= 0 {
			http.Error(w, "Укажите название и положительный целый приоритет", 400)
			return
		}
		group := models.EmployeeDocumentGroup{Name: strings.TrimSpace(input.Name), Priority: input.Priority}
		err = database.DB.Transaction(func(tx *gorm.DB) error {
			// новый код chat: убираем перемещаемую группу и вставляем на выбранную позицию
			var rows []models.EmployeeDocumentGroup
			if err := tx.Order("priority, id").Find(&rows).Error; err != nil {
				return err
			}
			if r.Method != http.MethodPost {
				found := false
				for i, row := range rows {
					if row.ID == uint(id) {
						group.ID = row.ID
						rows = append(rows[:i], rows[i+1:]...)
						found = true
						break
					}
				}
				if !found {
					return gorm.ErrRecordNotFound
				}
			}
			position := input.Priority - 1
			if position > len(rows) {
				position = len(rows)
			}
			rows = append(rows, models.EmployeeDocumentGroup{})
			copy(rows[position+1:], rows[position:])
			rows[position] = group
			if err := saveEmployeeDocumentGroupOrder(tx, rows); err != nil {
				return err
			}
			group = rows[position]
			return nil
		})
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(group)
			return
		}
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Надкатегория не найдена", 404)
		} else {
			http.Error(w, "Не удалось сохранить надкатегорию", 500)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func UpdateEmployeeDocumentCategoryGroupHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	var input struct {
		GroupID *uint `json:"group_id"`
	}
	if err != nil || id == 0 || json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "Некорректные данные", 400)
		return
	}
	var category models.CategoryEmployeeDocument
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&category, id).Error; err != nil {
			return err
		}
		if input.GroupID != nil {
			var group models.EmployeeDocumentGroup
			if err := tx.First(&group, *input.GroupID).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&category).Update("group_id", input.GroupID).Error; err != nil {
			return err
		}
		return tx.Preload("Group").First(&category, id).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Категория или надкатегория не найдена", 404)
		} else {
			http.Error(w, "Не удалось изменить группу", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}

// --------- конец нового кода chat
