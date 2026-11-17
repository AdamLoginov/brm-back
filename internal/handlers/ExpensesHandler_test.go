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
