// --------- новый код chat: обновление материала не меняет единицу/цех после движения
package handlers

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"testing"
)

func TestWarehouseMaterialAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "warehouse.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sql, _ := db.DB(); sql.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.MeasurementUnit{}, &models.AgreementWorkshop{}, &models.WarehouseMaterial{}, &models.WarehouseReceipt{}, &models.WorkOrderMaterial{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "A"}
	db.Create(&a)
	unit := models.MeasurementUnit{Name: "м", NameKey: "м"}
	db.Create(&unit)
	unit2 := models.MeasurementUnit{Name: "кг", NameKey: "кг"}
	db.Create(&unit2)
	shop := models.AgreementWorkshop{AgreementID: a.ID, Name: "Цех", NameKey: "цех"}
	db.Create(&shop)
	call := func(method, id, body string) int {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", strconv.Itoa(int(a.ID)))
		r.SetPathValue("materialId", id)
		w := httptest.NewRecorder()
		WarehouseHandler(w, r)
		return w.Code
	}
	body := fmt.Sprintf(`{"name":"Материал","workshop_id":%d,"unit_id":%d}`, shop.ID, unit.ID)
	if call("POST", "", body) != 200 {
		t.Fatal("create failed")
	}
	var m models.WarehouseMaterial
	db.First(&m)
	id := strconv.Itoa(int(m.ID))
	db.Create(&models.WarehouseReceipt{MaterialID: m.ID, QuantityThousandths: 1000, Date: "2028-02-01"})
	changed := fmt.Sprintf(`{"name":"Материал","workshop_id":%d,"unit_id":%d}`, shop.ID, unit2.ID)
	if call("PATCH", id, changed) != 400 {
		t.Fatal("unit changed after receipt")
	}
	if call("PATCH", id, strings.Replace(body, "Материал", "Новое имя", 1)) != 200 {
		t.Fatal("rename failed")
	}
	if call("PATCH", "invalid", body) != 400 {
		t.Fatal("invalid ID accepted")
	}
	if call("GET", "", "") != 200 {
		t.Fatal("stock list failed")
	}
}

// --------- конец нового кода chat
