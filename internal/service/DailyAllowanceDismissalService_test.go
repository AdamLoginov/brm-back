// --------- новый код chat: увольнение исключает начисления и прогноз, сохраняя выплаты
package service

import (
	"project/internal/models"
	"testing"
)

func TestDailyAllowanceDismissal(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	for _, date := range []string{"2026-01-09", "2026-01-10", "2026-01-11"} {
		entry := models.TimeSheet{AgreementID: agreement.ID, EmployeeCardID: employee.ID, TimeSheetMonthID: january.ID, Date: date, Status: "11"}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := SyncDailyAllowanceAccruals(db, january.ID); err != nil {
		t.Fatal(err)
	}
	payment := models.DailyAllowancePayment{AgreementID: agreement.ID, EmployeeCardID: employee.ID, AmountKopecks: 200000, Date: "2026-01-01"}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatal(err)
	}
	// Даже при старом закреплении за договором прогноз обязан учитывать увольнение.
	if err := db.Model(&employee).Updates(map[string]any{"status": true, "dismissal_date": "2026-01-10"}).Error; err != nil {
		t.Fatal(err)
	}
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-02-28", dailyNow("2026-01-12"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 150000 || row.SuggestedKopecks != 0 {
		t.Fatalf("balance or forecast: %+v", row)
	}
	for _, cell := range row.Cells {
		if cell.Date >= "2026-01-10" && (cell.State != "dismissed" || cell.AmountKopecks != 0 || cell.CoveredKopecks != 0 || cell.PreviewRequiredKopecks != 0) {
			t.Fatalf("dismissed day charged: %+v", cell)
		}
	}
	var count int64
	db.Model(&models.DailyAllowanceAccrual{}).Count(&count)
	if count != 1 {
		t.Fatalf("old accruals remain: %d", count)
	}
	var saved models.DailyAllowancePayment
	if err := db.First(&saved, payment.ID).Error; err != nil || saved.AmountKopecks != 200000 {
		t.Fatal("payment changed", err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-02-15"))
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range dailyTableEmployee(t, table, employee.ID).Cells {
		if cell.State != "dismissed" {
			t.Fatalf("later month not closed: %+v", cell)
		}
	}
}

// --------- конец нового кода chat
