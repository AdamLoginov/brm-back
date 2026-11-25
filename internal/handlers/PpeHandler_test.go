// --------- новый код chat: цена до копеек, фильтрация договора и снятие закрепления
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

func TestPpeCRUDAndTransfer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ppe.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Ppe{}, &models.Tool{}); err != nil {
		t.Fatal(err)
	}
	a, b := models.Agreement{Name: "A"}, models.Agreement{Name: "B"}
	db.Create(&a)
	db.Create(&b)
	employee := models.EmployeeCard{Name: "Иван", AgreementID: b.ID}
	db.Create(&employee)
	call := func(handler http.HandlerFunc, id string, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != want {
			t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
		}
		return w
	}
	payload := func(price, category string, agreement, assignee uint) string {
		return fmt.Sprintf(`{"name":"Куртка","size":" 48–50 ","category":%q,"price":%q,"agreement_id":%d,"employee_card_id":%d}`, category, price, agreement, assignee)
	}
	for _, price := range []string{"-1", "1.234", "abc", "", "90071992547409.92"} {
		call(SavePpe, "", payload(price, "winter", a.ID, employee.ID), 400)
	}
	call(SavePpe, "", payload("0", "invalid", a.ID, employee.ID), 400)
	call(SavePpe, "", payload("0", "winter", 0, employee.ID), 400)
	call(SavePpe, "", payload("0", "winter", a.ID, 99999), 404)
	w := call(SavePpe, "", payload("1234,56", "winter", a.ID, employee.ID), 200)
	var row models.Ppe
	json.Unmarshal(w.Body.Bytes(), &row)
	if row.PriceKopecks != 123456 || row.Size != "48–50" || row.EmployeeCardID == nil {
		t.Fatalf("saved: %+v", row)
	}
	call(SavePpe, "", payload("0", "summer", b.ID, 0), 200)
	w = call(ListPpe, fmt.Sprint(a.ID), "", 200)
	var rows []models.Ppe
	json.Unmarshal(w.Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0].EmployeeCard == nil || rows[0].EmployeeCard.Name != "Иван" {
		t.Fatal("agreement list or employee preload")
	}
	w = call(ListPpe, "", "", 200)
	json.Unmarshal(w.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Fatal("global list")
	}
	call(PutAgreementEmployeeCard, fmt.Sprint(employee.ID), fmt.Sprintf(`{"agreement_id":%d}`, b.ID), 200)
	var same models.Ppe
	db.First(&same, row.ID)
	if same.EmployeeCardID == nil {
		t.Fatal("unchanged agreement detached PPE")
	}
	call(PutAgreementEmployeeCard, fmt.Sprint(employee.ID), fmt.Sprintf(`{"agreement_id":%d}`, a.ID), 200)
	var detached models.Ppe
	db.First(&detached, row.ID)
	if detached.EmployeeCardID != nil {
		t.Fatal("transfer kept PPE")
	}
	call(SavePpe, fmt.Sprint(row.ID), payload("0", "demiseason", a.ID, 0), 200)
	var updated models.Ppe
	db.First(&updated, row.ID)
	if updated.PriceKopecks != 0 || updated.Category != "demiseason" || updated.EmployeeCardID != nil {
		t.Fatal("update")
	}
	db.Model(&employee).Update("status", true)
	call(SavePpe, fmt.Sprint(row.ID), payload("1", "winter", a.ID, employee.ID), 400)
	call(DeletePpe, fmt.Sprint(row.ID), "", 200)
	call(DeletePpe, fmt.Sprint(row.ID), "", 404)
	call(SavePpe, fmt.Sprint(row.ID), payload("1", "winter", a.ID, 0), 404)
	w = call(ListPpe, "", "", 200)
	json.Unmarshal(w.Body.Bytes(), &rows)
	if len(rows) != 1 {
		t.Fatal("deleted PPE remains visible")
	}
}

// --------- конец нового кода chat
