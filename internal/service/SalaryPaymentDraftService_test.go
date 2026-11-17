// --------- новый код chat: черновики, индивидуальное согласование и атомарная отмена
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func employeeApprovalFixture(t *testing.T) (*gorm.DB, models.Agreement, models.TimeSheetMonth, models.EmployeeCard, models.EmployeeCard) {
	db, a, m, e := approvalFixture(t)
	other := models.EmployeeCard{Name: "Другой", AgreementID: a.ID, SalaryRate: "3000"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&m).Association("Employees").Append(&other); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TimeSheet{AgreementID: a.ID, TimeSheetMonthID: m.ID, EmployeeCardID: other.ID, Date: "2026-01-10", Status: "12"}).Error; err != nil {
		t.Fatal(err)
	}
	return db, a, m, e, other
}
func salaryRow(t *testing.T, s SalaryDebtSummary, id uint) SalaryAccrualWithDebts {
	t.Helper()
	for _, r := range s.Accruals {
		if r.EmployeeCardID == id {
			return r
		}
	}
	t.Fatal("missing employee", id)
	return SalaryAccrualWithDebts{}
}
func employeeSelection(r SalaryAccrualWithDebts, approved bool, amount string) SalaryEmployeeSelection {
	return SalaryEmployeeSelection{EmployeeCardID: r.EmployeeCardID, Approved: approved, ApprovalID: r.ApprovalID, Revision: r.Revision, Amount: amount}
}
func TestPaymentDraftPersistenceResetAndIsolation(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{e.ID, other.ID} {
		r := salaryRow(t, s, id)
		s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: id, Revision: r.Revision, Amount: "5500.25"})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if r := salaryRow(t, s, e.ID); !r.HasDraft || r.PaymentKopecks != 550025 {
		t.Fatal("draft lost", r)
	}
	old := salaryRow(t, s, e.ID)
	if err = db.Model(&e).Update("salary_rate", "6000").Error; err != nil {
		t.Fatal(err)
	}
	// Устаревшую сумму нельзя сохранить или согласовать.
	_, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: old.Revision, Amount: "10"})
	if !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("stale accepted", err)
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	r := salaryRow(t, s, e.ID)
	if r.HasDraft || !r.DraftReset || r.PaymentKopecks != 600000 {
		t.Fatal("not reset", r)
	}
	if !salaryRow(t, s, other.ID).HasDraft {
		t.Fatal("other draft reset")
	}
	r = salaryRow(t, s, other.ID)
	s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: other.ID, Revision: r.Revision, Reset: true})
	if err != nil {
		t.Fatal(err)
	}
	if r = salaryRow(t, s, other.ID); r.HasDraft || r.PaymentKopecks != 300000 {
		t.Fatal("manual reset", r)
	}
	for _, amount := range []string{"-1", "1.001", "999999999999999999999999"} {
		_, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: other.ID, Revision: r.Revision, Amount: amount})
		if !errors.Is(err, ErrSalaryApprovalInput) {
			t.Fatal("invalid accepted", amount, err)
		}
	}
}
func TestEmployeeApprovalAndAtomicSelection(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	advance := models.SalaryAdvance{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-01", AmountKopecks: 10000}
	db.Create(&advance)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	r := salaryRow(t, s, e.ID)
	input := SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "individual-first", Employees: []SalaryEmployeeSelection{employeeSelection(r, true, "4800")}}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	r = salaryRow(t, s, e.ID)
	if !r.Approved || r.NewCompanyDebtKopecks != 10000 || salaryRow(t, s, other.ID).Approved {
		t.Fatal("isolation", s)
	}
	db.First(&advance, advance.ID)
	if !advance.IsSettled {
		t.Fatal("advance open")
	}
	_, err = ApproveSalary(db, a.ID, m.ID, approvalInput(t, s, 0, "legacy-blocked"))
	if !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("legacy can overwrite", err)
	}
	// Согласованный снимок переживает изменение исходной ставки.
	db.Model(&e).Update("salary_rate", "7000")
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).AmountKopecks != 500000 {
		t.Fatal("snapshot changed")
	}
	// Ошибка второй строки откатывает всю операцию.
	first := employeeSelection(salaryRow(t, s, e.ID), false, "")
	second := employeeSelection(salaryRow(t, s, other.ID), true, "-1")
	_, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "invalid-selection", Employees: []SalaryEmployeeSelection{first, second}})
	if !errors.Is(err, ErrSalaryApprovalInput) {
		t.Fatal(err)
	}
	db.First(&advance, advance.ID)
	if !advance.IsSettled {
		t.Fatal("partial cancellation")
	}
	second.Amount = "3500"
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "mixed-selection", Employees: []SalaryEmployeeSelection{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).Approved || !salaryRow(t, s, other.ID).Approved || salaryRow(t, s, other.ID).ExtraBonusKopecks != 50000 {
		t.Fatal("mixed states", s)
	}
	advance = models.SalaryAdvance{Model: gorm.Model{ID: advance.ID}}
	db.First(&advance)
	if advance.IsSettled {
		t.Fatal("advance not restored")
	}
	var debts int64
	db.Model(&models.SalaryDebt{}).Where("employee_card_id = ?", e.ID).Count(&debts)
	if debts != 0 {
		t.Fatal("created debt not deleted")
	}
	if s.TotalKopecks != 1000000 || s.TotalPaymentKopecks != 1040000 {
		t.Fatal("mixed totals", s.TotalKopecks, s.TotalPaymentKopecks)
	}
}
func TestLegacyApprovalSplitAndIndividualCancel(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	input := SalaryApprovalInput{Date: "2026-02-01", RequestKey: "old-whole-month", Revision: s.Revision}
	for _, r := range s.Accruals {
		input.Payments = append(input.Payments, struct {
			EmployeeCardID uint   `json:"employee_card_id"`
			Amount         string `json:"amount"`
		}{r.EmployeeCardID, fmt.Sprintf("%d", r.PayableKopecks/100-100)})
	}
	s, err = ApproveSalary(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{RequestKey: "split-and-cancel", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), false, "")}})
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).Approved || !salaryRow(t, s, other.ID).Approved {
		t.Fatal("legacy isolation")
	}
	var debt []models.SalaryDebt
	db.Find(&debt)
	if len(debt) != 1 || debt[0].EmployeeCardID != other.ID {
		t.Fatal("legacy journal split", debt)
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{RequestKey: "cancel-remainder", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, other.ID), false, "")}})
	if err != nil {
		t.Fatal(err)
	}
	db.Find(&debt)
	var count int64
	db.Model(&models.SalaryDebt{}).Count(&count)
	if count != 0 {
		t.Fatal("remaining debt")
	}
}

// --------- конец нового кода chat

// --------- новый код chat: повтор запроса и ошибка после начала финансовых изменений
func TestEmployeeApprovalRetryAndRollback(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	advance := models.SalaryAdvance{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-01", AmountKopecks: 10000}
	db.Create(&advance)
	current, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	input := SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "atomic-with-retry", Employees: []SalaryEmployeeSelection{
		employeeSelection(salaryRow(t, current, e.ID), true, "4800"), employeeSelection(salaryRow(t, current, other.ID), true, "3500"),
	}}
	failure := errors.New("test: bonus write failed")
	if err = db.Callback().Create().Before("gorm:create").Register("test_fail_bonus", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "salary_bonus" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	db.Callback().Create().Remove("test_fail_bonus")
	if !errors.Is(err, failure) {
		t.Fatal("expected write failure", err)
	}
	for _, model := range []any{&models.SalaryApproval{}, &models.SalaryApprovalBatch{}, &models.SalaryDebt{}} {
		var count int64
		db.Model(model).Count(&count)
		if count != 0 {
			t.Fatal("partial transaction", model, count)
		}
	}
	db.First(&advance, advance.ID)
	if advance.IsSettled {
		t.Fatal("partial advance closure")
	}
	saved, err := ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if salaryHash(saved) != salaryHash(repeated) {
		t.Fatal("retry changed result")
	}
	var count int64
	db.Model(&models.SalaryApproval{}).Count(&count)
	if count != 2 {
		t.Fatal("duplicate approvals", count)
	}
	input.Employees[0].Amount = "9999"
	_, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	if !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("key reused for other payload", err)
	}
	// Выбор всех и снятие всех поддерживаются тем же API; отмена также идемпотентна.
	cancel := SalaryEmployeeApprovalInput{RequestKey: "cancel-all-retry"}
	for _, r := range saved.Accruals {
		cancel.Employees = append(cancel.Employees, employeeSelection(r, false, ""))
	}
	cleared, err := ApplySalaryEmployeeApprovals(db, a.ID, m.ID, cancel)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ApplySalaryEmployeeApprovals(db, a.ID, m.ID, cancel)
	if err != nil || salaryHash(cleared) != salaryHash(again) {
		t.Fatal("cancel retry", err)
	}
	for _, r := range cleared.Accruals {
		if r.Approved {
			t.Fatal("not cleared")
		}
	}
}

// --------- конец нового кода chat
