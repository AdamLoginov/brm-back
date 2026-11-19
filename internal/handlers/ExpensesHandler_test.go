// --------- новый код chat: статусы возмещения и границы договора
package handlers

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"testing"
)

func TestExpenseReimbursementStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "expenses.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	// Старые расходы получают false при миграции.
	if err := db.Exec("CREATE TABLE expenses (id integer PRIMARY KEY, account text)").Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO expenses (id, account) VALUES (1, 'employee')")
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.Expenses{}); err != nil {
		t.Fatal(err)
	}
	var old models.Expenses
	db.First(&old, 1)
	if old.IsReimbursed {
		t.Fatal("old expense reimbursed")
	}
	agreement := models.Agreement{Name: "Договор"}
	db.Create(&agreement)
	employee := models.EmployeeCard{Name: "Иван"}
	db.Create(&employee)
	expense := models.Expenses{AgreementId: agreement.ID, EmployeeCardId: employee.ID, Account: "employee", Price: 150}
	db.Create(&expense)
	call := func(agreementID uint, body string) int {
		req := httptest.NewRequest("PATCH", "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreementID)))
		req.SetPathValue("expenseId", strconv.Itoa(int(expense.ID)))
		res := httptest.NewRecorder()
		UpdateExpenseReimbursementHandler(res, req)
		return res.Code
	}
	for _, value := range []bool{true, false} {
		if code := call(agreement.ID, `{"is_reimbursed":`+strconv.FormatBool(value)+`}`); code != 200 {
			t.Fatalf("status %d", code)
		}
		db.First(&expense, expense.ID)
		if expense.IsReimbursed != value {
			t.Fatal("status not saved")
		}
	}
	if code := call(agreement.ID+100, `{"is_reimbursed":true}`); code != 404 {
		t.Fatal("cross-contract mutation")
	}
	for _, body := range []string{`{}`, `{"is_reimbursed":null}`, `{"is_reimbursed":"true"}`, `{"is_reimbursed":true} {}`} {
		if code := call(agreement.ID, body); code != 400 {
			t.Fatal("invalid status accepted")
		}
	}
	db.Model(&expense).Update("account", "company")
	if code := call(agreement.ID, `{"is_reimbursed":true}`); code != 404 {
		t.Fatal("company expense accepted")
	}
}

// --------- конец нового кода chat
// --------- новый код chat: даты, редактирование и сохранение статуса
func TestExpenseEditing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "edit.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.Expenses{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Test"}
	db.Create(&agreement)
	employee := models.EmployeeCard{Name: "Test"}
	db.Create(&employee)
	call := func(id uint, expenseID uint, body string, create bool) int {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(id)))
		req.SetPathValue("expenseId", strconv.Itoa(int(expenseID)))
		res := httptest.NewRecorder()
		if create {
			CreateExpensesHandler(res, req)
		} else {
			UpdateExpensesHandler(res, req)
		}
		return res.Code
	}
	body := `{"name":"Test","price":123.45,"account":"employee","employee_card_id":` + strconv.Itoa(int(employee.ID)) + `,"expense_date":"2026-09-15"}`
	if code := call(agreement.ID, 0, body, true); code != 200 {
		t.Fatalf("create: %d", code)
	}
	var expense models.Expenses
	db.First(&expense)
	created := expense.CreatedAt
	if expense.ExpenseDate != "2026-09-15" {
		t.Fatal("date not saved")
	}
	db.Model(&expense).Update("is_reimbursed", true)
	body = strings.ReplaceAll(body, "2026-09-15", "2030-12-31")
	if code := call(agreement.ID, expense.ID, body, false); code != 200 {
		t.Fatalf("edit: %d", code)
	}
	db.First(&expense, expense.ID)
	if !expense.IsReimbursed || expense.ExpenseDate != "2030-12-31" || !expense.CreatedAt.Equal(created) {
		t.Fatal("date/status changed incorrectly")
	}
	if code := call(agreement.ID+100, expense.ID, body, false); code != 404 {
		t.Fatal("cross-agreement update")
	}
	for _, date := range []string{"", "2026-02-30", "2026-1-1"} {
		if code := call(agreement.ID, expense.ID, strings.ReplaceAll(body, "2030-12-31", date), false); code != 400 {
			t.Fatal("invalid date accepted")
		}
	}
	if code := call(agreement.ID, expense.ID, strings.ReplaceAll(body, `"account":"employee"`, `"account":"company"`), false); code != 200 {
		t.Fatal("account update failed")
	}
	db.First(&expense, expense.ID)
	if expense.IsReimbursed {
		t.Fatal("reimbursement not reset")
	}
	// --------- новый код chat: категория сохраняется и сбрасывается без изменения возмещения
	category := models.ExpenseCategory{Name: "Проезд", NameKey: "проезд"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	withCategory := strings.TrimSuffix(body, "}") + `,"category_id":` + strconv.Itoa(int(category.ID)) + `}`
	if call(agreement.ID, expense.ID, withCategory, false) != 200 {
		t.Fatal("category update failed")
	}
	var categorized models.Expenses
	db.First(&categorized, expense.ID)
	if categorized.CategoryID == nil || *categorized.CategoryID != category.ID {
		t.Fatal("category not saved")
	}
	if call(agreement.ID, 0, withCategory, true) != 200 {
		t.Fatal("create with category failed")
	}
	if call(agreement.ID, expense.ID, strings.TrimSuffix(body, "}")+`,"category_id":99999}`, false) != 400 {
		t.Fatal("unknown category accepted")
	}
	if call(agreement.ID, expense.ID, strings.TrimSuffix(body, "}")+`,"category_id":null}`, false) != 200 {
		t.Fatal("category reset failed")
	}
	var cleared models.Expenses
	db.First(&cleared, expense.ID)
	if cleared.CategoryID != nil {
		t.Fatal("category not cleared")
	}
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
