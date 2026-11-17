// --------- новый код chat: сохранение и миграция официального трудоустройства
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

func TestEmployeeEmploymentAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "employees.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	// Имитируем сотрудника, созданного до добавления новых колонок.
	if err := db.Exec("CREATE TABLE employee_cards (id integer PRIMARY KEY, name text, salary_rate text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO employee_cards (id, name, salary_rate) VALUES (1, 'Иван', '3000')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.EmployeeCard{}); err != nil {
		t.Fatal(err)
	}
	var employee models.EmployeeCard
	if err := db.First(&employee, 1).Error; err != nil {
		t.Fatal(err)
	}
	if employee.OfficiallyEmployed || employee.MonthlySalaryKopecks != 0 || employee.SalaryRate != "3000" {
		t.Fatalf("migration: %+v", employee)
	}
	call := func(handler http.HandlerFunc, body string) int {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", "1")
		res := httptest.NewRecorder()
		handler(res, req)
		return res.Code
	}
	for _, tc := range []struct {
		body     string
		code     int
		official bool
		amount   int64
	}{
		{`{"name":"Иван","salary_rate":"3000","officially_employed":true,"monthly_salary_kopecks":1234567}`, 200, true, 1234567},
		{`{"name":"Иван","salary_rate":"3000","officially_employed":true,"monthly_salary_kopecks":0}`, 200, true, 0},
		{`{"name":"Иван","salary_rate":"3000","officially_employed":true,"monthly_salary_kopecks":-1}`, 400, true, 0},
		{`{"name":"Иван","salary_rate":"3000","officially_employed":false,"monthly_salary_kopecks":500}`, 200, false, 0},
	} {
		if code := call(UpdateEmployeeCardHandler, tc.body); code != tc.code {
			t.Fatalf("status=%d want=%d", code, tc.code)
		}
		if err := db.First(&employee, 1).Error; err != nil {
			t.Fatal(err)
		}
		if employee.OfficiallyEmployed != tc.official || employee.MonthlySalaryKopecks != tc.amount || employee.SalaryRate != "3000" {
			t.Fatalf("saved: %+v", employee)
		}
	}
	for _, body := range []string{`{"name":"Анна","officially_employed":"true"}`, `{"name":"Анна","officially_employed":true,"monthly_salary_kopecks":1.5}`, `{"name":"Анна","officially_employed":true,"monthly_salary_kopecks":-1}`} {
		if code := call(CreateEmployeeCardHandler, body); code != 400 {
			t.Fatalf("invalid create status=%d", code)
		}
	}
	if code := call(CreateEmployeeCardHandler, `{"name":"Анна","officially_employed":true,"monthly_salary_kopecks":456789}`); code != 200 {
		t.Fatalf("create status=%d", code)
	}
	var created models.EmployeeCard
	if err := db.Where("name = ?", "Анна").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if !created.OfficiallyEmployed || created.MonthlySalaryKopecks != 456789 {
		t.Fatalf("created: %+v", created)
	}
}

// --------- конец нового кода chat
