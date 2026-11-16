// --------- новый код chat: проверки авансов и вычетов
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
)

func TestSalaryAdvances(t *testing.T) {
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.SalaryAdvance{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Первый", Number: "1"}
	other := models.Agreement{Name: "Другой", Number: "2"}
	for _, item := range []*models.Agreement{&agreement, &other} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	stranger := models.EmployeeCard{Name: "Пётр", AgreementID: other.ID}
	for _, item := range []*models.EmployeeCard{&employee, &stranger} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	input := SalaryAdvanceInput{EmployeeCardID: employee.ID, Date: "2026-01-15", Amount: "1500,01"}
	for _, value := range []string{"", "0", "-1", "abc", "1.001", "99999999999999999999"} {
		bad := input
		bad.Amount = value
		if _, err := SaveSalaryAdvance(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryAdvanceInput) {
			t.Fatalf("invalid amount %q: %v", value, err)
		}
	}
	bad := input
	bad.Date = "2026-02-30"
	if _, err := SaveSalaryAdvance(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryAdvanceInput) {
		t.Fatal("invalid date accepted")
	}
	bad = input
	bad.EmployeeCardID = stranger.ID
	if _, err := SaveSalaryAdvance(db, agreement.ID, 0, bad); !errors.Is(err, ErrSalaryAdvanceInput) {
		t.Fatal("foreign employee accepted")
	}
	row, err := SaveSalaryAdvance(db, agreement.ID, 0, input)
	if err != nil || row.AmountKopecks != 150001 || row.IsSettled {
		t.Fatalf("create: %+v %v", row, err)
	}
	if _, err := SaveSalaryAdvance(db, other.ID, row.ID, input); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross agreement edit allowed")
	}
	if err := DeleteSalaryAdvance(db, other.ID, row.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross agreement delete allowed")
	}
	if err := db.Model(&employee).Update("agreement_id", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	input.Comment = "После перевода"
	updated, err := SaveSalaryAdvance(db, agreement.ID, row.ID, input)
	if err != nil || updated.ID != row.ID || updated.Comment != input.Comment {
		t.Fatalf("edit transferred employee: %v", err)
	}
	if _, err := SaveSalaryAdvance(db, agreement.ID, 0, input); !errors.Is(err, ErrSalaryAdvanceInput) {
		t.Fatal("new advance to transferred employee allowed")
	}
	list, err := GetSalaryAdvances(db, agreement.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("transferred employee hidden: %v", err)
	}
	// Другая дата и другой договор не должны смешиваться; закрытые авансы не вычитаются.
	extra := []models.SalaryAdvance{
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2025-12-01", AmountKopecks: 100},
		{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-01", AmountKopecks: 900, IsSettled: true},
		{AgreementID: other.ID, EmployeeCardID: employee.ID, Date: "2026-01-01", AmountKopecks: 700},
	}
	if err := db.Create(&extra).Error; err != nil {
		t.Fatal(err)
	}
	accruals := []models.SalaryAccrual{{EmployeeCardID: employee.ID, AmountKopecks: 100000}}
	summary, err := AddSalaryAdvances(db, agreement.ID, accruals)
	if err != nil || summary.TotalAdvancesKopecks != 150101 || summary.TotalPayableKopecks != -50101 || summary.Accruals[0].PayableKopecks != -50101 || accruals[0].AmountKopecks != 100000 {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	if err := DeleteSalaryAdvance(db, agreement.ID, row.ID); err != nil {
		t.Fatal(err)
	}
	summary, err = AddSalaryAdvances(db, agreement.ID, accruals)
	if err != nil || summary.TotalAdvancesKopecks != 100 {
		t.Fatalf("deleted advance counted: %+v %v", summary, err)
	}
	huge := models.SalaryAdvance{AgreementID: agreement.ID, EmployeeCardID: employee.ID, Date: "2026-01-01", AmountKopecks: maxSalaryKopecks}
	if err := db.Create(&huge).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := AddSalaryAdvances(db, agreement.ID, accruals); !errors.Is(err, ErrSalaryAmountOverflow) {
		t.Fatal("aggregate overflow accepted")
	}
}

// --------- конец нового кода chat
