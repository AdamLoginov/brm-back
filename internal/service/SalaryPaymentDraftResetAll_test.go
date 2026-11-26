// --------- новый код chat: общий сброс не затрагивает согласованных и другие табели
package service

import (
	"project/internal/models"
	"testing"
)

func TestResetAllSalaryPaymentDrafts(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{e.ID, other.ID} {
		s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: id, Revision: salaryRow(t, s, id).Revision, Amount: "5500.25"})
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "bulk-reset-approve", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), true, "5500.25")}})
	if err != nil {
		t.Fatal(err)
	}
	outside := models.SalaryPaymentDraft{AgreementID: a.ID, TimeSheetMonthID: m.ID + 1, EmployeeCardID: other.ID, AmountKopecks: 12300, Revision: "other-month"}
	if err = db.Create(&outside).Error; err != nil {
		t.Fatal(err)
	}
	s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{ResetAll: true})
	if err != nil {
		t.Fatal(err)
	}
	row := salaryRow(t, s, other.ID)
	if row.HasDraft || row.PaymentKopecks != 300000 {
		t.Fatal("draft was not reset", row)
	}
	row = salaryRow(t, s, e.ID)
	if !row.Approved || row.PaymentKopecks != 550025 {
		t.Fatal("approval changed", row)
	}
	var count int64
	if err = db.Model(&models.SalaryPaymentDraft{}).Where("agreement_id = ? AND time_sheet_month_id = ? AND employee_card_id = ?", a.ID, m.ID, e.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("approved draft lost", err, count)
	}
	if err = db.First(&outside, outside.ID).Error; err != nil {
		t.Fatal("other month draft lost", err)
	}
	s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{ResetAll: true})
	if err != nil || salaryRow(t, s, other.ID).HasDraft {
		t.Fatal("repeat reset failed", err)
	}
}

// --------- конец нового кода chat
