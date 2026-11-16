// --------- новый код chat: изоляция штрафов по договору, сотруднику, месяцу и году
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func TestSalaryPenaltiesByMonth(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryPenalty{}, &models.SalaryAdvance{}); err != nil {
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
	stranger := models.EmployeeCard{Name: "Пётр", AgreementID: other.ID}
	for _, row := range []*models.EmployeeCard{&employee, &stranger} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	month := models.TimeSheetMonth{AgreementID: agreement.ID, Year: 2026, Month: 11}
	if err := db.Create(&month).Error; err != nil {
		t.Fatal(err)
	}
	input := SalaryPenaltyInput{EmployeeCardID: employee.ID, Date: "2026-11-01", Amount: "100.01"}
	for _, amount := range []string{"0", "-1", "", "abc", "1.001", "99999999999999999999"} {
		bad := input
		bad.Amount = amount
		if _, err := SaveSalaryPenalty(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryPenaltyInput) {
			t.Fatalf("invalid amount accepted: %q", amount)
		}
	}
	bad := input
	bad.Date = "2026-02-30"
	if _, err := SaveSalaryPenalty(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryPenaltyInput) {
		t.Fatal("invalid date")
	}
	bad = input
	bad.EmployeeCardID = stranger.ID
	if _, err := SaveSalaryPenalty(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryPenaltyInput) {
		t.Fatal("foreign employee")
	}
	penalty, err := SaveSalaryPenalty(db, agreement.ID, 0, input)
	if err != nil || penalty.AmountKopecks != 10001 {
		t.Fatalf("create: %+v %v", penalty, err)
	}
	if _, err := SaveSalaryPenalty(db, other.ID, penalty.ID, input); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("foreign edit")
	}
	if err := DeleteSalaryPenalty(db, other.ID, penalty.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("foreign delete")
	}
	for _, row := range []models.SalaryPenalty{
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-11-30", AmountKopecks: 99},
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-12-01", AmountKopecks: 777},
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-10-31", AmountKopecks: 777},
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2025-11-01", AmountKopecks: 777},
		{AgreementID: other.ID, EmployeeCardID: employee.ID, Date: "2026-11-01", AmountKopecks: 777},
		{AgreementID: agreement.ID, EmployeeCardID: stranger.ID, Date: "2026-11-01", AmountKopecks: 777},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.SalaryAdvance{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-01", AmountKopecks: 200}).Error; err != nil {
		t.Fatal(err)
	}
	gross := []models.SalaryAccrual{{EmployeeCardID: employee.ID, AmountKopecks: 10000}}
	summary, err := AddSalaryAdvances(db, agreement.ID, gross)
	if err != nil {
		t.Fatal(err)
	}
	check := func(want int64) {
		t.Helper()
		got, err := AddSalaryPenalties(db, agreement.ID, month.ID, summary)
		if err != nil || got.TotalPenaltiesKopecks != want || got.TotalPayableKopecks != 9800-want || got.Accruals[0].PayableKopecks != 9800-want || got.Accruals[0].PenaltiesKopecks != want {
			t.Fatalf("summary: %+v %v", got, err)
		}
		if gross[0].AmountKopecks != 10000 || summary.Accruals[0].PayableKopecks != 9800 {
			t.Fatal("source mutated")
		}
	}
	check(10100)
	if err := db.Model(&employee).Update("agreement_id", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	input.Date = "2026-12-15"
	if _, err := SaveSalaryPenalty(db, agreement.ID, penalty.ID, input); err != nil {
		t.Fatal(err)
	}
	check(99)
	if _, err := SaveSalaryPenalty(db, agreement.ID, 0, input); !errors.Is(err, ErrSalaryPenaltyInput) {
		t.Fatal("new penalty for transferred employee")
	}
	input.Date = "2026-11-15"
	if _, err := SaveSalaryPenalty(db, agreement.ID, penalty.ID, input); err != nil {
		t.Fatal(err)
	}
	check(10100)
	if err := DeleteSalaryPenalty(db, agreement.ID, penalty.ID); err != nil {
		t.Fatal(err)
	}
	check(99)
	if _, err := AddSalaryPenalties(db, other.ID, month.ID, summary); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("foreign timesheet")
	}
	huge := models.SalaryPenalty{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-11-15", AmountKopecks: maxSalaryKopecks}
	if err := db.Create(&huge).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AddSalaryPenalties(db, agreement.ID, month.ID, summary); !errors.Is(err, ErrSalaryAmountOverflow) {
		t.Fatal("overflow accepted")
	}
}

// --------- конец нового кода chat
