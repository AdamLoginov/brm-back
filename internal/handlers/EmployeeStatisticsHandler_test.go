// --------- новый код chat: API статистики, проверка параметров и авторизации
package handlers

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"testing"
)

func TestEmployeeStatisticsAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "stats.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err = db.AutoMigrate(&models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.EmployeeCard{Name: "Иван"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, year string
		status   int
	}{{"1", "2026", 200}, {"1", "0", 400}, {"1", "10000", 400}, {"1", "abc", 400}, {"0", "2026", 400}, {"999", "2026", 404}} {
		req := httptest.NewRequest("GET", "/?year="+item.year, nil)
		req.SetPathValue("id", item.id)
		res := httptest.NewRecorder()
		EmployeeStatisticsHandler(res, req)
		if res.Code != item.status {
			t.Fatal(item, res.Code, res.Body.String())
		}
	}
	req := httptest.NewRequest("GET", "/?year=2026", nil)
	req.SetPathValue("id", "1")
	res := httptest.NewRecorder()
	middleware.AuthMiddleware(EmployeeStatisticsHandler)(res, req)
	if res.Code != 401 {
		t.Fatal("anonymous allowed")
	}
}

// --------- конец нового кода chat
