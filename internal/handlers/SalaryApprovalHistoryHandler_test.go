// --------- новый код chat: автор из JWT, защита истории и проверка договора
package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"project/internal/service"
	"strings"
	"testing"
	"time"
)

func TestSalaryApprovalHistoryAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "history.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.Agreement{}, &models.EmployeeCard{}, &models.TimeSheetMonth{}, &models.TimeSheet{}, &models.SalaryAccrual{}, &models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.SalaryAlimony{}, &models.SalaryBonus{}, &models.Expenses{}, &models.SalaryDebt{}, &models.SalaryApproval{}, &models.SalaryPaymentDraft{}, &models.SalaryApprovalBatch{}); err != nil {
		t.Fatal(err)
	}
	a := models.Agreement{Name: "История"}
	db.Create(&a)
	e := models.EmployeeCard{Name: "Иван", AgreementID: a.ID, SalaryRate: "1000"}
	db.Create(&e)
	author := models.User{Login: "operator", Password: "test", EmployeeCardID: e.ID}
	db.Create(&author)
	m := models.TimeSheetMonth{AgreementID: a.ID, Month: 1, Year: 2026, Employees: []models.EmployeeCard{e}}
	db.Create(&m)
	db.Create(&models.TimeSheet{AgreementID: a.ID, TimeSheetMonthID: m.ID, EmployeeCardID: e.ID, Date: "2026-01-01", Status: "12"})
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": author.ID, "exp": time.Now().Add(time.Hour).Unix()}).SignedString(config.JWTKey)
	if err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, method, body, auth string, agreement uint) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		req.SetPathValue("id", fmt.Sprint(agreement))
		req.SetPathValue("monthId", fmt.Sprint(m.ID))
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		res := httptest.NewRecorder()
		middleware.AuthMiddleware(handler)(res, req)
		return res
	}
	if res := call(SalaryApprovalHistoryHandler, "GET", "", "", a.ID); res.Code != 401 {
		t.Fatal("anonymous", res.Code)
	}
	if res := call(SalaryApprovalHistoryHandler, "GET", "", token, a.ID+1); res.Code != 404 {
		t.Fatal("wrong agreement", res.Code)
	}
	s, err := service.GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"date":"2026-02-01","request_key":"history-api-create","employees":[{"employee_card_id":%d,"approved":true,"revision":%q,"amount":"1000"}]}`, e.ID, s.Accruals[0].Revision)
	spoof := strings.TrimSuffix(body, "}") + `,"author_user_id":999}`
	if res := call(SalaryEmployeeApprovalHandler, "POST", spoof, token, a.ID); res.Code != 400 {
		t.Fatal("spoof accepted", res.Code)
	}
	if res := call(SalaryEmployeeApprovalHandler, "POST", body, token, a.ID); res.Code != 200 {
		t.Fatal("approval", res.Code, res.Body.String())
	}
	res := call(SalaryApprovalHistoryHandler, "GET", "", token, a.ID)
	var history []service.SalaryApprovalHistoryGroup
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &history) != nil || len(history) != 1 {
		t.Fatal("history", res.Code, res.Body.String())
	}
	if history[0].AuthorUserID != author.ID || history[0].AuthorName != "operator" || history[0].TotalKopecks != 100000 {
		t.Fatal("wrong author or sum", history)
	}
}

// --------- конец нового кода chat
