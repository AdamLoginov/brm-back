// --------- новый код chat: API создания, изменения и статуса премии
package handlers

import (
	"encoding/json"
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

func TestSalaryBonusAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bonus.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.SalaryBonus{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор"}
	db.Create(&agreement)
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	db.Create(&employee)
	call := func(method, id, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreement.ID)))
		req.SetPathValue("bonusId", id)
		res := httptest.NewRecorder()
		SalaryBonusHandler(res, req)
		return res
	}
	input := `{"employee_card_id":` + strconv.Itoa(int(employee.ID)) + `,"month":11,"year":2026,"amount":"500.25","comment":"Премия"}`
	res := call("POST", "", input)
	var row models.SalaryBonus
	if err := json.Unmarshal(res.Body.Bytes(), &row); err != nil {
		t.Fatal(res.Body.String(), err)
	}
	if res.Code != 200 || row.IsPaid || row.AmountKopecks != 50025 {
		t.Fatal(res.Body.String())
	}
	id := strconv.Itoa(int(row.ID))
	for _, body := range []string{`{}`, `{"is_paid":null}`, `{"is_paid":"true"}`, `{"is_paid":true} {}`} {
		if res := call("PATCH", id, body); res.Code != 400 {
			t.Fatal("invalid status", res.Body.String())
		}
	}
	for _, paid := range []bool{true, false} {
		res := call("PATCH", id, `{"is_paid":`+strconv.FormatBool(paid)+`}`)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		db.First(&row, row.ID)
		if row.IsPaid != paid {
			t.Fatal("status not saved")
		}
		if res := call("POST", id, input); res.Code != 200 {
			t.Fatal("edit", res.Body.String())
		}
	}
	if res := call("GET", "", ""); res.Code != 200 {
		t.Fatal("list", res.Body.String())
	}
	if res := call("POST", "", strings.Replace(input, `"amount":"500.25"`, `"amount":"0"`, 1)); res.Code != 400 {
		t.Fatal("zero accepted")
	}
	if res := call("DELETE", id, ""); res.Code != 200 {
		t.Fatal("delete", res.Body.String())
	}
	if res := call("PATCH", id, `{"is_paid":true}`); res.Code != 404 {
		t.Fatal("deleted status")
	}
}

// --------- конец нового кода chat
