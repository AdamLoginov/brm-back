// новый код chat: сводка в режиме SQLite только для чтения
package service

import (
	"project/internal/models"
	"testing"
	"time"
)

func TestDashboardReadsCurrentSalaryWithoutWriting(t *testing.T) {
	db, a, _, employee := approvalFixture(t)
	if err := db.AutoMigrate(&models.DailyAllowancePayment{}, &models.DailyAllowanceAccrual{}, &models.Task{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Expenses{AgreementId: a.ID, EmployeeCardId: employee.ID, Price: 187415.31, Account: "company", ExpenseDate: "2026-01-15"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("pragma query_only=on").Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := ReadDashboard(db, start, start.AddDate(0, 1, 0), a.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 || result.Rows[0].Expense != 18741531 || result.Rows[0].Payable != 500000 || result.People != 1 || result.Days[9].Hours != 12 || result.Rows[0].Pending != 1 {
		t.Fatalf("неверная сводка: %+v", result)
	}
}
