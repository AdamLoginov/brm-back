// --------- новый код chat: автоматические штрафы и дробная норма смены
package service

import (
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
	"time"
)

func TestTimeSheetAutomaticPenalties(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	agreement, month, employee := salaryFixture(t, db)
	if err := db.Model(&month).Updates(map[string]any{"shift_hours": "11.5", "absence_penalty_kopecks": 12345}).Error; err != nil {
		t.Fatal(err)
	}
	entries := []models.TimeSheet{{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: "2026-01-10", Status: "П"}}
	if err := ChangeTimeSheetEntries(db, entries, "create"); err != nil {
		t.Fatal(err)
	}
	var penalty models.SalaryPenalty
	if err := db.Where("source_time_sheet_month_id = ?", month.ID).First(&penalty).Error; err != nil {
		t.Fatal(err)
	}
	if penalty.AmountKopecks != 12345 || penalty.Comment != "Прогул" || penalty.Date != entries[0].Date || penalty.AgreementID != agreement.ID || penalty.SourceTimeSheetID == nil || *penalty.SourceTimeSheetID != entries[0].ID {
		t.Fatalf("penalty: %+v", penalty)
	}
	if err := ChangeTimeSheetEntries(db, entries, "create"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.SalaryPenalty{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate penalty")
	}
	if _, err := ChangeSalaryPenaltyStatus(db, agreement.ID, penalty.ID, SalaryPenaltyForgiven); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&month).Update("absence_penalty_kopecks", 20000).Error; err != nil {
			return err
		}
		return SyncTimeSheetPenalties(tx, month.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&penalty, penalty.ID).Error; err != nil {
		t.Fatal(err)
	}
	if penalty.AmountKopecks != 20000 || penalty.Status != SalaryPenaltyForgiven || penalty.ClosedAt == nil {
		t.Fatal("status lost on amount update")
	}
	// Удаляем прощённый штраф заменой П на часы.
	entries[0].Status = "12"
	if err := ChangeTimeSheetEntries(db, entries, "update"); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.SalaryPenalty{}).Count(&count)
	if count != 0 {
		t.Fatal("forgiven penalty not deleted")
	}
	entries[0].Status = "П"
	if err := ChangeTimeSheetEntries(db, entries, "update"); err != nil {
		t.Fatal(err)
	}
	penalty = models.SalaryPenalty{}
	if err := db.First(&penalty).Error; err != nil {
		t.Fatal(err)
	}
	if penalty.Status != SalaryPenaltyActive || penalty.ClosedAt != nil {
		t.Fatal("new absence reused forgiven state")
	}
	closedAt := time.Now().UTC()
	if err := db.Model(&penalty).Updates(map[string]any{"status": SalaryPenaltyClosed, "closed_at": closedAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&month).Update("absence_penalty_kopecks", 30000).Error; err != nil {
			return err
		}
		return SyncTimeSheetPenalties(tx, month.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&penalty, penalty.ID).Error; err != nil {
		t.Fatal(err)
	}
	if penalty.AmountKopecks != 30000 || penalty.Status != SalaryPenaltyClosed || !penalty.ClosedAt.Equal(closedAt) {
		t.Fatal("closed state lost")
	}
	if err := ChangeTimeSheetEntries(db, entries, "delete"); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.SalaryPenalty{}).Count(&count)
	if count != 0 {
		t.Fatal("closed penalty not deleted")
	}
	// Ручной штраф остаётся при удалении сотрудника и обнулении настройки.
	manual := models.SalaryPenalty{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-10", AmountKopecks: 99}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatal(err)
	}
	entries[0].ID = 0
	if err := ChangeTimeSheetEntries(db, entries, "create"); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&month).Association("Employees").Clear(); err != nil {
			return err
		}
		return SyncTimeSheetPenalties(tx, month.ID)
	}); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.SalaryPenalty{}).Count(&count)
	if count != 1 {
		t.Fatal("employee removal affected manual penalty or left automatic")
	}
	if err := db.Model(&month).Association("Employees").Append(&employee); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return SyncTimeSheetPenalties(tx, month.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&month).Update("absence_penalty_kopecks", 0).Error; err != nil {
			return err
		}
		return SyncTimeSheetPenalties(tx, month.ID)
	}); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.SalaryPenalty{}).Count(&count)
	if count != 1 {
		t.Fatal("zero setting did not clear automatic penalties")
	}
}

// --------- новый код chat: граница увольнения, запрет API и очистка связанных данных
func TestTimeSheetDismissal(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	_, month, employee := salaryFixture(t, db)
	if err := db.Model(&month).Updates(map[string]any{"absence_penalty_kopecks": 10000, "daily_allowance_kopecks": 50000}).Error; err != nil {
		t.Fatal(err)
	}
	entries := []models.TimeSheet{
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: "2026-01-09", Status: "11"},
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: "2026-01-10", Status: "П"},
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: "2026-01-11", Status: "11"},
	}
	if err := ChangeTimeSheetEntries(db, entries, "create"); err != nil {
		t.Fatal(err)
	}
	employee.Status, employee.DismissalDate = true, "2026-01-10"
	if err := db.Model(&employee).Updates(map[string]any{"status": true, "dismissal_date": employee.DismissalDate}).Error; err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-01-10", "2026-01-31", "2026-02-01"} {
		if err := ChangeTimeSheetEntries(db, []models.TimeSheet{{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: date, Status: "12"}}, "create"); err == nil {
			t.Fatalf("accepted dismissed date %s", date)
		}
	}
	if err := ChangeTimeSheetEntries(db, entries[1:2], "update"); err == nil {
		t.Fatal("updated dismissed day")
	}
	entries[0].Status = "12"
	if err := ChangeTimeSheetEntries(db, entries[:1], "update"); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ClearDismissedEmployeeTimeSheets(tx, employee) }); err != nil {
		t.Fatal(err)
	}
	var remaining []models.TimeSheet
	if err := db.Where("employee_card_id = ?", employee.ID).Find(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Date != "2026-01-09" || remaining[0].Status != "12" {
		t.Fatalf("wrong remaining entries: %+v", remaining)
	}
	var penalties int64
	db.Model(&models.SalaryPenalty{}).Count(&penalties)
	if penalties != 0 {
		t.Fatal("dismissal penalty remains")
	}
	var accruals []models.DailyAllowanceAccrual
	if err := db.Find(&accruals).Error; err != nil {
		t.Fatal(err)
	}
	if len(accruals) != 1 {
		t.Fatalf("expected only pre-dismissal daily accrual, got %d", len(accruals))
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ClearDismissedEmployeeTimeSheets(tx, employee) }); err != nil {
		t.Fatal(err)
	}
}

// --------- конец нового кода chat

func TestTimeSheetPenaltyRollbackAndShiftCalculation(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	_, month, employee := salaryFixture(t, db)
	if err := db.Model(&month).Updates(map[string]any{"shift_hours": "11.5", "absence_penalty_kopecks": 10000}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_penalty BEFORE INSERT ON salary_penalties BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	rows := []models.TimeSheet{{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Date: "2026-01-10", Status: "П"}}
	if err := ChangeTimeSheetEntries(db, rows, "create"); err == nil {
		t.Fatal("expected failure")
	}
	var count int64
	db.Model(&models.TimeSheet{}).Count(&count)
	if count != 0 {
		t.Fatal("failed penalty left saved timesheet")
	}
	if err := db.Exec("DROP TRIGGER reject_penalty").Error; err != nil {
		t.Fatal(err)
	}
	rows[0].Status = "23"
	rows[0].ID = 0
	if err := ChangeTimeSheetEntries(db, rows, "create"); err != nil {
		t.Fatal(err)
	}
	accruals, err := CalculateSalaryAccruals(db, month.AgreementID, month.ID)
	if err != nil || len(accruals) != 1 || accruals[0].AmountKopecks != 1000000 || accruals[0].ShiftHours != "11.5" {
		t.Fatalf("fractional shift: %+v %v", accruals, err)
	}
	if err := db.Model(&month).Update("shift_hours", "9").Error; err != nil {
		t.Fatal(err)
	}
	accruals, err = CalculateSalaryAccruals(db, month.AgreementID, month.ID)
	if err != nil || accruals[0].AmountKopecks != 1277778 || accruals[0].ShiftHours != "9" {
		t.Fatalf("recalculation: %+v %v", accruals, err)
	}
}

func TestTimeSheetSettingsValidation(t *testing.T) {
	for _, value := range []string{"0", "-1", "25", "abc", "", "1.234"} {
		if _, _, err := ParseTimeSheetSettings(&value, nil, "12", 0); err == nil {
			t.Fatal("invalid hours", value)
		}
	}
	for _, value := range []string{"-1", "abc", "", "1.234", "999999999999999999999"} {
		if _, _, err := ParseTimeSheetSettings(nil, &value, "12", 0); err == nil {
			t.Fatal("invalid penalty", value)
		}
	}
	hours, amount := "11,5", "123,45"
	h, p, err := ParseTimeSheetSettings(&hours, &amount, "12", 0)
	if err != nil || h != "11.5" || p != 12345 {
		t.Fatal(h, p, err)
	}
}

// --------- конец нового кода chat
