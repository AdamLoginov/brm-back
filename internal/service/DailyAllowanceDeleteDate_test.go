// --------- новый код chat: удаление дня не затрагивает другие даты и договоры
package service

import (
	"errors"
	"fmt"
	"project/internal/models"
	"testing"
)

func TestDailyAllowanceDeleteDate(t *testing.T) {
	db, agreement, employee, month, _ := dailyFixture(t)
	other := models.Agreement{Name: "Другой", Number: "OTHER"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	payments := []models.DailyAllowancePayment{
		{AgreementID: agreement.ID, Date: "2026-01-20"},
		{AgreementID: agreement.ID, Date: "2026-01-20"},
		{AgreementID: agreement.ID, Date: "2026-01-21"},
		{AgreementID: other.ID, Date: "2026-01-20"},
	}
	for i := range payments {
		payments[i].EmployeeCardID = employee.ID
		payments[i].TimeSheetMonthID = month.ID
		payments[i].AmountKopecks = 50000
		payments[i].RequestKey = fmt.Sprintf("delete-date-%d", i)
		if err := db.Create(&payments[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := DeleteDailyAllowancePaymentsByDate(db, agreement.ID, "2026-02-30"); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatalf("invalid date: %v", err)
	}
	count, err := DeleteDailyAllowancePaymentsByDate(db, agreement.ID, "2026-01-20")
	if err != nil || count != 2 {
		t.Fatalf("deleted %d: %v", count, err)
	}
	var remaining []models.DailyAllowancePayment
	if err := db.Order("id").Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 || remaining[0].ID != payments[2].ID || remaining[1].ID != payments[3].ID {
		t.Fatalf("remaining: %+v", remaining)
	}
	count, err = DeleteDailyAllowancePaymentsByDate(db, agreement.ID, "2026-01-20")
	if err != nil || count != 0 {
		t.Fatalf("repeat: %d %v", count, err)
	}
	table, err := GetDailyAllowanceTable(db, agreement.ID, month.ID, "", dailyNow("2026-01-25"))
	if err != nil {
		t.Fatal(err)
	}
	if row := dailyTableEmployee(t, table, employee.ID); row.BalanceKopecks != 50000 {
		t.Fatalf("balance: %+v", row)
	}
}

// --------- конец нового кода chat
