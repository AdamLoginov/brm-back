package handlers

import (
	"encoding/json"
	"fmt"          // новый код chat: проверка месяца увольнения
	"gorm.io/gorm" // новый код chat
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service" // новый код chat
	"time"                     // новый код chat
)

// --------- новый код chat: месяц увольнения включительно доступен для табеля
func validateTimeSheetEmployeeDates(employees []models.EmployeeCard, year, month uint) error {
	if year < 1 || year > 9999 || month < 1 || month > 12 {
		return fmt.Errorf("Укажите корректный месяц и год табеля")
	}
	first := fmt.Sprintf("%04d-%02d-01", year, month)
	for _, employee := range employees {
		if !employee.Status {
			continue
		}
		if _, err := time.Parse("2006-01-02", employee.DismissalDate); err != nil {
			return fmt.Errorf("У сотрудника %s не указана корректная дата увольнения", employee.Name)
		}
		if employee.DismissalDate < first {
			return fmt.Errorf("Сотрудник %s уволен %s: нельзя добавить в табель после месяца увольнения", employee.Name, employee.DismissalDate)
		}
	}
	return nil
}

// --------- конец нового кода chat

func GetAllTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var timeSheetMonth []models.TimeSheetMonth

	if err := database.DB.Where("agreement_id = ?", agreementId).Find(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка получения данных табелей"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func GetDetailTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	var timeSheetMonthId = r.PathValue("id")
	var timeSheetMonth models.TimeSheetMonth

	if err := database.DB.Preload("Employees").Preload("TimeSheets").Where("id = ?", timeSheetMonthId).Find(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка получения данных табелей"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func CreateTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		// --------- новый код chat: настройки смены и штрафа (сумма в рублях строкой)
		ShiftHours     *string `json:"shift_hours"`
		AbsencePenalty *string `json:"absence_penalty"`
		DailyAllowance *string `json:"daily_allowance"` // новый код chat: суточные в рублях
		// --------- конец нового кода chat
		Month        uint   `json:"month"`
		Year         uint   `json:"year"`
		AgreementId  uint   `json:"agreement_id"`
		EmployeesIds []uint `json:"employee_card_id"`
	}

	var employees []models.EmployeeCard

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Where("id IN ?", input.EmployeesIds).Find(&employees).Error; err != nil {
		msg := "[Error] Ошибка при поиске сотрудников по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return // новый код chat: прервать сохранение после ошибки
	}

	// --------- новый код chat: запрет добавления после месяца увольнения
	if err := validateTimeSheetEmployeeDates(employees, input.Year, input.Month); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	// --------- новый код chat: проверка настроек нового табеля
	shiftHours, penalty, err := service.ParseTimeSheetSettings(input.ShiftHours, input.AbsencePenalty, "12", 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	// --------- новый код chat: проверка суточных нового табеля
	dailyAllowance, err := service.ParseTimeSheetDailyAllowance(input.DailyAllowance, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	timeSheetMonth := models.TimeSheetMonth{
		DailyAllowanceKopecks: dailyAllowance,                             // новый код chat
		ShiftHours:            shiftHours, AbsencePenaltyKopecks: penalty, // новый код chat
		Month:       input.Month,
		Year:        input.Year,
		AgreementID: input.AgreementId,
		Employees:   employees,
	}

	if err := database.DB.Create(&timeSheetMonth).Error; err != nil {
		msg := "[Error] Ошибка при записи данных в базу"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeSheetMonth)
}

func UpdateTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {

	var input struct {
		// --------- новый код chat: настройки смены и штрафа (сумма в рублях строкой)
		ShiftHours     *string `json:"shift_hours"`
		AbsencePenalty *string `json:"absence_penalty"`
		DailyAllowance *string `json:"daily_allowance"` // новый код chat: суточные в рублях
		// --------- конец нового кода chat
		Month            uint   `json:"month"`
		Year             uint   `json:"year"`
		EmployeesIds     []uint `json:"employee_card_id"`
		TimeSheetMonthId uint   `json:"timesheet_month_id"`
	}
	var timeSheetMonth models.TimeSheetMonth
	var employees []models.EmployeeCard

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.First(&timeSheetMonth, input.TimeSheetMonthId).Error; err != nil {
		msg := "[Error] Ошибка при поиске табеля по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return // новый код chat: прервать сохранение после ошибки
	}

	if err := database.DB.Where("id IN ?", input.EmployeesIds).Find(&employees).Error; err != nil {
		msg := "[Error] Ошибка при поиске сотрудников по ID"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return // новый код chat: прервать сохранение после ошибки
	}

	// --------- новый код chat: дата увольнения проверяется и при изменении табеля
	if err := validateTimeSheetEmployeeDates(employees, input.Year, input.Month); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	// --------- новый код chat: настройки, состав сотрудников и штрафы сохраняются вместе
	shiftHours, penalty, err := service.ParseTimeSheetSettings(input.ShiftHours, input.AbsencePenalty, timeSheetMonth.ShiftHours, timeSheetMonth.AbsencePenaltyKopecks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- новый код chat: сохраняем прежние суточные, если поле не передано
	dailyAllowance, err := service.ParseTimeSheetDailyAllowance(input.DailyAllowance, timeSheetMonth.DailyAllowanceKopecks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&timeSheetMonth).Updates(map[string]interface{}{"month": input.Month, "year": input.Year, "shift_hours": shiftHours, "absence_penalty_kopecks": penalty, "daily_allowance_kopecks": dailyAllowance}).Error; err != nil {
			return err
		}
		if err := tx.Model(&timeSheetMonth).Association("Employees").Replace(&employees); err != nil {
			return err
		}
		// --------- новый код chat: обновление суточных при изменении ставки или сотрудников
		if err := service.SyncTimeSheetPenalties(tx, timeSheetMonth.ID); err != nil {
			return err
		}
		return service.SyncDailyAllowanceAccruals(tx, timeSheetMonth.ID)
		// --------- конец нового кода chat
	})
	if err != nil {
		log.Println(err)
		http.Error(w, "Не удалось обновить табель и штрафы", http.StatusInternalServerError)
		return
	}
	// --------- конец нового кода chat

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Данные успешно обновлены!"})
}

func DeleteTimeSheetMonthHandler(w http.ResponseWriter, r *http.Request) {
	timeSheetMonthID := r.PathValue("id")

	var timeSheetMonth models.TimeSheetMonth

	// Находим табель
	if err := database.DB.First(&timeSheetMonth, timeSheetMonthID).Error; err != nil {

		msg := "[Error] Табель не найден"
		log.Println(msg)
		http.Error(w, msg, http.StatusNotFound)
		return
	}

	// Soft delete TimeSheets + самого TimeSheetMonth
	// --------- новый код chat: удаление табеля и всех связанных автоматических штрафов
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := service.DeleteTimeSheetPenalties(tx, timeSheetMonth.ID); err != nil {
			return err
		}
		if err := service.DeleteDailyAllowanceAccruals(tx, timeSheetMonth.ID); err != nil {
			return err
		} // новый код chat: выдачи сохраняются
		return tx.Select("TimeSheets", "SalaryAccruals").Delete(&timeSheetMonth).Error
	}); err != nil { // --------- конец нового кода chat
		msg := "[Error] Ошибка при удалении данных"
		log.Println(msg, err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Табель успешно удален"})
}
