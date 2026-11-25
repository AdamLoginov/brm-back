// --------- новый код chat: API черновика, согласования сотрудника и ограничения входа
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
	"project/internal/middleware"
	"project/internal/models"
	"project/internal/service"
	"strconv"
	"strings"
	"testing"
)

func TestSalaryPaymentDraftAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "draft.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	// новый код chat: таблица алиментов в тестовой базе
	if err = db.AutoMigrate(&models.SalaryAlimony{}, &models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.SalaryBonus{}, &models.Expenses{}, &models.SalaryDebt{}, &models.SalaryApproval{}, &models.SalaryPaymentDraft{}, &models.SalaryApprovalBatch{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "Тест"}
	db.Create(&a)
	e := models.EmployeeCard{Name: "Иван", AgreementID: a.ID, SalaryRate: "1000"}
	db.Create(&e)
	m := models.TimeSheetMonth{AgreementID: a.ID, Month: 1, Year: 2026, Employees: []models.EmployeeCard{e}}
	db.Create(&m)
	db.Create(&models.TimeSheet{AgreementID: a.ID, TimeSheetMonthID: m.ID, EmployeeCardID: e.ID, Date: "2026-01-01", Status: "12"})
	call := func(handler http.HandlerFunc, body string, agreement uint) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreement)))
		req.SetPathValue("monthId", strconv.Itoa(int(m.ID)))
		res := httptest.NewRecorder()
		handler(res, req)
		return res
	}
	for _, handler := range []http.HandlerFunc{SalaryPaymentDraftHandler, SalaryEmployeeApprovalHandler} {
		if r := call(middleware.AuthMiddleware(handler), "{}", a.ID); r.Code != 401 {
			t.Fatal("anonymous allowed")
		}
		for _, body := range []string{`{"unknown":1}`, `{} {}`} {
			if r := call(handler, body, a.ID); r.Code != 400 {
				t.Fatal("bad input", r.Code, r.Body.String())
			}
		}
	}
	s, err := service.GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"employee_card_id":%d,"revision":%q,"amount":"900.25"}`, e.ID, s.Accruals[0].Revision)
	if r := call(SalaryPaymentDraftHandler, body, a.ID+1); r.Code != 404 {
		t.Fatal("agreement isolation", r.Code)
	}
	r := call(SalaryPaymentDraftHandler, body, a.ID)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if err = json.Unmarshal(r.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Accruals[0].HasDraft || s.Accruals[0].PaymentKopecks != 90025 {
		t.Fatal("draft response")
	}
	body = fmt.Sprintf(`{"date":"2026-02-01","request_key":"api-employee-approval","employees":[{"employee_card_id":%d,"approved":true,"revision":%q,"amount":"950.25"}]}`, e.ID, s.Accruals[0].Revision)
	r = call(SalaryEmployeeApprovalHandler, body, a.ID)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	repeat := call(SalaryEmployeeApprovalHandler, body, a.ID)
	if repeat.Code != 200 || repeat.Body.String() != r.Body.String() {
		t.Fatal("retry mismatch")
	}
	if err = json.Unmarshal(r.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Accruals[0].Approved || s.Accruals[0].HasDraft || s.Accruals[0].PaymentKopecks != 95025 || s.Accruals[0].NewCompanyDebtKopecks != 4975 {
		t.Fatal("approval response", s)
	}
	if r = call(SalaryPaymentDraftHandler, fmt.Sprintf(`{"employee_card_id":%d,"revision":%q,"amount":"1"}`, e.ID, s.Accruals[0].Revision), a.ID); r.Code != 409 {
		t.Fatal("approved draft editable", r.Code)
	}
}

// --------- конец нового кода chat
