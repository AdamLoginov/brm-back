// --------- новый код chat: API создания, изменения и статуса долга
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

func TestSalaryDebtAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "debt.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.SalaryDebt{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор"}
	db.Create(&agreement)
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	db.Create(&employee)
	call := func(method, id, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreement.ID)))
		req.SetPathValue("debtId", id)
		res := httptest.NewRecorder()
		SalaryDebtHandler(res, req)
		return res
	}
	input := `{"employee_card_id":` + strconv.Itoa(int(employee.ID)) + `,"date":"2026-01-15","side":"company","amount":"500.25","comment":"Премия"}`
	res := call("POST", "", input)
	var row models.SalaryDebt
	if err := json.Unmarshal(res.Body.Bytes(), &row); err != nil {
		t.Fatal(res.Body.String(), err)
	}
	if res.Code != 200 || row.IsClosed || row.AmountKopecks != 50025 {
		t.Fatal(res.Body.String())
	}
	id := strconv.Itoa(int(row.ID))
	for _, body := range []string{`{}`, `{"is_closed":null}`, `{"is_closed":"true"}`, `{"is_closed":true} {}`} {
		if res := call("PATCH", id, body); res.Code != 400 {
			t.Fatal("invalid status", res.Body.String())
		}
	}
	for _, paid := range []bool{true, false} {
		res := call("PATCH", id, `{"is_closed":`+strconv.FormatBool(paid)+`}`)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		db.First(&row, row.ID)
		if row.IsClosed != paid {
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
	if res := call("PATCH", id, `{"is_closed":true}`); res.Code != 404 {
		t.Fatal("deleted status")
	}
}

// --------- конец нового кода chat
