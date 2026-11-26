// --------- новый код chat: проверки расчёта денег и сохранения начислений
package service

import (
	"errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"math/big"
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestSalaryRateAndRounding(t *testing.T) {
	for _, item := range []struct {
		input string
		want  int64
	}{
		{"", 0}, {"не указано", 0}, {"-500", 0}, {"NaN", 0}, {"1e3", 0},
		{"5000", 500000}, {" 5 000,50 ", 500050}, {"0.01", 1}, {"1.234", 0},
		{"99999999999999999999999999", 0},
	} {
		if got := salaryRateKopecks(item.input); got != item.want {
			t.Errorf("rate %q: %d, want %d", item.input, got, item.want)
		}
	}
	for _, item := range []struct {
		hours      string
		rate, want int64
	}{
		{"12", 500000, 500000}, {"18", 500000, 750000}, {"11", 500000, 458333},
		{"2", 100, 17}, {"6", 1, 1}, {"1.5", 120000, 15000}, {"0", 100, 0},
	} {
		got, err := salaryAmountKopecks(parseSalaryDecimal(item.hours), item.rate, "12")
		if err != nil || got != item.want {
			t.Errorf("hours %s rate %d: %d %v, want %d", item.hours, item.rate, got, err, item.want)
		}
	}
	if _, err := salaryAmountKopecks(big.NewRat(24, 1), maxSalaryKopecks, "12"); !errors.Is(err, ErrSalaryAmountOverflow) {
		t.Fatal("overflow must be rejected")
	}
}

func salaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "salary.db")+"?_foreign_keys=on"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	// новый код chat: история видимости общих алиментов
	if err := db.AutoMigrate(&models.SalaryAlimonyEmployeeHistory{}, &models.SalaryAlimony{}, &models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.DailyAllowanceAccrual{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func salaryFixture(t *testing.T, db *gorm.DB) (models.Agreement, models.TimeSheetMonth, models.EmployeeCard) {
	t.Helper()
	agreement := models.Agreement{Name: "Тест", Number: "1"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Сотрудник", AgreementID: agreement.ID, SalaryRate: "5000"}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	month := models.TimeSheetMonth{Month: 1, Year: 2026, AgreementID: agreement.ID, Employees: []models.EmployeeCard{employee}}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	return agreement, month, employee
}

func TestSalaryAccrualRecalculationAndIsolation(t *testing.T) {
	db := salaryTestDB(t)
	agreement, month, employee := salaryFixture(t, db)
	otherAgreement, otherMonth, _ := salaryFixture(t, db)
	if err := db.Model(&otherMonth).Association("Employees").Append(&employee); err != nil {
		t.Fatal(err)
	}
	entries := []models.TimeSheet{
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Status: "11"},
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Status: "Б"},
		{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Status: "Д"},
		{TimeSheetMonthID: otherMonth.ID, EmployeeCardID: employee.ID, Status: "12"},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := CalculateSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("calculate: %v %v", rows, err)
	}
	firstID := rows[0].ID
	if rows[0].AmountKopecks != 458333 || rows[0].TotalHours != "11.00" {
		t.Fatalf("unexpected calculation: %+v", rows[0])
	}
	if err := db.Model(&employee).Update("salary_rate", "6000").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&entries[0]).Update("status", "18").Error; err != nil {
		t.Fatal(err)
	}
	stored, err := GetSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || stored[0].AmountKopecks != 458333 {
		t.Fatal("GET must not recalculate")
	}
	rows, err = CalculateSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != firstID || rows[0].AmountKopecks != 900000 {
		t.Fatalf("recalculate: %+v %v", rows, err)
	}
	if _, err := CalculateSalaryAccruals(db, agreement.ID, otherMonth.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("another agreement's month must be rejected")
	}
	otherRows, err := CalculateSalaryAccruals(db, otherAgreement.ID, otherMonth.ID)
	if err != nil || len(otherRows) != 2 {
		t.Fatalf("other agreement: %+v %v", otherRows, err)
	}
	if err := db.Model(&month).Association("Employees").Clear(); err != nil {
		t.Fatal(err)
	}
	rows, err = CalculateSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || len(rows) != 0 {
		t.Fatal("removed employees must lose accruals")
	}
	if err := db.Model(&month).Association("Employees").Append(&employee); err != nil {
		t.Fatal(err)
	}
	rows, err = CalculateSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || len(rows) != 1 {
		t.Fatal("employee must be calculable after returning")
	}
	var count int64
	db.Unscoped().Model(&models.SalaryAccrual{}).Where("time_sheet_month_id = ?", month.ID).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate accruals: %d", count)
	}
	if err := db.Select("TimeSheets", "SalaryAccruals").Delete(&month).Error; err != nil {
		t.Fatal(err)
	}
	db.Model(&models.SalaryAccrual{}).Where("time_sheet_month_id = ?", month.ID).Count(&count)
	if count != 0 {
		t.Fatal("deleted month still has visible accruals")
	}
	if _, err := CalculateSalaryAccruals(db, agreement.ID, month.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("deleted month must not be recalculated")
	}
	stored, err = GetSalaryAccruals(db, otherAgreement.ID, otherMonth.ID)
	if err != nil || len(stored) != 2 {
		t.Fatal("deleting one month affected another")
	}
}

func TestSalaryZeroRatesAndTransactionRollback(t *testing.T) {
	db := salaryTestDB(t)
	agreement, month, employee := salaryFixture(t, db)
	employeeTwo := models.EmployeeCard{Name: "Второй", AgreementID: agreement.ID, SalaryRate: "не указано"}
	if err := db.Create(&employeeTwo).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&month).Association("Employees").Append(&employeeTwo); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TimeSheet{TimeSheetMonthID: month.ID, EmployeeCardID: employeeTwo.ID, Status: "24"}).Error; err != nil {
		t.Fatal(err)
	}
	rows, err := CalculateSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("zero rates: %v", err)
	}
	for _, row := range rows {
		if row.AmountKopecks != 0 {
			t.Fatal("missing hours / invalid rate must yield zero")
		}
	}
	if err := db.Create(&models.TimeSheet{TimeSheetMonthID: month.ID, EmployeeCardID: employee.ID, Status: "12"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER fail_salary_update BEFORE UPDATE ON salary_accruals WHEN NEW.employee_name = 'Второй' BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := CalculateSalaryAccruals(db, agreement.ID, month.ID); err == nil {
		t.Fatal("expected transaction failure")
	}
	stored, err := GetSalaryAccruals(db, agreement.ID, month.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range stored {
		if row.AmountKopecks != 0 {
			t.Fatal("part of failed recalculation was committed")
		}
	}
}

// --------- конец нового кода chat
