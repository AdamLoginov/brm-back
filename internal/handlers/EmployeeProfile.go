// --------- новый код chat: сохранение телефонов и связей со справочником
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strconv"
	"strings"
	"sync"
)

var employeeProfileMutex sync.Mutex

func saveEmployeeProfile(employee *models.EmployeeCard, id string) error {
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	// --------- новый код chat: согласованное изменение ФИО и путей документов
	service.FileStorageMutex.Lock()
	defer service.FileStorageMutex.Unlock()
	batch := &service.FileStorageBatch{}
	defer batch.Close()
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// --------- новый код chat: уволенный сотрудник не закреплён за договором
		if employee.Status {
			employee.AgreementID = 0
			employee.Agreement = models.Agreement{}
		}
		// --------- конец нового кода chat
		// Старые клиенты передают только Phone; новые — весь список Phones.
		if employee.Phones == nil && employee.Phone != nil {
			employee.Phones = []string{*employee.Phone}
		}
		phones := []string{}
		for _, phone := range employee.Phones {
			if value := strings.TrimSpace(phone); value != "" {
				phones = append(phones, value)
			}
		}
		employee.Phones = phones
		employee.Phone = nil
		if len(phones) > 0 {
			employee.Phone = &phones[0]
		}
		selected := []models.EmployeeSpecialization{}
		if employee.SpecializationIDs != nil {
			seen := map[uint]bool{}
			for _, key := range employee.SpecializationIDs {
				if seen[key] {
					continue
				}
				seen[key] = true
				var item models.EmployeeSpecialization
				if err := tx.First(&item, key).Error; err != nil {
					return errors.New("Специализация больше не существует. Обновите список.")
				}
				selected = append(selected, item)
			}
		}
		employee.Specializations = nil
		if id == "" {
			if err := tx.Omit("Specializations").Create(employee).Error; err != nil {
				return err
			}
		} else {
			key, err := strconv.ParseUint(id, 10, 64)
			if err != nil || key == 0 {
				return errors.New("Некорректный сотрудник")
			}
			var existing models.EmployeeCard
			if err := tx.First(&existing, key).Error; err != nil {
				return err
			}
			employee.ID = uint(key)
			if err := tx.Model(&models.EmployeeCard{}).Where("id = ?", key).Select("*").
				Omit("id", "created_at", "updated_at", "agreement", "employee_documents", "Specializations", "TimeSheetMonths").
				Updates(employee).Error; err != nil {
				return err
			}
		}
		if employee.SpecializationIDs != nil {
			if err := tx.Model(employee).Association("Specializations").Replace(selected); err != nil {
				return err
			}
		}
		return service.SyncEmployeeStorage(tx, batch, *employee)
	})
	if err == nil {
		batch.Commit()
	}
	return err
	// --------- конец нового кода chat
}

func ListEmployeeSpecializations(w http.ResponseWriter, r *http.Request) {
	items := []models.EmployeeSpecialization{}
	if err := database.DB.Order("name").Find(&items).Error; err != nil {
		http.Error(w, "Не удалось загрузить специализации", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func SaveEmployeeSpecialization(w http.ResponseWriter, r *http.Request) {
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	var input struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		http.Error(w, "Некорректные данные", 400)
		return
	}
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		http.Error(w, "Название должно содержать от 1 до 100 символов", 400)
		return
	}
	var item models.EmployeeSpecialization
	if id := r.PathValue("id"); id != "" {
		key, err := strconv.ParseUint(id, 10, 64)
		if err != nil || key == 0 {
			http.Error(w, "Некорректный идентификатор", 400)
			return
		}
		if database.DB.First(&item, key).Error != nil {
			http.Error(w, "Специализация не найдена", 404)
			return
		}
	}
	var all []models.EmployeeSpecialization
	if database.DB.Find(&all).Error != nil {
		http.Error(w, "Не удалось проверить справочник", 500)
		return
	}
	for _, other := range all {
		if other.ID != item.ID && strings.EqualFold(other.Name, input.Name) {
			http.Error(w, "Такая специализация уже существует", 409)
			return
		}
	}
	item.Name = input.Name
	if database.DB.Save(&item).Error != nil {
		http.Error(w, "Не удалось сохранить специализацию", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func DeleteEmployeeSpecialization(w http.ResponseWriter, r *http.Request) {
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	key, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || key == 0 {
		http.Error(w, "Некорректный идентификатор", 400)
		return
	}
	var count int64
	if database.DB.Table("employee_card_specializations").Where("employee_specialization_id = ?", key).Count(&count).Error != nil {
		http.Error(w, "Не удалось проверить использование", 500)
		return
	}
	if count > 0 {
		http.Error(w, "Специализация назначена сотрудникам. Сначала снимите её в карточках.", 409)
		return
	}
	result := database.DB.Delete(&models.EmployeeSpecialization{}, key)
	if result.Error != nil {
		http.Error(w, "Не удалось удалить специализацию", 500)
		return
	}
	if result.RowsAffected == 0 {
		http.Error(w, "Специализация не найдена", 404)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --------- конец нового кода chat
