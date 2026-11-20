// --------- новый код chat: справочники, изоляция договора и запрет удаления используемых связей
package handlers

import (
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

func TestWorkReferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "refs.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous; sql, _ := db.DB(); sql.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.MeasurementUnit{}, &models.AgreementWorkshop{}, &models.AgreementWorkBlock{}, &models.AgreementWorkPosition{}, &models.WorkOrderShift{}, &models.WorkOrderTask{}, &models.WarehouseMaterial{}); err != nil {
		t.Fatal(err)
	}
	a, b := models.Agreement{Name: "A"}, models.Agreement{Name: "B"}
	db.Create(&a)
	db.Create(&b)
	call := func(method, kind string, agreement, id uint, body string) int {
		r := httptest.NewRequest(method, "/", strings.NewReader(body))
		r.SetPathValue("id", strconv.Itoa(int(agreement)))
		r.SetPathValue("kind", kind)
		r.SetPathValue("referenceId", strconv.Itoa(int(id)))
		w := httptest.NewRecorder()
		WorkOrderReferenceHandler(w, r)
		return w.Code
	}
	if call("POST", "units", a.ID, 0, `{"name":"  Метр "}`) != 200 {
		t.Fatal("unit create")
	}
	if call("POST", "units", b.ID, 0, `{"name":"МЕТР"}`) != 409 {
		t.Fatal("global duplicate")
	}
	if call("POST", "workshops", a.ID, 0, `{"name":"Цех"}`) != 200 {
		t.Fatal("shop create")
	}
	var shop models.AgreementWorkshop
	db.First(&shop)
	if call("PATCH", "workshops", b.ID, shop.ID, `{"name":"Чужой"}`) != 404 {
		t.Fatal("foreign reference changed")
	}
	block := models.AgreementWorkBlock{AgreementID: a.ID, WorkshopID: shop.ID, Name: "Блок", NameKey: "блок"}
	db.Create(&block)
	if call("DELETE", "workshops", a.ID, shop.ID, "") != 400 {
		t.Fatal("used workshop deleted")
	}
	position := models.AgreementWorkPosition{AgreementID: a.ID, BlockID: block.ID, Number: "1013", Name: "Тест"}
	db.Create(&position)
	if call("DELETE", "blocks", a.ID, block.ID, "") != 400 {
		t.Fatal("used block deleted")
	}
	var unit models.MeasurementUnit
	db.First(&unit)
	material := models.WarehouseMaterial{AgreementID: a.ID, WorkshopID: shop.ID, UnitID: unit.ID, Name: "Кабель", NameKey: "кабель"}
	db.Create(&material)
	if call("DELETE", "units", a.ID, unit.ID, "") != 400 {
		t.Fatal("used unit deleted")
	}
	task := models.WorkOrderTask{PositionID: position.ID, UnitID: unit.ID}
	db.Create(&task)
	if call("DELETE", "positions", a.ID, position.ID, "") != 400 {
		t.Fatal("used position deleted")
	}
	if call("PATCH", "units", a.ID, unit.ID, `{"name":"м"}`) != 200 {
		t.Fatal("rename used unit denied")
	}
	if call("GET", "", a.ID, 0, "") != 200 {
		t.Fatal("references load")
	}
	// --------- новый код chat: блок без позиций может быть напрямую указан в работе.
	directBlock := models.AgreementWorkBlock{AgreementID: a.ID, WorkshopID: shop.ID, Name: "Самостоятельный", NameKey: "самостоятельный"}
	db.Create(&directBlock)
	db.Create(&models.WorkOrderTask{BlockID: directBlock.ID})
	if call("DELETE", "blocks", a.ID, directBlock.ID, "") != 400 {
		t.Fatal("directly used block deleted")
	}
	otherShop := models.AgreementWorkshop{AgreementID: a.ID, Name: "Второй цех", NameKey: "второй цех"}
	db.Create(&otherShop)
	if call("PATCH", "blocks", a.ID, directBlock.ID, `{"name":"Самостоятельный","workshop_id":`+strconv.Itoa(int(otherShop.ID))+`}`) != 400 {
		t.Fatal("directly used block moved")
	}
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
