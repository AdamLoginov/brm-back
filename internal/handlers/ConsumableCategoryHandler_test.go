// --------- новый код chat: категории, дубликаты и защита используемых категорий
package handlers

import (
	"encoding/json"
	"fmt"
	"project/internal/models"
	"testing"
)

func TestConsumableCategoryLifecycle(t *testing.T) {
	db := consumableTestDB(t)
	w := callConsumable(t, SaveConsumableCategory, "", `{"name":"  Крепёж  "}`, 200)
	var category models.ConsumableCategory
	json.Unmarshal(w.Body.Bytes(), &category)
	if category.Name != "Крепёж" {
		t.Fatal("name normalization")
	}
	callConsumable(t, SaveConsumableCategory, "", `{"name":"КРЕПЁЖ"}`, 409)
	row := models.Consumable{Name: "Болт", Quantity: 1, AddedDate: "2026-11-25", CategoryID: &category.ID}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprint(category.ID)
	callConsumable(t, DeleteConsumableCategory, id, "", 409)
	callConsumable(t, SaveConsumableCategory, id, `{"name":"Метизы"}`, 200)
	var saved models.Consumable
	db.First(&saved, row.ID)
	if saved.CategoryID == nil || *saved.CategoryID != category.ID {
		t.Fatal("rename changed assignment")
	}
	callConsumable(t, SaveConsumableCategory, "", `{"name":"метизы"}`, 409)
	callConsumable(t, DeleteConsumable, fmt.Sprint(row.ID), "", 200)
	callConsumable(t, DeleteConsumableCategory, id, "", 200)
	w = callConsumable(t, ListConsumableCategories, "", "", 200)
	var categories []models.ConsumableCategory
	json.Unmarshal(w.Body.Bytes(), &categories)
	if len(categories) != 0 {
		t.Fatal("deleted category listed")
	}
	callConsumable(t, SaveConsumableCategory, "", `{"name":"Метизы"}`, 200)
	callConsumable(t, SaveConsumableCategory, "", `{"name":""}`, 400)
}

// --------- конец нового кода chat
