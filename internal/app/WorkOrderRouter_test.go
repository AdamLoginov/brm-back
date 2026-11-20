// --------- новый код chat: реальные маршруты книги и склада, включая авторизацию
package app

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"strings"
	"testing"
	"time"
)

func TestWorkOrderRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "work-api.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old, key := database.DB, config.JWTKey
	database.DB = db
	config.JWTKey = []byte("work-order-test-key")
	t.Cleanup(func() { database.DB = old; config.JWTKey = key; sql, _ := db.DB(); sql.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.MeasurementUnit{}, &models.AgreementWorkshop{}, &models.AgreementWorkBlock{}, &models.AgreementWorkPosition{}, &models.WorkOrderBook{}, &models.WorkOrderDay{}, &models.WorkOrderShift{}, &models.WorkOrderEmployee{}, &models.WorkOrderTask{}, &models.WarehouseMaterial{}, &models.WarehouseReceipt{}, &models.WorkOrderMaterial{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "A"}
	db.Create(&a)
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString(config.JWTKey)
	router := NewRouter()
	base := fmt.Sprintf("/api/agreements/%d", a.ID)
	call := func(method, path, body string, auth bool) int {
		r := httptest.NewRequest(method, base+path, strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code >= 500 {
			t.Log(w.Body.String())
		}
		return w.Code
	}
	for _, path := range []string{"/work-references", "/work-orders", "/warehouse"} {
		if call("GET", path, "", false) != 401 {
			t.Fatal("anonymous access", path)
		}
		if call("GET", path, "", true) != 200 {
			t.Fatal("read failed", path)
		}
	}
	if call("POST", "/work-orders/books/create", `{"year":2028,"month":2}`, true) != 200 {
		t.Fatal("create book route")
	}
	var book models.WorkOrderBook
	db.First(&book)
	if call("GET", fmt.Sprintf("/work-orders/books/%d", book.ID), "", true) != 200 {
		t.Fatal("book detail route")
	}
	if call("POST", "/work-references/workshops/create", `{"name":"Цех"}`, true) != 200 {
		t.Fatal("create shop route")
	}
	var shop models.AgreementWorkshop
	db.First(&shop)
	var day models.WorkOrderDay
	db.First(&day)
	body := fmt.Sprintf(`{"day_id":%d,"workshop_id":%d,"number":1,"version":0,"employees":[],"tasks":[]}`, day.ID, shop.ID)
	if call("POST", "/work-orders/shifts/create", body, true) != 200 {
		t.Fatal("shift route")
	}
	var shift models.WorkOrderShift
	db.First(&shift)
	if call("DELETE", fmt.Sprintf("/work-orders/shifts/%d?version=%d", shift.ID, shift.Version), "", true) != 204 {
		t.Fatal("delete route")
	}
	// новый код chat: сквозной API поступления, истории и изменения склада
	if call("POST", "/work-references/units/create", `{"name":"м"}`, true) != 200 {
		t.Fatal("unit route")
	}
	var unit models.MeasurementUnit
	db.First(&unit)
	materialBody := fmt.Sprintf(`{"name":"Кабель","workshop_id":%d,"unit_id":%d}`, shop.ID, unit.ID)
	if call("POST", "/warehouse/create", materialBody, true) != 200 {
		t.Fatal("material route")
	}
	var material models.WarehouseMaterial
	db.First(&material)
	receiptPath := fmt.Sprintf("/warehouse/%d/receipts", material.ID)
	if call("POST", receiptPath+"/create", `{"date":"2028-02-01","quantity":"1.125","remove_usage_ids":[]}`, true) != 200 {
		t.Fatal("receipt create route")
	}
	var receipt models.WarehouseReceipt
	db.First(&receipt)
	if receipt.QuantityThousandths != 1125 {
		t.Fatal("receipt precision")
	}
	if call("GET", fmt.Sprintf("/warehouse/%d/history", material.ID), "", true) != 200 {
		t.Fatal("history route")
	}
	if call("PATCH", fmt.Sprintf("%s/%d", receiptPath, receipt.ID), `{"date":"2028-02-02","quantity":"2.25","remove_usage_ids":[]}`, true) != 200 {
		t.Fatal("receipt edit route")
	}
	if call("DELETE", fmt.Sprintf("%s/%d", receiptPath, receipt.ID), `{"remove_usage_ids":[]}`, true) != 200 {
		t.Fatal("receipt delete route")
	}
	if call("DELETE", fmt.Sprintf("/warehouse/%d", material.ID), "", true) != 204 {
		t.Fatal("material delete route")
	}
}

// --------- конец нового кода chat
