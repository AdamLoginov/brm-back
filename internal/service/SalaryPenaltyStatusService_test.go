// --------- новый код chat: статусы штрафов и миграция старых записей
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
	"time"
)

func TestSalaryPenaltyStatuses(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Договор", Number: "1"}
	other := models.Agreement{Name: "Другой", Number: "2"}
	for _, row := range []*models.Agreement{&agreement, &other} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	month := models.TimeSheetMonth{AgreementID: agreement.ID, Year: 2026, Month: 11}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	input := SalaryPenaltyInput{EmployeeCardID: employee.ID, Date: "2026-11-01", Amount: "100"}
	row, err := SaveSalaryPenalty(db, agreement.ID, 0, input)
	if err != nil || row.Status != SalaryPenaltyActive || row.ClosedAt != nil {
		t.Fatalf("new status: %+v %v", row, err)
	}
	summary := SalaryAccrualSummary{Accruals: []SalaryAccrualWithAdvances{{SalaryAccrual: models.SalaryAccrual{EmployeeCardID: employee.ID, AmountKopecks: 20000}, PayableKopecks: 20000}}, TotalKopecks: 20000, TotalPayableKopecks: 20000}
	check := func(want int64) {
		t.Helper()
		got, err := AddSalaryPenalties(db, agreement.ID, month.ID, summary)
		if err != nil || got.TotalPenaltiesKopecks != want || got.TotalPayableKopecks != 20000-want {
			t.Fatalf("deduction: %+v %v", got, err)
		}
	}
	check(10000)
	if _, err := ChangeSalaryPenaltyStatus(db, other.ID, row.ID, SalaryPenaltyForgiven); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross contract change")
	}
	for _, status := range []string{"closed", "bad", "active"} {
		if _, err := ChangeSalaryPenaltyStatus(db, agreement.ID, row.ID, status); !errors.Is(err, ErrSalaryPenaltyStatus) {
			t.Fatal("invalid transition", status, err)
		}
	}
	forgiven, err := ChangeSalaryPenaltyStatus(db, agreement.ID, row.ID, SalaryPenaltyForgiven)
	if err != nil || forgiven.Status != SalaryPenaltyForgiven || forgiven.ClosedAt == nil {
		t.Fatalf("forgive: %+v %v", forgiven, err)
	}
	check(0)
	list, err := GetSalaryPenalties(db, agreement.ID)
	if err != nil || len(list) != 1 || list[0].Status != SalaryPenaltyForgiven {
		t.Fatal("forgiven hidden")
	}
	input.Comment = "Обновление"
	edited, err := SaveSalaryPenalty(db, agreement.ID, row.ID, input)
	if err != nil || edited.Status != SalaryPenaltyForgiven || !edited.ClosedAt.Equal(*forgiven.ClosedAt) {
		t.Fatal("edit reset status/date")
	}
	active, err := ChangeSalaryPenaltyStatus(db, agreement.ID, row.ID, SalaryPenaltyActive)
	if err != nil || active.ClosedAt != nil || active.Status != SalaryPenaltyActive {
		t.Fatal("restore", err)
	}
	check(10000)
	// Закрытие выплатой пока имитируем напрямую, публичного перехода в closed нет.
	closedAt := time.Now().UTC()
	if err := db.Model(&models.SalaryPenalty{}).Where("id = ?", row.ID).Updates(map[string]any{"status": SalaryPenaltyClosed, "closed_at": closedAt}).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{SalaryPenaltyActive, SalaryPenaltyForgiven} {
		if _, err := ChangeSalaryPenaltyStatus(db, agreement.ID, row.ID, status); !errors.Is(err, ErrSalaryPenaltyStatus) {
			t.Fatal("closed status changed")
		}
	}
	check(10000)
	input.Amount = "50"
	edited, err = SaveSalaryPenalty(db, agreement.ID, row.ID, input)
	if err != nil || edited.Status != SalaryPenaltyClosed || edited.ClosedAt == nil || !edited.ClosedAt.Equal(closedAt) {
		t.Fatal("closed edit", err)
	}
	check(5000)
	if err := DeleteSalaryPenalty(db, agreement.ID, row.ID); err != nil {
		t.Fatal(err)
	}
	check(0)
}

func TestSalaryPenaltyLegacyMigration(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.Exec(`CREATE TABLE salary_penalties (id integer PRIMARY KEY, created_at datetime, updated_at datetime, deleted_at datetime, agreement_id integer, employee_card_id integer, employee_name text, date text, amount_kopecks integer CHECK(amount_kopecks > 0), comment text)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO salary_penalties (id,agreement_id,employee_card_id,date,amount_kopecks,comment) VALUES (1,1,1,'2026-11-01',12345,'old')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	var row models.SalaryPenalty
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != SalaryPenaltyActive || row.ClosedAt != nil || row.AmountKopecks != 12345 || row.Comment != "old" {
		t.Fatalf("migration: %+v", row)
	}
}

// --------- конец нового кода chat
