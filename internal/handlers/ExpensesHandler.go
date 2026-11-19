package handlers

import (
	"encoding/json"
	"errors"       // новый код chat
	"gorm.io/gorm" // новый код chat
	"io"           // новый код chat
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service" // новый код chat
	"strconv"                  // новый код chat
	"time"                     // новый код chat
)

// --------- новый код chat: проверка даты, суммы, счёта и сотрудника
func validateExpenseInput(expense *models.Expenses) error {
	// --------- новый код chat: только существующая категория или отсутствие категории
	if expense.CategoryID != nil {
		var category models.ExpenseCategory
		if *expense.CategoryID == 0 || database.DB.First(&category, *expense.CategoryID).Error != nil {
			return errors.New("Категория расхода не найдена")
		}
	}
	// --------- конец нового кода chat
	date, err := time.Parse("2006-01-02", expense.ExpenseDate)
	if err != nil || date.Year() < 1 || date.Format("2006-01-02") != expense.ExpenseDate {
		return errors.New("Укажите корректную дату расхода")
	}
	if _, err := service.ExpenseKopecks(expense.Price); err != nil {
		return errors.New("Некорректная сумма расхода")
	}
	if expense.Account != "none" && expense.Account != "company" && expense.Account != "employee" {
		return errors.New("Некорректный счёт расхода")
	}
	if expense.Account == "employee" && expense.EmployeeCardId == 0 {
		return errors.New("Укажите сотрудника для личного расхода")
	}
	if expense.EmployeeCardId != 0 {
		var employee models.EmployeeCard
		if database.DB.First(&employee, expense.EmployeeCardId).Error != nil {
			return errors.New("Сотрудник не найден")
		}
	}
	return nil
}

func UpdateExpensesHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	expenseID, idErr := strconv.ParseUint(r.PathValue("expenseId"), 10, 32)
	if err != nil || idErr != nil || agreementID == 0 || expenseID == 0 {
		http.Error(w, "Некорректный идентификатор", 400)
		return
	}
	// Принимаем только редактируемые поля; статусы и служебные даты не перезаписываются.
	var input struct {
		Name           string  `json:"name"`
		Price          float64 `json:"price"`
		Account        string  `json:"account"`
		EmployeeCardID uint    `json:"employee_card_id"`
		ExpenseDate    string  `json:"expense_date"`
		CategoryID     *uint   `json:"category_id"` // новый код chat
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные расхода", 400)
		return
	}
	values := models.Expenses{Name: input.Name, Price: input.Price, Account: input.Account, EmployeeCardId: input.EmployeeCardID, ExpenseDate: input.ExpenseDate}
	values.CategoryID = input.CategoryID // новый код chat
	if err := validateExpenseInput(&values); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var expense models.Expenses
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND agreement_id = ?", expenseID, agreementID).First(&expense).Error; err != nil {
			return err
		}
		reimbursed := expense.IsReimbursed
		if expense.EmployeeCardId != values.EmployeeCardId || expense.Account != values.Account {
			reimbursed = false
		}
		if err := tx.Model(&expense).Updates(map[string]interface{}{
			"name": values.Name, "price": values.Price, "account": values.Account,
			"employee_card_id": values.EmployeeCardId, "expense_date": values.ExpenseDate,
			"is_reimbursed": reimbursed,
			"category_id":   values.CategoryID, // новый код chat: map сохраняет также NULL
		}).Error; err != nil {
			return err
		}
		return tx.Preload("EmployeeCard").First(&expense, expense.ID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Расход не найден в договоре", 404)
		} else {
			http.Error(w, "Не удалось изменить расход", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expense)
}

// --------- конец нового кода chat

func GetAgreementExpensesHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var agreement models.Agreement

	if err := database.DB.Preload("Expenses.EmployeeCard").First(&agreement, agreementId).Error; err != nil {
		log.Println("[Error] Ошибка в получении данных")
		http.Error(w, "Ошибка при получении данных", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agreement)
}

func DeleteExpensesHandleer(w http.ResponseWriter, r *http.Request) {
	var expensesId = r.PathValue("id")

	if err := database.DB.Delete(&models.Expenses{}, expensesId).Error; err != nil {
		log.Println("[ERROR] Ошибка при удалении элемента расхода")
		http.Error(w, "Ошибка при удалении", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "элемент расходов успешно удален!"})
}

func CreateExpensesHandler(w http.ResponseWriter, r *http.Request) {
	var expenses models.Expenses

	if err := json.NewDecoder(r.Body).Decode(&expenses); err != nil {
		log.Println("[ERROR] Ошибка в получении данных при создании затрат")
		http.Error(w, "Ошибка в полученных данных", http.StatusBadRequest)
		return
	}

	// --------- новый код chat: договор из маршрута и проверка личных расходов
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный договор", 400)
		return
	}
	var agreement models.Agreement
	if err := database.DB.First(&agreement, agreementID).Error; err != nil {
		http.Error(w, "Договор не найден", 404)
		return
	}
	expenses.AgreementId = uint(agreementID)
	// --------- новый код chat: календарная дата без ограничения будущих дат
	if expenses.ExpenseDate == "" {
		expenses.ExpenseDate = time.Now().Format("2006-01-02")
	}
	if err := validateExpenseInput(&expenses); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// --------- конец нового кода chat
	if _, err := service.ExpenseKopecks(expenses.Price); err != nil {
		http.Error(w, "Некорректная сумма расхода", 400)
		return
	}
	if expenses.Account == "employee" {
		var employee models.EmployeeCard
		if expenses.EmployeeCardId == 0 {
			http.Error(w, "Укажите сотрудника для личного расхода", 400)
			return
		}
		if err := database.DB.First(&employee, expenses.EmployeeCardId).Error; err != nil {
			http.Error(w, "Сотрудник не найден", 400)
			return
		}
	}
	expenses.IsReimbursed = false
	// --------- конец нового кода chat
	if err := database.DB.Create(&expenses).Error; err != nil {
		log.Println("[ERROR] Ошибка при сохранении данных данных при создании затрат")
		http.Error(w, "Ошибка при создании расхода", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "элемент трат успешно создан!"})
}

// --------- новый код chat: ручное переключение возмещения только личного расхода
func UpdateExpenseReimbursementHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	expenseID, expenseErr := strconv.ParseUint(r.PathValue("expenseId"), 10, 32)
	if err != nil || expenseErr != nil || agreementID == 0 || expenseID == 0 {
		http.Error(w, "Некорректный идентификатор", 400)
		return
	}
	var input struct {
		IsReimbursed *bool `json:"is_reimbursed"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || input.IsReimbursed == nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Укажите статус возмещения", 400)
		return
	}
	var expense models.Expenses
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND agreement_id = ? AND account = ?", expenseID, agreementID, "employee").First(&expense).Error; err != nil {
			return err
		}
		if err := tx.Model(&expense).Update("is_reimbursed", *input.IsReimbursed).Error; err != nil {
			return err
		}
		return tx.Preload("EmployeeCard").First(&expense, expense.ID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "Личный расход не найден в договоре", 404)
		} else {
			http.Error(w, "Не удалось изменить статус", 500)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expense)
}

// --------- конец нового кода chat
