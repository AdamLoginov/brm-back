// --------- новый код chat: сохранённая выплата переживает согласование и отмену
package service

import (
	"project/internal/models"
	"testing"
)

func TestSavedPaymentSurvivesApprovalCancellation(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "source_changed"}[changed], func(t *testing.T) {
			db, a, m, e, other := employeeApprovalFixture(t)
			if err := db.Create(&models.SalaryAdvance{AgreementID: a.ID, EmployeeCardID: e.ID, Date: "2026-01-01", AmountKopecks: 10000}).Error; err != nil {
				t.Fatal(err)
			}
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
			s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-01", RequestKey: "save-approve", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), true, "5500.25")}})
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				if err = db.Model(&e).Update("salary_rate", "6000").Error; err != nil {
					t.Fatal(err)
				}
			}
			s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{RequestKey: "save-cancel", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), false, "")}})
			if err != nil {
				t.Fatal(err)
			}
			row := salaryRow(t, s, e.ID)
			if changed {
				if row.HasDraft || row.PaymentKopecks != 590000 {
					t.Fatalf("changed source retained stale draft: %+v", row)
				}
			} else {
				if row.Approved || !row.HasDraft || row.PaymentKopecks != 550025 {
					t.Fatalf("saved payment lost: %+v", row)
				}
				s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
				if err != nil {
					t.Fatal(err)
				}
				row = salaryRow(t, s, e.ID)
				if !row.HasDraft || row.PaymentKopecks != 550025 {
					t.Fatal("reload lost draft")
				}
				s, err = SaveSalaryPaymentDraft(db, a.ID, m.ID, SalaryPaymentDraftInput{EmployeeCardID: e.ID, Revision: row.Revision, Reset: true})
				if err != nil {
					t.Fatal(err)
				}
				row = salaryRow(t, s, e.ID)
				if row.HasDraft || row.PaymentKopecks != 490000 {
					t.Fatal("explicit reset failed")
				}
			}
			if !salaryRow(t, s, other.ID).HasDraft {
				t.Fatal("other employee draft lost")
			}
		})
	}
}

// --------- конец нового кода chat
