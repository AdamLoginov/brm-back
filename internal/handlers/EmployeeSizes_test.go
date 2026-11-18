// --------- новый код chat: миграция, сохранение и очистка размеров
package handlers

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strings"
	"testing"
)

func TestEmployeeSizesAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sizes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.Exec("CREATE TABLE employee_cards (id integer PRIMARY KEY, name text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO employee_cards (id, name) VALUES (1, 'Иван')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.EmployeeCard{}); err != nil {
		t.Fatal(err)
	}
	var legacy models.EmployeeCard
	if err := db.First(&legacy, 1).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.IsITR || legacy.HeightCM != nil || legacy.ClothingSize != "" || legacy.ShoeSize != "" {
		t.Fatalf("migration: %+v", legacy)
	}
	call := func(handler http.HandlerFunc, body string, id string) int {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", id)
		res := httptest.NewRecorder()
		handler(res, req)
		return res.Code
	}
	for _, handler := range []http.HandlerFunc{CreateEmployeeCardHandler, UpdateEmployeeCardHandler} {
		for _, body := range []string{
			`{"name":"Иван","height_cm":0}`,
			`{"name":"Иван","height_cm":-1}`,
			`{"name":"Иван","height_cm":180.5}`,
			`{"name":"Иван","is_itr":"true"}`,
		} {
			if code := call(handler, body, "1"); code != 400 {
				t.Fatalf("invalid status=%d", code)
			}
		}
	}
	body := `{"name":"Анна","clothing_size":"48–50 / L","shoe_size":"42","height_cm":180,"is_itr":true}`
	if code := call(CreateEmployeeCardHandler, body, ""); code != 200 {
		t.Fatalf("create=%d", code)
	}
	if code := call(UpdateEmployeeCardHandler, body, "1"); code != 200 {
		t.Fatalf("update=%d", code)
	}
	var employees []models.EmployeeCard
	if err := db.Find(&employees).Error; err != nil {
		t.Fatal(err)
	}
	if len(employees) != 2 {
		t.Fatalf("count=%d", len(employees))
	}
	for _, e := range employees {
		if !e.IsITR || e.HeightCM == nil || *e.HeightCM != 180 || e.ClothingSize != "48–50 / L" || e.ShoeSize != "42" {
			t.Fatalf("saved: %+v", e)
		}
	}
	if code := call(UpdateEmployeeCardHandler, `{"name":"Иван","clothing_size":"","shoe_size":"","height_cm":null,"is_itr":false}`, "1"); code != 200 {
		t.Fatalf("clear=%d", code)
	}
	var cleared models.EmployeeCard
	if err := db.First(&cleared, 1).Error; err != nil {
		t.Fatal(err)
	}
	if cleared.IsITR || cleared.HeightCM != nil || cleared.ClothingSize != "" || cleared.ShoeSize != "" {
		t.Fatalf("clear: %+v", cleared)
	}
}

// --------- конец нового кода chat
