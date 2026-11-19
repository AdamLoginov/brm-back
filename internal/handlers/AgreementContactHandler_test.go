// --------- новый код chat: жизненный цикл контактов и границы договора
package handlers

import (
	"encoding/json"
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

func TestAgreementContacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "contacts.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.AgreementContact{}); err != nil {
		t.Fatal(err)
	}
	first, second := models.Agreement{Name: "Первый"}, models.Agreement{Name: "Второй"}
	db.Create(&first)
	db.Create(&second)
	call := func(method string, agreementID, contactID uint, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreementID)))
		req.SetPathValue("contactId", strconv.Itoa(int(contactID)))
		res := httptest.NewRecorder()
		AgreementContactHandler(res, req)
		return res
	}
	body := `{"full_name":"  Иванов   Иван Иванович ","position":"Мастер","phones":[" +7 111 ","+7 222","+7 111",""],"emails":["one@example.ru","two@example.ru","ONE@example.ru"],"comment":"Связаться утром"}`
	res := call("POST", first.ID, 0, body)
	if res.Code != 200 {
		t.Fatalf("create %d: %s", res.Code, res.Body.String())
	}
	var created models.AgreementContact
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var stored models.AgreementContact
	db.First(&stored, created.ID)
	if stored.FullName != "Иванов Иван Иванович" || len(stored.Phones) != 2 || len(stored.Emails) != 2 || stored.Comment != "Связаться утром" {
		t.Fatalf("incorrect persisted data: %+v", stored)
	}
	for _, method := range []string{"PATCH", "DELETE"} {
		if res := call(method, second.ID, created.ID, body); res.Code != 404 {
			t.Fatalf("cross-agreement %s: %d", method, res.Code)
		}
	}
	res = call("GET", second.ID, 0, "")
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatal("contacts leaked")
	}
	res = call("GET", first.ID, 0, "")
	var listed []models.AgreementContact
	if json.Unmarshal(res.Body.Bytes(), &listed) != nil || len(listed) != 1 {
		t.Fatal("list failed")
	}
	for _, invalid := range []string{
		`{"full_name":" "}`, `{"full_name":"Иван","emails":["broken"]}`,
		`{"full_name":"Иван","agreement_id":2}`, `{"full_name":"Иван"} {}`,
		`{"full_name":"Иван","phones":"123"}`,
	} {
		if res := call("POST", first.ID, 0, invalid); res.Code != 400 {
			t.Fatalf("invalid input accepted: %s", invalid)
		}
	}
	if res := call("POST", first.ID+999, 0, body); res.Code != 404 {
		t.Fatal("missing agreement accepted")
	}
	if res := call("PATCH", first.ID, created.ID, `{"full_name":"Петров","position":"","phones":[],"emails":[],"comment":""}`); res.Code != 200 {
		t.Fatalf("clear failed: %s", res.Body.String())
	}
	var updated models.AgreementContact
	db.First(&updated, created.ID)
	if updated.FullName != "Петров" || updated.Position != "" || updated.Comment != "" || len(updated.Phones) != 0 || len(updated.Emails) != 0 {
		t.Fatal("empty values not saved")
	}
	if res := call("DELETE", first.ID, created.ID, ""); res.Code != 204 {
		t.Fatal("delete failed")
	}
	if res := call("PATCH", first.ID, created.ID, body); res.Code != 404 {
		t.Fatal("deleted contact resurrected")
	}
	res = call("GET", first.ID, 0, "")
	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatal("deleted contact listed")
	}
	if res := call("POST", first.ID, 0, `{"full_name":"Только ФИО"}`); res.Code != 200 {
		t.Fatal("optional fields required")
	}
	db.Delete(&first)
	if res := call("GET", first.ID, 0, ""); res.Code != 404 {
		t.Fatal("deleted agreement accessible")
	}
}

// --------- конец нового кода chat
