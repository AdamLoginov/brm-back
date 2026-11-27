// --------- новый код chat: приоритеты, назначение группы и сохранность документов
package handlers

import (
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"strconv"
	"strings"
	"testing"
)

func TestEmployeeDocumentGroups(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "groups.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.EmployeeDocumentGroup{}, &models.CategoryEmployeeDocument{}, &models.EmployeeDocuments{}); err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, method, id, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", id)
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, body, res.Code, status, res.Body.String())
		}
		return res
	}
	for _, priority := range []string{"0", "-1", "1.5"} {
		call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Личные","priority":`+priority+`}`, 400)
	}
	call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":" ","priority":1}`, 400)
	res := call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Личные","priority":2}`, 200)
	var group models.EmployeeDocumentGroup
	json.Unmarshal(res.Body.Bytes(), &group)
	id := strconv.Itoa(int(group.ID))
	call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Дополнительные","priority":1}`, 200)
	call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Работа","priority":1}`, 200)
	call(EmployeeDocumentGroupsHandler, "PUT", id, `{"name":"Личные новые","priority":1}`, 200)
	call(EmployeeDocumentGroupsHandler, "PUT", id, `{"name":"Личные новые","priority":3}`, 200)
	res = call(EmployeeDocumentGroupsHandler, "GET", "", "", 200)
	var groups []models.EmployeeDocumentGroup
	json.Unmarshal(res.Body.Bytes(), &groups)
	if len(groups) != 3 || groups[0].Priority != 1 || groups[2].Priority != 3 || groups[2].Name != "Личные новые" {
		t.Fatal(groups)
	}
	res = call(CreateCategoryEmployeeDocumentHandler, "POST", "", `{"category_name":"Паспорт","group_id":`+id+`}`, 200)
	var category models.CategoryEmployeeDocument
	json.Unmarshal(res.Body.Bytes(), &category)
	if category.Group == nil || category.Group.ID != group.ID {
		t.Fatal("group missing", res.Body.String())
	}
	cid := strconv.Itoa(int(category.ID))
	call(UpdateEmployeeDocumentCategoryGroupHandler, "PUT", cid, `{"group_id":9999}`, 404)
	call(UpdateEmployeeDocumentCategoryGroupHandler, "PUT", cid, `{"group_id":null}`, 200)
	call(UpdateEmployeeDocumentCategoryGroupHandler, "PUT", cid, `{"group_id":`+id+`}`, 200)
	doc := models.EmployeeDocuments{CategoryEmployeeDocumentID: category.ID, FileName: "passport.pdf"}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatal(err)
	}
	call(EmployeeDocumentGroupsHandler, "DELETE", id, "", 204)
	var remaining models.CategoryEmployeeDocument
	if err := db.First(&remaining, category.ID).Error; err != nil || remaining.GroupID != nil {
		t.Fatal("category lost or still grouped", remaining, err)
	}
	if err := db.First(&models.EmployeeDocuments{}, doc.ID).Error; err != nil {
		t.Fatal("document lost", err)
	}
	call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Новая","priority":3}`, 200)
	call(EmployeeDocumentGroupsHandler, "DELETE", id, "", 404)
	call(middleware.AuthMiddleware(EmployeeDocumentGroupsHandler), "GET", "", "", 401)
	// --------- новый код chat: вставка в середину, перенос в обе стороны и удаление
	assertOrder := func(names ...string) {
		t.Helper()
		var rows []models.EmployeeDocumentGroup
		if err := db.Order("priority").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(names) {
			t.Fatalf("order: %+v", rows)
		}
		for i, row := range rows {
			if row.Priority != i+1 || row.Name != names[i] {
				t.Fatalf("order: %+v, want %v", rows, names)
			}
		}
	}
	assertOrder("Работа", "Дополнительные", "Новая")
	res = call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Начало","priority":1}`, 200)
	var first models.EmployeeDocumentGroup
	json.Unmarshal(res.Body.Bytes(), &first)
	assertOrder("Начало", "Работа", "Дополнительные", "Новая")
	res = call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Середина","priority":3}`, 200)
	var middle models.EmployeeDocumentGroup
	json.Unmarshal(res.Body.Bytes(), &middle)
	assertOrder("Начало", "Работа", "Середина", "Дополнительные", "Новая")
	firstID := strconv.Itoa(int(first.ID))
	call(EmployeeDocumentGroupsHandler, "PUT", firstID, `{"name":"Начало","priority":999}`, 200)
	assertOrder("Работа", "Середина", "Дополнительные", "Новая", "Начало")
	call(EmployeeDocumentGroupsHandler, "PUT", firstID, `{"name":"Начало","priority":1}`, 200)
	assertOrder("Начало", "Работа", "Середина", "Дополнительные", "Новая")
	call(EmployeeDocumentGroupsHandler, "PUT", firstID, `{"name":"Начало","priority":1}`, 200)
	assertOrder("Начало", "Работа", "Середина", "Дополнительные", "Новая")
	call(EmployeeDocumentGroupsHandler, "DELETE", strconv.Itoa(int(middle.ID)), "", 204)
	assertOrder("Начало", "Работа", "Дополнительные", "Новая")
	call(EmployeeDocumentGroupsHandler, "DELETE", firstID, "", 204)
	assertOrder("Работа", "Дополнительные", "Новая")
	// Ошибка в конце перестановки должна откатить и создание, и временные приоритеты.
	if err := db.Exec("CREATE TRIGGER fail_group_order BEFORE UPDATE OF priority ON employee_document_groups WHEN NEW.priority = 1 BEGIN SELECT RAISE(ABORT, 'test rollback'); END").Error; err != nil {
		t.Fatal(err)
	}
	call(EmployeeDocumentGroupsHandler, "POST", "", `{"name":"Не сохранять","priority":1}`, 500)
	assertOrder("Работа", "Дополнительные", "Новая")
	if err := db.Exec("DROP TRIGGER fail_group_order").Error; err != nil {
		t.Fatal(err)
	}
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
