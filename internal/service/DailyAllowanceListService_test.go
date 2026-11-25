// --------- новый код chat: список, отмена начислений и сохранение фактических выплат
package service

import (
	"project/internal/models"
	"testing"
)

func TestDailyAllowanceDisabledList(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	for _, item := range []struct {
		month uint
		date  string
	}{{january.ID, "2026-01-10"}, {february.ID, "2026-02-10"}} {
		if err := db.Create(&models.TimeSheet{AgreementID: agreement.ID, TimeSheetMonthID: item.month, EmployeeCardID: employee.ID, Date: item.date, Status: "11"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	payments, err := CreateDailyAllowancePayments(db, agreement.ID, DailyAllowanceBatchInput{MonthID: january.ID, Date: "2026-01-10", RequestKey: "list-initial-payment", Payments: []DailyAllowancePaymentItem{{EmployeeCardID: employee.ID, Amount: "1000"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveDailyAllowanceList(db, agreement.ID, january.ID, []uint{}); err != nil {
		t.Fatal(err)
	}
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-01-31", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.Enabled || row.SuggestedKopecks != 0 || row.BalanceKopecks != 40000 {
		t.Fatalf("disabled calculation: %+v", row)
	}
	for _, cell := range row.Cells {
		if cell.AmountKopecks != 0 || cell.PreviewRequiredKopecks != 0 {
			t.Fatal("disabled day charged")
		}
	}
	var saved models.DailyAllowancePayment
	if err := db.First(&saved, payments[0].ID).Error; err != nil || saved.AmountKopecks != 100000 {
		t.Fatal("history changed", err)
	}
	_, err = CreateDailyAllowancePayments(db, agreement.ID, DailyAllowanceBatchInput{MonthID: january.ID, Date: "2026-01-11", RequestKey: "list-disabled-payment", Payments: []DailyAllowancePaymentItem{{EmployeeCardID: employee.ID, Amount: "500"}}})
	if err == nil {
		t.Fatal("disabled employee paid")
	}
	if _, err := UpdateDailyAllowancePayment(db, agreement.ID, saved.ID, DailyAllowancePaymentUpdate{Date: saved.Date, Amount: "1500"}); err == nil {
		t.Fatal("disabled payment increased")
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil || !dailyTableEmployee(t, table, employee.ID).Enabled {
		t.Fatal("other month affected", err)
	}
	if err := SaveDailyAllowanceList(db, agreement.ID+999, january.ID, []uint{employee.ID}); err == nil {
		t.Fatal("cross agreement update")
	}
	if err := SaveDailyAllowanceList(db, agreement.ID, january.ID, []uint{999999}); err == nil {
		t.Fatal("nonmember accepted")
	}
	if err := SaveDailyAllowanceList(db, agreement.ID, january.ID, []uint{employee.ID}); err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	if !row.Enabled || row.BalanceKopecks != -10000 {
		t.Fatalf("reenabled calculation: %+v", row)
	}
}

// --------- конец нового кода chat
