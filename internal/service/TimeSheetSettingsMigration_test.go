// --------- новый код chat: переход старых таблиц на новые настройки
package service

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestTimeSheetSettingsMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	statements := []string{
		`CREATE TABLE time_sheet_months (id integer PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, month integer, year integer, agreement_id integer)`,
		`INSERT INTO time_sheet_months (id,month,year,agreement_id) VALUES (1,11,2026,1)`,
		`CREATE TABLE salary_accruals (id integer PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, time_sheet_month_id integer NOT NULL, employee_card_id integer NOT NULL, agreement_id integer NOT NULL, employee_name text, total_hours text, shift_hours integer, salary_rate_kopecks integer, amount_kopecks integer)`,
		`INSERT INTO salary_accruals (id,time_sheet_month_id,employee_card_id,agreement_id,employee_name,total_hours,shift_hours,salary_rate_kopecks,amount_kopecks) VALUES (1,1,1,1,'Old','24',12,500000,1000000)`,
	}
	for _, sql := range statements {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheet{}, &models.TimeSheetMonth{}, &models.SalaryAccrual{}, &models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	var month models.TimeSheetMonth
	if err := db.First(&month, 1).Error; err != nil {
		t.Fatal(err)
	}
	if month.ShiftHours != "12" || month.AbsencePenaltyKopecks != 0 || month.DailyAllowanceKopecks != 0 || month.Month != 11 {
		t.Fatalf("month migration: %+v", month)
	}
	var accrual models.SalaryAccrual
	if err := db.First(&accrual, 1).Error; err != nil {
		t.Fatal(err)
	}
	if accrual.ShiftHours != "12" || accrual.AmountKopecks != 1000000 {
		t.Fatalf("accrual migration: %+v", accrual)
	}
	// Повторный старт приложения не должен сбросить изменённые настройки.
	if err := db.Model(&month).Updates(map[string]any{"shift_hours": "11.5", "absence_penalty_kopecks": 12345, "daily_allowance_kopecks": 70055}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.TimeSheetMonth{}, &models.SalaryAccrual{}, &models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&month, 1).Error; err != nil {
		t.Fatal(err)
	}
	if month.ShiftHours != "11.5" || month.AbsencePenaltyKopecks != 12345 || month.DailyAllowanceKopecks != 70055 {
		t.Fatal("restart reset settings")
	}
}

// --------- конец нового кода chat
