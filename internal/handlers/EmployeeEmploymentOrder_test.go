// --------- новый код chat: необязательные реквизиты приказа и сохранение при отключении статуса
package handlers

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"testing"
)

func TestEmployeeEmploymentOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "order.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err = db.AutoMigrate(&models.EmployeeCard{}, &models.Tool{}, &models.Ppe{}); err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", OfficiallyEmployed: true}
	if err = saveEmployeeProfile(&employee, ""); err != nil {
		t.Fatal("empty optional fields", err)
	}
	id := strconv.Itoa(int(employee.ID))
	employee.EmploymentOrderNumber = " 15-к/2026 "
	employee.EmploymentOrderDate = "2026-11-26"
	if err = saveEmployeeProfile(&employee, id); err != nil {
		t.Fatal(err)
	}
	var saved models.EmployeeCard
	if err = db.First(&saved, employee.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.EmploymentOrderNumber != "15-к/2026" || saved.EmploymentOrderDate != "2026-11-26" {
		t.Fatal("not saved")
	}
	saved.OfficiallyEmployed = false
	if err = saveEmployeeProfile(&saved, id); err != nil {
		t.Fatal(err)
	}
	if err = db.First(&saved, employee.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.OfficiallyEmployed || saved.EmploymentOrderNumber != "15-к/2026" || saved.EmploymentOrderDate != "2026-11-26" {
		t.Fatal("hidden fields lost")
	}
	for _, date := range []string{"2026-02-30", "26.11.2026", "0000-01-01"} {
		saved.EmploymentOrderDate = date
		if err = saveEmployeeProfile(&saved, id); err == nil {
			t.Fatal("invalid date accepted", date)
		}
	}
	saved.OfficiallyEmployed = true
	saved.EmploymentOrderDate = ""
	saved.EmploymentOrderNumber = ""
	if err = saveEmployeeProfile(&saved, id); err != nil {
		t.Fatal("clear optional fields", err)
	}
	if err = db.First(&saved, employee.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.EmploymentOrderDate != "" || saved.EmploymentOrderNumber != "" {
		t.Fatal("fields not cleared")
	}
}

// --------- конец нового кода chat
