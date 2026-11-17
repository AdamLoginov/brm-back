// --------- новый код chat: проверка HTTP согласования, повторов и отмены
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

func TestSalaryApprovalAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "approval.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.SalaryBonus{}, &models.Expenses{}, &models.SalaryDebt{}, &models.SalaryApproval{}, &models.SalaryPaymentDraft{}, &models.SalaryApprovalBatch{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "Тест"}
	db.Create(&a)
	employee := models.EmployeeCard{Name: "Иван", AgreementID: a.ID, SalaryRate: "1000"}
	db.Create(&employee)
	month := models.TimeSheetMonth{AgreementID: a.ID, Month: 1, Year: 2026, Employees: []models.EmployeeCard{employee}}
	db.Create(&month)
	db.Create(&models.TimeSheet{AgreementID: a.ID, EmployeeCardID: employee.ID, TimeSheetMonthID: month.ID, Date: "2026-01-01", Status: "12"})
	call := func(handler http.HandlerFunc, method, body string, agreementID uint) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", strconv.Itoa(int(agreementID)))
		req.SetPathValue("monthId", strconv.Itoa(int(month.ID)))
		res := httptest.NewRecorder()
		handler(res, req)
		return res
	}
	if r := call(middleware.AuthMiddleware(SalaryApprovalHandler), "POST", "{}", a.ID); r.Code != 401 {
		t.Fatal("anonymous")
	}
	for _, body := range []string{`{}`, `{"date":"bad"}`, `{} {}`} {
		if r := call(SalaryApprovalHandler, "POST", body, a.ID); r.Code != 400 {
			t.Fatal(r.Body.String())
		}
	}
	calculated := call(CalculateSalaryAccrualHandler, "POST", "", a.ID)
	var summary service.SalaryDebtSummary
	if err := json.Unmarshal(calculated.Body.Bytes(), &summary); err != nil {
		t.Fatal(calculated.Body.String(), err)
	}
	input := fmt.Sprintf(`{"date":"2026-02-01","request_key":"handler-request-1","revision":%q,"payments":[{"employee_card_id":%d,"amount":"900.00"}]}`, summary.Revision, employee.ID)
	response := call(SalaryApprovalHandler, "POST", input, a.ID)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if !summary.Approved || summary.TotalPaymentKopecks != 90000 {
		t.Fatal("not approved")
	}
	repeat := call(SalaryApprovalHandler, "POST", input, a.ID)
	if repeat.Code != 200 || repeat.Body.String() != response.Body.String() {
		t.Fatal("retry mismatch", repeat.Body.String())
	}
	fetched := call(GetSalaryAccrualHandler, "GET", "", a.ID)
	if fetched.Code != 200 || fetched.Body.String() != response.Body.String() {
		t.Fatal("GET mismatch", fetched.Body.String())
	}
	if r := call(SalaryApprovalHandler, "POST", strings.Replace(input, "handler-request-1", "handler-request-2", 1), a.ID); r.Code != 409 {
		t.Fatal("reapproval accepted")
	}
	cancel := fmt.Sprintf(`{"approval_id":%d}`, summary.ApprovalID)
	if r := call(SalaryApprovalHandler, "DELETE", cancel, a.ID+99); r.Code != 404 {
		t.Fatal("cross-contract cancel")
	}
	if r := call(SalaryApprovalHandler, "DELETE", cancel, a.ID); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := call(SalaryApprovalHandler, "DELETE", cancel, a.ID); r.Code != 200 {
		t.Fatal("repeat cancel")
	}
}

// --------- конец нового кода chat
