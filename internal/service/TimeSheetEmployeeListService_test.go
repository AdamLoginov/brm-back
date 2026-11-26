// --------- новый код chat: скрытие сохраняет часы, штрафы и зарплату, отменяет только суточные месяца
package service

import (
	"project/internal/models"
	"testing"
)

func TestTimeSheetEmployeeVisibility(t *testing.T) {
	db, a, e, january, february := dailyFixture(t)
	if err := db.Model(&e).Update("salary_rate", "1200").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&january).Updates(map[string]any{"shift_hours": "12", "absence_penalty_kopecks": 10000}).Error; err != nil {
		t.Fatal(err)
	}
	dailyEntries(t, db, a.ID, january.ID, e.ID, map[string]string{"2026-01-10": "12", "2026-01-11": "П"})
	dailyEntries(t, db, a.ID, february.ID, e.ID, map[string]string{"2026-02-10": "12"})
	if err := SyncTimeSheetPenalties(db, january.ID); err != nil {
		t.Fatal(err)
	}
	paid, err := CreateDailyAllowancePayments(db, a.ID, DailyAllowanceBatchInput{MonthID: january.ID, Date: "2026-01-10", RequestKey: "visibility-initial", Payments: []DailyAllowancePaymentItem{{EmployeeCardID: e.ID, Amount: "1000"}}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := CalculateSalaryAccruals(db, a.ID, january.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = SaveTimeSheetEmployeeList(db, a.ID, january.ID, []uint{}); err != nil {
		t.Fatal(err)
	}
	var month models.TimeSheetMonth
	db.Preload("Employees").First(&month, january.ID)
	if len(month.Employees) != 1 || !TimeSheetEmployeeHidden(month, e.ID) {
		t.Fatal("membership lost", month)
	}
	var count int64
	db.Model(&models.TimeSheet{}).Where("time_sheet_month_id = ?", january.ID).Count(&count)
	if count != 2 {
		t.Fatal("marks deleted")
	}
	db.Model(&models.SalaryPenalty{}).Where("agreement_id = ?", a.ID).Count(&count)
	if count != 1 {
		t.Fatal("penalty deleted")
	}
	after, err := CalculateSalaryAccruals(db, a.ID, january.ID)
	if err != nil || len(after) != 1 || after[0].AmountKopecks != before[0].AmountKopecks || after[0].TotalHours != before[0].TotalHours {
		t.Fatal("salary changed", after, err)
	}
	table, err := GetDailyAllowanceTable(db, a.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil || len(table.Employees) != 0 {
		t.Fatal("hidden visible in daily", table, err)
	}
	db.Model(&models.DailyAllowanceAccrual{}).Where("time_sheet_month_id = ?", january.ID).Count(&count)
	if count != 0 {
		t.Fatal("hidden daily accrued")
	}
	var payment models.DailyAllowancePayment
	if err = db.First(&payment, paid[0].ID).Error; err != nil || payment.AmountKopecks != 100000 {
		t.Fatal("payment lost", err)
	}
	if err = SaveDailyAllowanceList(db, a.ID, january.ID, []uint{e.ID}); err == nil {
		t.Fatal("daily list bypass")
	}
	if _, err = CreateDailyAllowancePayments(db, a.ID, DailyAllowanceBatchInput{MonthID: january.ID, Date: "2026-01-11", RequestKey: "visibility-blocked", Payments: []DailyAllowancePaymentItem{{EmployeeCardID: e.ID, Amount: "500"}}}); err == nil {
		t.Fatal("hidden paid")
	}
	table, err = GetDailyAllowanceTable(db, a.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil || len(table.Employees) != 1 {
		t.Fatal("other month affected", err)
	}
	if err = SaveTimeSheetEmployeeList(db, a.ID, january.ID, []uint{e.ID}); err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, a.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil || len(table.Employees) != 1 || !table.Employees[0].Enabled {
		t.Fatal("restore failed", err)
	}
	db.Model(&models.DailyAllowanceAccrual{}).Where("time_sheet_month_id = ?", january.ID).Count(&count)
	if count != 1 {
		t.Fatal("daily not restored")
	}
}
func TestTimeSheetEmployeeListCandidates(t *testing.T) {
	db, a, e, january, february := dailyFixture(t)
	dismissed := models.EmployeeCard{Name: "Уволен", AgreementID: a.ID, Status: true, DismissalDate: "2026-01-15"}
	otherAgreement := models.Agreement{Name: "Другой договор"}
	if err := db.Create(&otherAgreement).Error; err != nil {
		t.Fatal(err)
	}
	foreign := models.EmployeeCard{Name: "Другой договор", AgreementID: otherAgreement.ID}
	if err := db.Create(&dismissed).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err := SaveTimeSheetEmployeeList(db, a.ID, january.ID, []uint{e.ID, dismissed.ID}); err != nil {
		t.Fatal(err)
	}
	if err := SaveTimeSheetEmployeeList(db, a.ID, february.ID, []uint{e.ID, dismissed.ID}); err == nil {
		t.Fatal("dismissed added after month")
	}
	if err := SaveTimeSheetEmployeeList(db, a.ID, january.ID, []uint{foreign.ID}); err == nil {
		t.Fatal("foreign added")
	}
	if err := SaveTimeSheetEmployeeList(db, a.ID+99, january.ID, []uint{}); err == nil {
		t.Fatal("wrong agreement")
	}
	if err := SaveTimeSheetEmployeeList(db, a.ID, january.ID, []uint{e.ID, e.ID}); err == nil {
		t.Fatal("duplicates")
	}
	var month models.TimeSheetMonth
	db.Preload("Employees").First(&month, january.ID)
	if len(month.Employees) != 2 || len(month.HiddenEmployeeIDs) != 0 {
		t.Fatal("invalid request modified month")
	}
}

// --------- конец нового кода chat
