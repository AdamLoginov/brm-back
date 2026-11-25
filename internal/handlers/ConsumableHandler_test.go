// --------- новый код chat: валидация, необязательные связи и изоляция договоров
package handlers

import (
	"encoding/json"
	"fmt"
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

func consumableTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "consumables.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.ConsumableCategory{}, &models.Consumable{}); err != nil {
		t.Fatal(err)
	}
	return db
}
func callConsumable(t *testing.T, h http.HandlerFunc, id string, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != want {
		t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
	}
	return w
}
func TestConsumableCRUD(t *testing.T) {
	db := consumableTestDB(t)
	agreement := models.Agreement{Name: "Договор"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	category := models.ConsumableCategory{Name: "Крепёж", NameKey: "крепёж"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	for _, quantity := range []string{"0", "-1", "1.2", "9007199254740992", `"2"`} {
		callConsumable(t, SaveConsumable, "", `{"name":"Болт","added_date":"2026-11-25","quantity":`+quantity+`}`, 400)
	}
	callConsumable(t, SaveConsumable, "", `{"name":"Болт","added_date":"2026-02-30","quantity":1}`, 400)
	callConsumable(t, SaveConsumable, "", `{"name":" ","added_date":"2026-11-25","quantity":1}`, 400)
	callConsumable(t, SaveConsumable, "", `{"name":"Болт","added_date":"2026-11-25","quantity":1,"category_id":999}`, 404)
	callConsumable(t, SaveConsumable, "", `{"name":"Болт","added_date":"2026-11-25","quantity":1,"agreement_id":999}`, 404)
	w := callConsumable(t, SaveConsumable, "", `{"name":"Болт","added_date":"2026-11-25","quantity":2,"agreement_id":null,"category_id":null}`, 200)
	var unassigned models.Consumable
	json.Unmarshal(w.Body.Bytes(), &unassigned)
	if unassigned.ID == 0 || unassigned.AgreementID != nil || unassigned.CategoryID != nil {
		t.Fatal("optional relations")
	}
	body := fmt.Sprintf(`{"name":"Гайка","added_date":"2026-11-24","quantity":12,"agreement_id":%d,"category_id":%d}`, agreement.ID, category.ID)
	w = callConsumable(t, SaveConsumable, "", body, 200)
	var assigned models.Consumable
	json.Unmarshal(w.Body.Bytes(), &assigned)
	w = callConsumable(t, ListConsumables, fmt.Sprint(agreement.ID), "", 200)
	var list []models.Consumable
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != assigned.ID {
		t.Fatal("agreement isolation")
	}
	w = callConsumable(t, ListConsumables, "", "", 200)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatal("global list")
	}
	w = callConsumable(t, SaveConsumable, fmt.Sprint(assigned.ID), `{"name":"Гайка","added_date":"2026-11-23","quantity":5,"agreement_id":null,"category_id":null}`, 200)
	var updated models.Consumable
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Quantity != 5 || updated.AgreementID != nil || updated.CategoryID != nil || updated.AddedDate != "2026-11-23" {
		t.Fatal("update did not clear optional fields")
	}
	callConsumable(t, DeleteConsumable, fmt.Sprint(assigned.ID), "", 200)
	callConsumable(t, DeleteConsumable, fmt.Sprint(assigned.ID), "", 404)
	w = callConsumable(t, ListConsumables, "", "", 200)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatal("deleted item visible")
	}
	callConsumable(t, SaveConsumable, fmt.Sprint(assigned.ID), body, 404)
}

// --------- конец нового кода chat
