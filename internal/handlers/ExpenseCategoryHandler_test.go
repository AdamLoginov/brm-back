// --------- новый код chat: общий справочник, уникальность и удаление связей
package handlers

import (
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExpenseCategories(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "categories.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Expenses{}, &models.ExpenseCategory{}); err != nil {
		t.Fatal(err)
	}
	call := func(id string, body string, delete bool) int {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		if delete {
			DeleteExpenseCategory(w, r)
		} else {
			SaveExpenseCategory(w, r)
		}
		return w.Code
	}
	if call("", `{"name":"  Проезд  "}`, false) != 200 {
		t.Fatal("create failed")
	}
	if call("", `{"name":"ПРОЕЗД"}`, false) != 409 {
		t.Fatal("duplicate accepted")
	}
	if call("", `{"name":"   "}`, false) != 400 {
		t.Fatal("empty accepted")
	}
	var category models.ExpenseCategory
	db.First(&category)
	id := strconv.Itoa(int(category.ID))
	if call(id, `{"name":"Транспорт"}`, false) != 200 {
		t.Fatal("rename failed")
	}
	db.First(&category, category.ID)
	if category.Name != "Транспорт" {
		t.Fatal("rename not stored")
	}
	rows := []models.Expenses{
		{AgreementId: 1, Name: "First", Price: 100, CategoryID: &category.ID, IsReimbursed: true},
		{AgreementId: 2, Name: "Second", Price: 200, CategoryID: &category.ID},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	db.Delete(&rows[1])
	if call(id, "", true) != 204 {
		t.Fatal("delete failed")
	}
	var saved []models.Expenses
	db.Unscoped().Order("id").Find(&saved)
	if len(saved) != 2 || saved[0].CategoryID != nil || saved[1].CategoryID != nil || !saved[0].IsReimbursed || saved[0].Price != 100 || !saved[1].DeletedAt.Valid {
		t.Fatal("expenses altered or category retained")
	}
	if call(id, "", true) != 404 {
		t.Fatal("missing category accepted")
	}
	if call("", `{"name":"Транспорт"}`, false) != 200 {
		t.Fatal("deleted name cannot be reused")
	}
	res := httptest.NewRecorder()
	ListExpenseCategories(res, httptest.NewRequest("GET", "/", nil))
	if res.Code != 200 || !strings.Contains(res.Body.String(), "Транспорт") {
		t.Fatal("list failed")
	}
}

// --------- конец нового кода chat
