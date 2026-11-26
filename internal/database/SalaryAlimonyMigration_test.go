// --------- новый код chat: старые суммы удаляются только при смене структуры
package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestEmployeeAlimonyMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.Exec("CREATE TABLE salary_alimonies (id integer PRIMARY KEY, agreement_id integer, time_sheet_month_id integer, employee_card_id integer, amount_kopecks integer)").Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO salary_alimonies VALUES (1,1,1,1,10000)").Error; err != nil {
		t.Fatal(err)
	}
	if err = migrateEmployeeAlimony(db); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn("salary_alimonies", "agreement_id") || db.Migrator().HasColumn("salary_alimonies", "time_sheet_month_id") {
		t.Fatal("legacy columns retained")
	}
	var count int64
	if err = db.Model(&models.SalaryAlimony{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("legacy amount retained", err)
	}
	for i := 0; i < 2; i++ {
		if err = db.Create(&models.SalaryAlimony{EmployeeCardID: 1, Date: "2026-11-26", AmountKopecks: 10000}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = migrateEmployeeAlimony(db); err != nil {
		t.Fatal(err)
	}
	if err = db.Model(&models.SalaryAlimony{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("repeat migration deleted records", count, err)
	}
}

// --------- конец нового кода chat
