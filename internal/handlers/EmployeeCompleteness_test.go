// новый код chat: документы добавляются к общему списку только для сводной проверки
package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEmployeeListOptionalDocuments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "completeness.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.EmployeeCard{}, &models.EmployeeDocuments{}, &models.CategoryEmployeeDocument{}); err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Проверка"}
	category := models.CategoryEmployeeDocument{CategoryName: "Паспорт"}
	for _, value := range []interface{}{&employee, &category} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	document := models.EmployeeDocuments{EmployeeCardID: employee.ID, CategoryEmployeeDocumentID: category.ID, FileName: "passport.pdf"}
	if err := db.Create(&document).Error; err != nil {
		t.Fatal(err)
	}
	deleted := models.EmployeeDocuments{EmployeeCardID: employee.ID, CategoryEmployeeDocumentID: category.ID, FileName: "deleted.pdf"}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		path := "/api/employeecards"
		if include {
			path += "?include_documents=true"
		}
		response := httptest.NewRecorder()
		GetAllEmployeeCardHandler(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
		var rows []models.EmployeeCard
		if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows=%d", len(rows))
		}
		if !include && len(rows[0].EmployeeDocuments) != 0 {
			t.Fatal("regular list unexpectedly includes documents")
		}
		if include && (len(rows[0].EmployeeDocuments) != 1 || rows[0].EmployeeDocuments[0].CategoryEmployeeDocument.CategoryName != "Паспорт") {
			t.Fatal("documents or category missing; deleted documents must be excluded")
		}
	}
}
