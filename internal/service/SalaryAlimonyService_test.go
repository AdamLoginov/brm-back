// --------- новый код chat: сохранение, удержание и согласование алиментов
package service

import (
	"errors"
	"project/internal/models"
	"testing"
)

func TestSalaryAlimonyLifecycle(t *testing.T) {
	db, a, m, e := approvalFixture(t)
	if err := db.Model(&e).Update("has_alimony", true).Error; err != nil {
		t.Fatal(err)
	}
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	original := salaryRow(t, s, e.ID)
	amount := "100.25"
	s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: original.Revision, Amount: "1000", AlimonyAmount: &amount})
	if err != nil {
		t.Fatal(err)
	}
	row := salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 10025 || row.PayableKopecks != original.PayableKopecks-10025 || row.AmountKopecks != original.AmountKopecks || row.PaymentKopecks != 100000 || !row.HasDraft || s.TotalAlimonyKopecks != 10025 {
		t.Fatalf("incorrect deduction: %+v", row)
	}
	for _, invalid := range []string{"-1", "1.001", "abc", "9999999999999999999999999999"} {
		_, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: row.Revision, Amount: "1000", AlimonyAmount: &invalid})
		if err == nil {
			t.Fatalf("accepted invalid amount %s", invalid)
		}
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 10025 || !row.HasDraft {
		t.Fatal("save did not persist")
	}
	// новый код chat: ошибка выплаты откатывает изменение алиментов целиком
	changed := "200"
	_, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: row.Revision, Amount: "invalid", AlimonyAmount: &changed})
	if err == nil {
		t.Fatal("invalid payment accepted")
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 10025 || row.PaymentKopecks != 100000 {
		t.Fatal("partial save", row)
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "alimony-approve-1", Employees: []SalaryEmployeeSelection{employeeSelection(row, true, "1000")}})
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, s, e.ID)
	_, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: row.Revision, Amount: "1000", AlimonyAmount: &amount})
	if !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("approved changed", err)
	}
	if err = db.Model(&e).Update("has_alimony", false).Error; err != nil {
		t.Fatal(err)
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 10025 {
		t.Fatal("snapshot changed")
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "alimony-cancel-1", Employees: []SalaryEmployeeSelection{employeeSelection(row, false, "1000")}})
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 0 || row.PayableKopecks != original.PayableKopecks {
		t.Fatal("disabled deduction retained", row)
	}
	if err = db.Model(&e).Update("has_alimony", true).Error; err != nil {
		t.Fatal(err)
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).AlimonyKopecks != 0 {
		t.Fatal("old amount resurrected")
	}
	var count int64
	if err = db.Model(&models.SalaryAlimony{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("duplicate alimony", count, err)
	}
}

// --------- конец нового кода chat
