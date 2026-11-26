// --------- новый код chat: API состава табеля проверяет авторизацию, договор и явный пустой список
package handlers

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"strings"
	"testing"
)

func TestTimeSheetEmployeeListAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "list.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err = db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.DailyAllowanceAccrual{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "Табель"}
	db.Create(&a)
	e := models.EmployeeCard{Name: "Сотрудник", AgreementID: a.ID}
	db.Create(&e)
	m := models.TimeSheetMonth{AgreementID: a.ID, Month: 1, Year: 2026, Employees: []models.EmployeeCard{e}}
	db.Create(&m)
	call := func(method, body string, agreement uint, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", fmt.Sprint(agreement))
		req.SetPathValue("monthId", fmt.Sprint(m.ID))
		res := httptest.NewRecorder()
		if auth {
			middleware.AuthMiddleware(TimeSheetEmployeeListHandler)(res, req)
		} else {
			TimeSheetEmployeeListHandler(res, req)
		}
		return res
	}
	if res := call("GET", "", a.ID, true); res.Code != 401 {
		t.Fatal("anonymous", res.Code)
	}
	if res := call("GET", "", a.ID+1, false); res.Code != 404 {
		t.Fatal("scope", res.Code)
	}
	for _, body := range []string{`{}`, `{"enabled_ids":null}`, `{"enabled_ids":[],"unknown":true}`, `{"enabled_ids":[]} {}`} {
		if res := call("PUT", body, a.ID, false); res.Code != 400 {
			t.Fatal("invalid", body, res.Code)
		}
	}
	if res := call("PUT", `{"enabled_ids":[]}`, a.ID, false); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	var saved models.TimeSheetMonth
	db.Preload("Employees").First(&saved, m.ID)
	if len(saved.Employees) != 1 || len(saved.HiddenEmployeeIDs) != 1 {
		t.Fatal("membership deleted")
	}
	if res := call("GET", "", a.ID, false); res.Code != 200 || !strings.Contains(res.Body.String(), "Сотрудник") {
		t.Fatal("hidden missing in editor")
	}
}

// --------- конец нового кода chat
