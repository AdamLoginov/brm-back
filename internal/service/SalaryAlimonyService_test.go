// --------- новый код chat: алименты сотрудника между договорами, согласование и отмена
package service

import (
	"errors"
	"fmt"
	"project/internal/models"
	"testing"
)

func TestEmployeeAlimonyAcrossAgreements(t *testing.T) {
	db, a, m, e := approvalFixture(t)
	if err := db.Model(&e).Update("has_alimony", true).Error; err != nil {
		t.Fatal(err)
	}
	b := models.Agreement{Name: "Другой договор"}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	n := models.TimeSheetMonth{AgreementID: b.ID, Month: m.Month, Year: m.Year, ShiftHours: m.ShiftHours, Employees: []models.EmployeeCard{e}}
	if err := db.Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.TimeSheet{AgreementID: b.ID, TimeSheetMonthID: n.ID, EmployeeCardID: e.ID, Date: "2026-01-10", Status: "12"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"0", "-1", "1.001", "bad", "999999999999999999999"} {
		if _, err := SaveSalaryAlimony(db, a.ID, 0, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2026-01-01", Amount: bad}); err == nil {
			t.Fatal("invalid amount accepted", bad)
		}
	}
	record, err := SaveSalaryAlimony(db, a.ID, 0, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2025-12-01", Amount: "100.25", Comment: "Комментарий"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := SaveSalaryAlimony(db, b.ID, 0, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2026-02-01", Amount: "50"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := GetSalaryAlimonies(db, b.ID)
	if err != nil || len(list) != 2 {
		t.Fatal("shared list", list, err)
	}
	// После перевода история первого договора доступна через его табель.
	if err = db.Model(&e).Updates(map[string]any{"agreement_id": b.ID, "has_alimony": false}).Error; err != nil {
		t.Fatal(err)
	}
	list, err = GetSalaryAlimonies(db, a.ID)
	if err != nil || len(list) != 2 {
		t.Fatal("history lost", list, err)
	}
	if _, err = SaveSalaryAlimony(db, b.ID, 0, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2026-01-01", Amount: "1"}); !errors.Is(err, ErrSalaryAlimonyInput) {
		t.Fatal("disabled employee accepted", err)
	}
	people, err := GetSalaryAlimonyEmployees(db, b.ID)
	if err != nil || len(people) != 0 {
		t.Fatal("disabled employee selectable", err)
	}
	_, err = SaveSalaryAlimony(db, b.ID, record.ID, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2025-12-01", Amount: "100.25", Comment: "Исправлено"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	row := salaryRow(t, s, e.ID)
	if row.AlimonyKopecks != 15025 || len(row.Detail.Alimonies) != 2 || row.PayableKopecks != row.AmountKopecks-15025 {
		t.Fatal("wrong deduction", row)
	}
	other, err := GetSalaryCalculation(db, b.ID, n.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	stale := salaryRow(t, other, e.ID)
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-03", RequestKey: "alimony-close-first", Employees: []SalaryEmployeeSelection{employeeSelection(row, true, fmt.Sprintf("%.2f", float64(row.PayableKopecks)/100))}})
	if err != nil {
		t.Fatal(err)
	}
	approved := salaryRow(t, s, e.ID)
	if approved.AlimonyKopecks != 15025 {
		t.Fatal("snapshot wrong")
	}
	var stored models.SalaryAlimony
	if err = db.First(&stored, record.ID).Error; err != nil || !stored.IsSettled || stored.ClosedAt == nil || stored.SettlementApprovalID == nil {
		t.Fatal("not closed", stored, err)
	}
	_, err = ApplySalaryEmployeeApprovals(db, b.ID, n.ID, SalaryEmployeeApprovalInput{Date: "2026-02-03", RequestKey: "alimony-stale-second", Employees: []SalaryEmployeeSelection{employeeSelection(stale, true, "1000")}})
	if !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("double deduction accepted", err)
	}
	other, err = GetSalaryCalculation(db, b.ID, n.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, other, e.ID).AlimonyKopecks != 0 {
		t.Fatal("closed deducted again")
	}
	// Закрытые записи можно редактировать и удалять, снимок согласования остаётся прежним.
	_, err = SaveSalaryAlimony(db, b.ID, record.ID, SalaryAlimonyInput{EmployeeCardID: e.ID, Date: "2025-12-01", Amount: "200", Comment: "Изменено"})
	if err != nil {
		t.Fatal(err)
	}
	if err = DeleteSalaryAlimony(db, b.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).AlimonyKopecks != 15025 {
		t.Fatal("snapshot changed by editing")
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{Date: "2026-02-03", RequestKey: "alimony-cancel-first", Employees: []SalaryEmployeeSelection{employeeSelection(approved, false, "0")}})
	if err != nil {
		t.Fatal(err)
	}
	if salaryRow(t, s, e.ID).AlimonyKopecks != 20000 {
		t.Fatal("cancel failed to restore", s)
	}
	stored = models.SalaryAlimony{} // новый код chat: читаем nullable-поля в чистую структуру
	if err = db.First(&stored, record.ID).Error; err != nil || stored.IsSettled || stored.ClosedAt != nil || stored.SettlementApprovalID != nil {
		t.Fatal("closure not cleared", stored, err)
	}
	// Алименты снова доступны другому договору; повторная отмена не затрагивает новое закрытие.
	other, err = GetSalaryCalculation(db, b.ID, n.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	row = salaryRow(t, other, e.ID)
	_, err = ApplySalaryEmployeeApprovals(db, b.ID, n.ID, SalaryEmployeeApprovalInput{Date: "2026-02-03", RequestKey: "alimony-close-second", Employees: []SalaryEmployeeSelection{employeeSelection(row, true, fmt.Sprintf("%.2f", float64(row.PayableKopecks)/100))}})
	if err != nil {
		t.Fatal(err)
	}
	if err = CancelSalaryApproval(db, a.ID, m.ID, approved.ApprovalID); err != nil {
		t.Fatal(err)
	}
	if err = db.First(&stored, record.ID).Error; err != nil || !stored.IsSettled {
		t.Fatal("old cancellation reopened another approval")
	}
}

// --------- конец нового кода chat
