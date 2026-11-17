// --------- новый код chat: согласование, снимки, отмена и долги
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func approvalFixture(t *testing.T) (*gorm.DB, models.Agreement, models.TimeSheetMonth, models.EmployeeCard) {
	t.Helper()
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryAdvance{}, &models.SalaryPenalty{}, &models.Expenses{}, &models.SalaryBonus{}, &models.SalaryDebt{}, &models.SalaryApproval{}); err != nil {
		t.Fatal(err)
	}
	a, m, e := salaryFixture(t, db)
	if err := db.Create(&models.TimeSheet{AgreementID: a.ID, TimeSheetMonthID: m.ID, EmployeeCardID: e.ID, Date: "2026-01-10", Status: "12"}).Error; err != nil {
		t.Fatal(err)
	}
	return db, a, m, e
}
func approvalInput(t *testing.T, s SalaryDebtSummary, amount int64, key string) SalaryApprovalInput {
	t.Helper()
	var input SalaryApprovalInput
	body := fmt.Sprintf(`{"date":"2026-02-01","request_key":%q,"revision":%q,"payments":[{"employee_card_id":%d,"amount":"%d.%02d"}]}`, key, s.Revision, s.Accruals[0].EmployeeCardID, amount/100, amount%100)
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatal(err)
	}
	return input
}
func TestSalaryApprovalSnapshotAndCancel(t *testing.T) {
	db, a, m, e := approvalFixture(t)
	advance := models.SalaryAdvance{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2025-12-01", AmountKopecks: 10000}
	db.Create(&advance)
	penalty := models.SalaryPenalty{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-10", AmountKopecks: 20000, Status: "active"}
	db.Create(&penalty)
	expense := models.Expenses{AgreementId: a.ID, EmployeeCardId: e.ID, Account: "employee", Price: 300}
	db.Create(&expense)
	bonus := models.SalaryBonus{AgreementID: a.ID, EmployeeCardID: e.ID, Month: 1, Year: 2026, AmountKopecks: 40000}
	db.Create(&bonus)
	debt := models.SalaryDebt{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2020-01-01", Side: "employee", AmountKopecks: 50000}
	db.Create(&debt)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalPayableKopecks != 490000 {
		t.Fatalf("initial: %+v", s)
	}
	input := approvalInput(t, s, 480000, "approval-test-1")
	saved, err := ApproveSalary(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Approved || saved.TotalPaymentKopecks != 480000 || saved.Accruals[0].NewCompanyDebtKopecks != 10000 {
		t.Fatalf("snapshot: %+v", saved)
	}
	db.First(&advance, advance.ID)
	{
		penaltyID := penalty.ID
		penalty = models.SalaryPenalty{}
		db.First(&penalty, penaltyID)
	}
	db.First(&expense, expense.ID)
	db.First(&bonus, bonus.ID)
	db.First(&debt, debt.ID)
	if !advance.IsSettled || penalty.Status != "closed" || !expense.IsReimbursed || !bonus.IsPaid || !debt.IsClosed {
		t.Fatal("statuses not closed")
	}
	duplicate, err := ApproveSalary(db, a.ID, m.ID, input)
	if err != nil || duplicate.ApprovalID != saved.ApprovalID {
		t.Fatal("duplicate", err)
	}
	var count int64
	db.Model(&models.SalaryDebt{}).Where("source_approval_id = ?", saved.ApprovalID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate debt")
	}
	db.Model(&e).Update("salary_rate", "9999")
	db.Model(&models.TimeSheet{}).Where("time_sheet_month_id = ?", m.ID).Update("status", "1")
	frozen, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.TotalPayableKopecks != 490000 || frozen.Accruals[0].TotalHours != "12.00" || frozen.Accruals[0].Detail.TimeSheets[0].Status != "12" {
		t.Fatal("snapshot changed")
	}
	if err := CancelSalaryApproval(db, a.ID+1, m.ID, saved.ApprovalID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross-contract cancel", err)
	}
	if err := CancelSalaryApproval(db, a.ID, m.ID, saved.ApprovalID); err != nil {
		t.Fatal(err)
	}
	db.First(&advance, advance.ID)
	{
		penaltyID := penalty.ID
		penalty = models.SalaryPenalty{}
		db.First(&penalty, penaltyID)
	}
	db.First(&expense, expense.ID)
	db.First(&bonus, bonus.ID)
	db.First(&debt, debt.ID)
	if advance.IsSettled || penalty.Status != "active" || penalty.ClosedAt != nil || expense.IsReimbursed || bonus.IsPaid || debt.IsClosed {
		t.Fatalf("statuses not restored: advance=%v penalty=%s closed=%v expense=%v bonus=%v debt=%v", advance.IsSettled, penalty.Status, penalty.ClosedAt, expense.IsReimbursed, bonus.IsPaid, debt.IsClosed)
	}
	db.Model(&models.SalaryDebt{}).Where("source_approval_id = ?", saved.ApprovalID).Count(&count)
	if count != 0 {
		t.Fatal("generated debt survived")
	}
	if err := CancelSalaryApproval(db, a.ID, m.ID, saved.ApprovalID); err != nil {
		t.Fatal("repeat cancel", err)
	}
	if _, err := ApproveSalary(db, a.ID, m.ID, input); !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("cancelled request reused", err)
	}
	changed, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil || changed.Approved || changed.TotalPayableKopecks == 490000 {
		t.Fatal("recalculation", err)
	}
}

func TestSalaryApprovalDifferencesAndStale(t *testing.T) {
	for _, tc := range []struct {
		name                                               string
		advance, payment, companyDebt, employeeDebt, bonus int64
	}{
		{"extra", 0, 550000, 0, 0, 50000},
		{"less", 0, 400000, 100000, 0, 0},
		{"negative", 600000, 0, 0, 100000, 0},
		{"exact", 0, 500000, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, a, m, e := approvalFixture(t)
			if tc.advance > 0 {
				db.Create(&models.SalaryAdvance{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-01", AmountKopecks: tc.advance})
			}
			s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ApproveSalary(db, a.ID, m.ID, approvalInput(t, s, tc.payment, "case-"+tc.name))
			if err != nil {
				t.Fatal(err)
			}
			row := result.Accruals[0]
			if row.NewCompanyDebtKopecks != tc.companyDebt || row.NewEmployeeDebtKopecks != tc.employeeDebt || row.ExtraBonusKopecks != tc.bonus {
				t.Fatalf("difference: %+v", row)
			}
			if tc.bonus > 0 {
				var bonus models.SalaryBonus
				db.First(&bonus)
				if !bonus.IsPaid || bonus.Month != m.Month || bonus.AmountKopecks != tc.bonus {
					t.Fatal("extra bonus")
				}
			}
			if tc.companyDebt > 0 {
				// Закрытый последующим расчётом долг не удаляем при отмене источника.
				db.Model(&models.SalaryDebt{}).Where("source_approval_id = ?", result.ApprovalID).Update("is_closed", true)
			}
			if err := CancelSalaryApproval(db, a.ID, m.ID, result.ApprovalID); err != nil {
				t.Fatal(err)
			}
			var count int64
			db.Model(&models.SalaryBonus{}).Count(&count)
			if count != 0 {
				t.Fatal("extra bonus survived")
			}
			db.Model(&models.SalaryDebt{}).Count(&count)
			want := int64(0)
			if tc.companyDebt > 0 {
				want = 1
			}
			if count != want {
				t.Fatal("closed debt rule")
			}
		})
	}
	db, a, m, e := approvalFixture(t)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	db.Create(&models.SalaryDebt{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-01", Side: "company", AmountKopecks: 100})
	if _, err := ApproveSalary(db, a.ID, m.ID, approvalInput(t, s, 500000, "stale-input")); !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("stale accepted", err)
	}
	var count int64
	db.Model(&models.SalaryApproval{}).Count(&count)
	if count != 0 {
		t.Fatal("partial approval")
	}
}

func TestSalaryApprovalBatchRollback(t *testing.T) {
	db, a, m, _ := approvalFixture(t)
	other := models.EmployeeCard{Name: "Другой", AgreementID: a.ID, SalaryRate: "1000"}
	db.Create(&other)
	db.Model(&m).Association("Employees").Append(&other)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	input := approvalInput(t, s, 100, "batch-invalid")
	input.Payments = append(input.Payments, input.Payments[0])
	input.Payments[1].EmployeeCardID = 999999
	if _, err := ApproveSalary(db, a.ID, m.ID, input); !errors.Is(err, ErrSalaryApprovalInput) {
		t.Fatal("bad roster", err)
	}
	for _, model := range []any{&models.SalaryApproval{}, &models.SalaryDebt{}, &models.SalaryBonus{}} {
		var count int64
		db.Model(model).Count(&count)
		if count != 0 {
			t.Fatal("batch left partial changes")
		}
	}
}

// --------- конец нового кода chat
