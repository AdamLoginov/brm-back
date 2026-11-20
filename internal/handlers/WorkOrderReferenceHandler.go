// --------- новый код chat: справочники книги нарядов и общие вспомогательные функции API
package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"io"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strconv"
	"strings"
)

func workJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func workError(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, "Запись не найдена в договоре", 404)
		return
	}
	message := err.Error()
	code := 400
	if strings.Contains(message, "UNIQUE constraint") {
		message = "Такая запись уже существует"
		code = 409
	}
	if strings.Contains(message, "locked") {
		message = "Данные сейчас сохраняются другим пользователем. Повторите действие"
		code = 409
	}
	if strings.Contains(message, "Запись изменена") {
		code = 409
	}
	http.Error(w, message, code)
}
func workDecode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*1024*1024))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные", 400)
		return false
	}
	return true
}
func workID(r *http.Request, key string) uint {
	n, _ := strconv.ParseUint(r.PathValue(key), 10, 32)
	return uint(n)
}
func workAgreement(w http.ResponseWriter, r *http.Request) uint {
	id := workID(r, "id")
	if id == 0 {
		http.Error(w, "Некорректный договор", 400)
		return 0
	}
	var a models.Agreement
	if err := database.DB.First(&a, id).Error; err != nil {
		workError(w, err)
		return 0
	}
	return id
}

type workReferenceInput struct {
	Name       string `json:"name"`
	Number     string `json:"number"`
	WorkshopID uint   `json:"workshop_id"`
	BlockID    uint   `json:"block_id"`
	UnitID     uint   `json:"unit_id"`
}

func WorkOrderReferenceHandler(w http.ResponseWriter, r *http.Request) {
	agreementID := workAgreement(w, r)
	if agreementID == 0 {
		return
	}
	if r.Method == "GET" {
		units := []models.MeasurementUnit{}
		shops := []models.AgreementWorkshop{}
		blocks := []models.AgreementWorkBlock{}
		positions := []models.AgreementWorkPosition{}
		employees := []models.EmployeeCard{}
		for _, q := range []*gorm.DB{database.DB.Order("name").Find(&units), database.DB.Where("agreement_id = ?", agreementID).Order("name").Find(&shops), database.DB.Where("agreement_id = ?", agreementID).Order("name").Find(&blocks), database.DB.Where("agreement_id = ?", agreementID).Order("number").Find(&positions), database.DB.Select("id", "name", "surname", "middle_name").Where("agreement_id = ?", agreementID).Find(&employees)} {
			if q.Error != nil {
				workError(w, q.Error)
				return
			}
		}
		workJSON(w, map[string]interface{}{"units": units, "workshops": shops, "blocks": blocks, "positions": positions, "employees": employees})
		return
	}
	kind := r.PathValue("kind")
	id := workID(r, "referenceId")
	remove := r.Method == "DELETE"
	if (r.Method == "PATCH" || remove) && id == 0 {
		http.Error(w, "Не указана запись", 400)
		return
	}
	var input workReferenceInput
	if !remove && !workDecode(w, r, &input) {
		return
	}
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	input.Number = strings.TrimSpace(input.Number)
	if !remove && (input.Name == "" || len(input.Name) > 500) {
		http.Error(w, "Укажите название до 500 байт", 400)
		return
	}
	err := service.WorkTransaction(database.DB, agreementID, func(tx *gorm.DB) error {
		used := func(model interface{}, query string, args ...interface{}) error {
			var count int64
			if err := tx.Model(model).Where(query, args...).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("Запись используется: удаление или перенос запрещены")
			}
			return nil
		}
		switch kind {
		case "units":
			row := models.MeasurementUnit{}
			if id != 0 {
				if err := tx.First(&row, id).Error; err != nil {
					return err
				}
			}
			if remove {
				if err := used(&models.WorkOrderTask{}, "unit_id = ?", id); err != nil {
					return err
				}
				if err := used(&models.WarehouseMaterial{}, "unit_id = ?", id); err != nil {
					return err
				}
				return tx.Delete(&row).Error
			}
			row.Name = input.Name
			row.NameKey = strings.ToLower(input.Name)
			return tx.Save(&row).Error
		case "workshops":
			row := models.AgreementWorkshop{AgreementID: agreementID}
			if id != 0 {
				if err := tx.Where("id = ? AND agreement_id = ?", id, agreementID).First(&row).Error; err != nil {
					return err
				}
			}
			if remove {
				for _, m := range []interface{}{&models.AgreementWorkBlock{}, &models.WorkOrderShift{}, &models.WarehouseMaterial{}} {
					if err := used(m, "workshop_id = ?", id); err != nil {
						return err
					}
				}
				return tx.Delete(&row).Error
			}
			row.Name = input.Name
			row.NameKey = strings.ToLower(input.Name)
			return tx.Save(&row).Error
		case "blocks":
			row := models.AgreementWorkBlock{AgreementID: agreementID}
			if id != 0 {
				if err := tx.Where("id = ? AND agreement_id = ?", id, agreementID).First(&row).Error; err != nil {
					return err
				}
			}
			if remove {
				if err := used(&models.WorkOrderTask{}, "block_id = ?", id); err != nil {
					return err
				} // новый код chat: блок может использоваться напрямую
				if err := used(&models.AgreementWorkPosition{}, "block_id = ?", id); err != nil {
					return err
				}
				return tx.Delete(&row).Error
			}
			var shop models.AgreementWorkshop
			if input.WorkshopID == 0 {
				return errors.New("Выберите цех")
			}
			if err := tx.Where("id = ? AND agreement_id = ?", input.WorkshopID, agreementID).First(&shop).Error; err != nil {
				return err
			}
			if id != 0 && row.WorkshopID != input.WorkshopID {
				if err := used(&models.WorkOrderTask{}, "block_id = ?", id); err != nil {
					return err
				} // новый код chat
				if err := used(&models.AgreementWorkPosition{}, "block_id = ?", id); err != nil {
					return err
				}
			}
			row.WorkshopID = input.WorkshopID
			row.Name = input.Name
			row.NameKey = strings.ToLower(input.Name)
			return tx.Save(&row).Error
		case "positions":
			row := models.AgreementWorkPosition{AgreementID: agreementID}
			if id != 0 {
				if err := tx.Where("id = ? AND agreement_id = ?", id, agreementID).First(&row).Error; err != nil {
					return err
				}
			}
			if remove {
				if err := used(&models.WorkOrderTask{}, "position_id = ?", id); err != nil {
					return err
				}
				return tx.Delete(&row).Error
			}
			if input.Number == "" || len(input.Number) > 100 {
				return errors.New("Укажите номер позиции до 100 байт")
			}
			var block models.AgreementWorkBlock
			if input.BlockID == 0 {
				return errors.New("Выберите блок")
			}
			if err := tx.Where("id = ? AND agreement_id = ?", input.BlockID, agreementID).First(&block).Error; err != nil {
				return err
			}
			if id != 0 && row.BlockID != input.BlockID {
				if err := used(&models.WorkOrderTask{}, "position_id = ?", id); err != nil {
					return err
				}
			}
			row.BlockID = input.BlockID
			row.Name = input.Name
			row.Number = input.Number
			return tx.Save(&row).Error
		default:
			return errors.New("Неизвестный справочник")
		}
	})
	if err != nil {
		workError(w, err)
		return
	}
	workJSON(w, map[string]bool{"ok": true})
}

// --------- конец нового кода chat
