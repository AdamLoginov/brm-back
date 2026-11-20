// --------- новый код chat: атомарное сохранение нарядов, часов и расхода материалов
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
	"time"
)

// Первая запись в транзакции берёт блокировку SQLite до проверки остатков.
// Конкурирующий запрос либо видит новое состояние, либо получает ошибку без частичного сохранения.
func WorkTransaction(db *gorm.DB, agreementID uint, fn func(*gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Agreement{}).Where("id = ?", agreementID).UpdateColumn("id", agreementID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return fn(tx)
	})
}

type WorkEmployeeInput struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Hours          string `json:"hours"`
}
type WorkMaterialInput struct {
	MaterialID uint   `json:"material_id"`
	Quantity   string `json:"quantity"`
}
type WorkTaskInput struct {
	PositionID uint                `json:"position_id"`
	Name       string              `json:"name"`
	UnitID     uint                `json:"unit_id"`
	Quantity   string              `json:"quantity"`
	Hours      string              `json:"hours"`
	Materials  []WorkMaterialInput `json:"materials"`
}
type WorkShiftInput struct {
	DayID      uint                `json:"day_id"`
	WorkshopID uint                `json:"workshop_id"`
	Number     int                 `json:"number"`
	Version    int64               `json:"version"`
	Employees  []WorkEmployeeInput `json:"employees"`
	Tasks      []WorkTaskInput     `json:"tasks"`
}
type WorkTaskDetail struct {
	models.WorkOrderTask
	Materials []models.WorkOrderMaterial `json:"materials"`
}
type WorkShiftDetail struct {
	models.WorkOrderShift
	Employees []models.WorkOrderEmployee `json:"employees"`
	Tasks     []WorkTaskDetail           `json:"tasks"`
}
type WorkBookDetail struct {
	models.WorkOrderBook
	Days   []models.WorkOrderDay `json:"days"`
	Shifts []WorkShiftDetail     `json:"shifts"`
}

func LoadWorkBook(db *gorm.DB, agreementID, bookID uint) (WorkBookDetail, error) {
	var result WorkBookDetail
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND agreement_id = ?", bookID, agreementID).First(&result.WorkOrderBook).Error; err != nil {
			return err
		}
		result.Days = []models.WorkOrderDay{}
		result.Shifts = []WorkShiftDetail{}
		if err := tx.Where("book_id = ?", bookID).Order("date").Find(&result.Days).Error; err != nil {
			return err
		}
		var shifts []models.WorkOrderShift
		if err := tx.Where("day_id IN (?)", tx.Model(&models.WorkOrderDay{}).Select("id").Where("book_id = ?", bookID)).Order("day_id, workshop_id, number").Find(&shifts).Error; err != nil {
			return err
		}
		for _, shift := range shifts {
			detail := WorkShiftDetail{WorkOrderShift: shift, Employees: []models.WorkOrderEmployee{}, Tasks: []WorkTaskDetail{}}
			if err := tx.Where("shift_id = ?", shift.ID).Order("id").Find(&detail.Employees).Error; err != nil {
				return err
			}
			var tasks []models.WorkOrderTask
			if err := tx.Where("shift_id = ?", shift.ID).Order("id").Find(&tasks).Error; err != nil {
				return err
			}
			for _, task := range tasks {
				row := WorkTaskDetail{WorkOrderTask: task, Materials: []models.WorkOrderMaterial{}}
				if err := tx.Where("task_id = ?", task.ID).Order("id").Find(&row.Materials).Error; err != nil {
					return err
				}
				detail.Tasks = append(detail.Tasks, row)
			}
			result.Shifts = append(result.Shifts, detail)
		}
		return nil
	})
	return result, err
}

func CreateWorkBook(db *gorm.DB, agreementID uint, year, month int) (models.WorkOrderBook, error) {
	book := models.WorkOrderBook{AgreementID: agreementID, Year: year, Month: month}
	if year < 1900 || year > 2200 || month < 1 || month > 12 {
		return book, errors.New("Некорректный месяц книги")
	}
	err := WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		if err := tx.Create(&book).Error; err != nil {
			return err
		}
		days := []models.WorkOrderDay{}
		for date := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC); int(date.Month()) == month; date = date.AddDate(0, 0, 1) {
			days = append(days, models.WorkOrderDay{BookID: book.ID, Date: date.Format("2006-01-02")})
		}
		return tx.Create(&days).Error
	})
	return book, err
}

func clearWorkShift(tx *gorm.DB, id uint) error {
	if err := tx.Where("task_id IN (?)", tx.Model(&models.WorkOrderTask{}).Select("id").Where("shift_id = ?", id)).Delete(&models.WorkOrderMaterial{}).Error; err != nil {
		return err
	}
	if err := tx.Where("shift_id = ?", id).Delete(&models.WorkOrderTask{}).Error; err != nil {
		return err
	}
	return tx.Where("shift_id = ?", id).Delete(&models.WorkOrderEmployee{}).Error
}

func SaveWorkShift(db *gorm.DB, agreementID, shiftID uint, input WorkShiftInput) (models.WorkOrderShift, string, error) {
	var shift models.WorkOrderShift
	warning := ""
	err := WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		if input.Number != 1 && input.Number != 2 {
			return errors.New("Укажите смену 1 или 2")
		}
		if len(input.Employees) > 1000 || len(input.Tasks) > 1000 {
			return errors.New("Слишком большая запись смены")
		}
		var day models.WorkOrderDay
		if err := tx.Joins("JOIN work_order_books b ON b.id = work_order_days.book_id").Where("work_order_days.id = ? AND b.agreement_id = ?", input.DayID, agreementID).First(&day).Error; err != nil {
			return err
		}
		var shop models.AgreementWorkshop
		if err := tx.Where("id = ? AND agreement_id = ?", input.WorkshopID, agreementID).First(&shop).Error; err != nil {
			return err
		}
		previous := map[uint]models.WorkOrderEmployee{}
		if shiftID != 0 {
			if err := tx.Where("id = ? AND agreement_id = ?", shiftID, agreementID).First(&shift).Error; err != nil {
				return err
			}
			if shift.Version != input.Version {
				return errors.New("Запись изменена другим действием. Обновите книгу и повторите")
			}
			if shift.DayID != input.DayID || shift.WorkshopID != input.WorkshopID || shift.Number != input.Number {
				return errors.New("Для смены даты, цеха или смены создайте отдельную запись")
			}
			var old []models.WorkOrderEmployee
			if err := tx.Where("shift_id = ?", shiftID).Find(&old).Error; err != nil {
				return err
			}
			for _, row := range old {
				previous[row.EmployeeCardID] = row
			}
		} else {
			shift = models.WorkOrderShift{AgreementID: agreementID, DayID: input.DayID, WorkshopID: input.WorkshopID, Number: input.Number}
		}
		employees := []models.WorkOrderEmployee{}
		used := map[uint]bool{}
		var employeeHours int64
		for _, e := range input.Employees {
			if e.EmployeeCardID == 0 || used[e.EmployeeCardID] {
				return errors.New("Сотрудник указан повторно или не выбран")
			}
			used[e.EmployeeCardID] = true
			hours, err := ParseWorkQuantity(e.Hours, 2, false)
			if err != nil {
				return err
			}
			if hours > 2400 {
				return errors.New("Не более 24 часов сотруднику за день")
			}
			name := ""
			if old, ok := previous[e.EmployeeCardID]; ok {
				name = old.EmployeeName
			} else {
				var employee models.EmployeeCard
				if err := tx.Where("id = ? AND agreement_id = ?", e.EmployeeCardID, agreementID).First(&employee).Error; err != nil {
					return errors.New("Новый участник должен быть закреплён за договором")
				}
				parts := []string{}
				if employee.Surname != nil {
					parts = append(parts, *employee.Surname)
				}
				parts = append(parts, employee.Name)
				if employee.MiddleName != nil {
					parts = append(parts, *employee.MiddleName)
				}
				name = strings.Join(parts, " ")
			}
			var other int64
			if err := tx.Model(&models.WorkOrderEmployee{}).Select("COALESCE(SUM(hours_hundredths),0)").Joins("JOIN work_order_shifts s ON s.id = work_order_employees.shift_id JOIN work_order_days d ON d.id = s.day_id").Where("employee_card_id = ? AND d.date = ? AND s.id <> ?", e.EmployeeCardID, day.Date, shiftID).Scan(&other).Error; err != nil {
				return err
			}
			if hours+other > 2400 {
				return fmt.Errorf("%s: суммарно по всем сменам дня больше 24 часов", name)
			}
			employeeHours += hours
			employees = append(employees, models.WorkOrderEmployee{EmployeeCardID: e.EmployeeCardID, EmployeeName: name, HoursHundredths: hours})
		}
		// Сначала удаляем старые строки внутри транзакции: они возвращают материалы в доступный остаток.
		if shiftID != 0 {
			if err := clearWorkShift(tx, shiftID); err != nil {
				return err
			}
		}
		shift.Version++
		if shiftID == 0 {
			if err := tx.Create(&shift).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&shift).Update("version", shift.Version).Error; err != nil {
			return err
		}
		for i := range employees {
			employees[i].ShiftID = shift.ID
		}
		if len(employees) > 0 {
			if err := tx.Create(&employees).Error; err != nil {
				return err
			}
		}
		var taskHours int64
		for _, inputTask := range input.Tasks {
			name := strings.TrimSpace(inputTask.Name)
			if name == "" || len(name) > 2000 {
				return errors.New("Укажите наименование работы (до 2000 байт)")
			}
			var position models.AgreementWorkPosition
			if err := tx.Joins("JOIN agreement_work_blocks b ON b.id = agreement_work_positions.block_id").Where("agreement_work_positions.id = ? AND agreement_work_positions.agreement_id = ? AND b.workshop_id = ?", inputTask.PositionID, agreementID, input.WorkshopID).First(&position).Error; err != nil {
				return errors.New("Выберите позицию блока текущего цеха")
			}
			var unit models.MeasurementUnit
			if err := tx.First(&unit, inputTask.UnitID).Error; err != nil || inputTask.UnitID == 0 {
				return errors.New("Выберите единицу измерения работы")
			}
			quantity, err := ParseWorkQuantity(inputTask.Quantity, 2, false)
			if err != nil {
				return err
			}
			hours, err := ParseWorkQuantity(inputTask.Hours, 2, true)
			if err != nil {
				return err
			}
			taskHours += hours
			if taskHours > employeeHours {
				return errors.New("Часы работ превышают сумму часов сотрудников")
			}
			task := models.WorkOrderTask{ShiftID: shift.ID, PositionID: inputTask.PositionID, Name: name, UnitID: inputTask.UnitID, QuantityHundredths: quantity, HoursHundredths: hours}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			if len(inputTask.Materials) > 1000 {
				return errors.New("Слишком много материалов")
			}
			seen := map[uint]bool{}
			for _, m := range inputTask.Materials {
				if seen[m.MaterialID] {
					return errors.New("Материал повторяется в работе")
				}
				seen[m.MaterialID] = true
				var material models.WarehouseMaterial
				if err := tx.Where("id = ? AND agreement_id = ? AND workshop_id = ?", m.MaterialID, agreementID, input.WorkshopID).First(&material).Error; err != nil {
					return errors.New("Материал не принадлежит выбранному цеху")
				}
				amount, err := ParseWorkQuantity(m.Quantity, 3, false)
				if err != nil {
					return err
				}
				balance, err := WarehouseBalance(tx, m.MaterialID)
				if err != nil {
					return err
				}
				if amount > balance {
					return fmt.Errorf("Недостаточно материала «%s» на складе цеха", material.Name)
				}
				if err := tx.Create(&models.WorkOrderMaterial{TaskID: task.ID, MaterialID: m.MaterialID, QuantityThousandths: amount}).Error; err != nil {
					return err
				}
			}
		}
		if taskHours < employeeHours {
			warning = "По работам распределены не все часы сотрудников"
		}
		return nil
	})
	return shift, warning, err
}

func DeleteWorkShift(db *gorm.DB, agreementID, shiftID uint, version int64) error {
	return WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		var shift models.WorkOrderShift
		if err := tx.Where("id = ? AND agreement_id = ?", shiftID, agreementID).First(&shift).Error; err != nil {
			return err
		}
		if shift.Version != version {
			return errors.New("Запись изменена. Обновите книгу")
		}
		if err := clearWorkShift(tx, shiftID); err != nil {
			return err
		}
		return tx.Delete(&shift).Error
	})
}

// --------- конец нового кода chat
