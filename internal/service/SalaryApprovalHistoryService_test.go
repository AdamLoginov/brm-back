// --------- новый код chat: история, группировка, отмена и изоляция по месяцу
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func TestSalaryApprovalHistoryGroupsAndCancellation(t *testing.T) {
	db, a, m, e, other := employeeApprovalFixture(t)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	author := models.User{Login: "history-author", Password: "test", EmployeeCardID: e.ID}
	if err := db.Create(&author).Error; err != nil {
		t.Fatal(err)
	}
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	input := SalaryEmployeeApprovalInput{ActorID: author.ID, Date: "2026-02-01", RequestKey: "history-group-first", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), true, "5000"), employeeSelection(salaryRow(t, s, other.ID), true, "3000")}}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	history, err := GetSalaryApprovalHistory(db, a.ID, m.ID)
	if err != nil || len(history) != 1 {
		t.Fatal("history group", history, err)
	}
	group := history[0]
	if group.AuthorUserID != author.ID || group.AuthorName != author.Login || len(group.Employees) != 2 || group.TotalKopecks != 800000 || group.CreatedAt.IsZero() {
		t.Fatal("group metadata", group)
	}
	if _, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input); err != nil {
		t.Fatal("retry", err)
	}
	input.ActorID = author.ID + 1
	if _, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, input); !errors.Is(err, ErrSalaryApprovalConflict) {
		t.Fatal("other author retry", err)
	}
	if _, err = GetSalaryApprovalHistory(db, a.ID+1, m.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("agreement isolation", err)
	}
	if _, err = GetSalaryApprovalHistory(db, a.ID, m.ID+1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("month isolation", err)
	}
	if err = db.Model(&author).Update("login", "changed-login").Error; err != nil {
		t.Fatal(err)
	}
	s, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{ActorID: author.ID, RequestKey: "history-cancel-one", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), false, "")}})
	if err != nil {
		t.Fatal(err)
	}
	history, err = GetSalaryApprovalHistory(db, a.ID, m.ID)
	if err != nil || len(history) != 1 || len(history[0].Employees) != 1 || history[0].TotalKopecks != 300000 || history[0].AuthorName != "history-author" {
		t.Fatal("partial cancellation", history, err)
	}
	_, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{ActorID: author.ID, RequestKey: "history-cancel-rest", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, other.ID), false, "")}})
	if err != nil {
		t.Fatal(err)
	}
	history, err = GetSalaryApprovalHistory(db, a.ID, m.ID)
	if err != nil || len(history) != 0 {
		t.Fatal("cancelled visible", history, err)
	}
	var count int64
	db.Model(&models.SalaryApproval{}).Count(&count)
	if count != 2 {
		t.Fatal("service records deleted", count)
	}
}
func TestLegacySalaryApprovalHistoryPartialCancellation(t *testing.T) {
	db, a, m, e, _ := employeeApprovalFixture(t)
	s, err := GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	input := SalaryApprovalInput{Date: "2026-02-01", RequestKey: "history-legacy", Revision: s.Revision}
	for _, row := range s.Accruals {
		input.Payments = append(input.Payments, struct {
			EmployeeCardID uint   `json:"employee_card_id"`
			Amount         string `json:"amount"`
		}{row.EmployeeCardID, "0"})
	}
	s, err = ApproveSalary(db, a.ID, m.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	history, err := GetSalaryApprovalHistory(db, a.ID, m.ID)
	if err != nil || len(history) != 1 || history[0].AuthorName != "Не указан" {
		t.Fatal(history, err)
	}
	key, created := history[0].Key, history[0].CreatedAt
	s, err = GetSalaryCalculation(db, a.ID, m.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ApplySalaryEmployeeApprovals(db, a.ID, m.ID, SalaryEmployeeApprovalInput{RequestKey: "legacy-history-cancel", Employees: []SalaryEmployeeSelection{employeeSelection(salaryRow(t, s, e.ID), false, "")}})
	if err != nil {
		t.Fatal(err)
	}
	history, err = GetSalaryApprovalHistory(db, a.ID, m.ID)
	if err != nil || len(history) != 1 || len(history[0].Employees) != 1 || history[0].Key != key || !history[0].CreatedAt.Equal(created) {
		t.Fatal("legacy metadata lost", history, err)
	}
}

// --------- конец нового кода chat
