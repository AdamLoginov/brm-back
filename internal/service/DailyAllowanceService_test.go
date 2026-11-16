// --------- новый код chat: суточные, прогноз, выдачи и пересчёт истории
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"testing"
	"time"
)

func dailyFixture(t *testing.T) (*gorm.DB, models.Agreement, models.EmployeeCard, models.TimeSheetMonth, models.TimeSheetMonth) {
	t.Helper()
	db := salaryTestDB(t)
	if err := db.AutoMigrate(&models.DailyAllowanceAccrual{}, &models.DailyAllowancePayment{}, &models.SalaryPenalty{}); err != nil {
		t.Fatal(err)
	}
	agreement := models.Agreement{Name: "Суточные", Number: "DA"}
	if err := db.Create(&agreement).Error; err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван", AgreementID: agreement.ID}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	january := models.TimeSheetMonth{AgreementID: agreement.ID, Year: 2026, Month: 1, DailyAllowanceKopecks: 50000, Employees: []models.EmployeeCard{employee}}
	february := models.TimeSheetMonth{AgreementID: agreement.ID, Year: 2026, Month: 2, DailyAllowanceKopecks: 60000, Employees: []models.EmployeeCard{employee}}
	for _, month := range []*models.TimeSheetMonth{&january, &february} {
		if err := db.Create(month).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, agreement, employee, january, february
}
func dailyNow(value string) time.Time { result, _ := time.Parse("2006-01-02", value); return result }
func dailyTableEmployee(t *testing.T, table DailyAllowanceTable, id uint) DailyAllowanceEmployee {
	t.Helper()
	for _, row := range table.Employees {
		if row.EmployeeCardID == id {
			return row
		}
	}
	t.Fatal("missing employee")
	return DailyAllowanceEmployee{}
}
func dailyEntries(t *testing.T, db *gorm.DB, agreementID, monthID, employeeID uint, values map[string]string) {
	t.Helper()
	for date, status := range values {
		row := models.TimeSheet{AgreementID: agreementID, TimeSheetMonthID: monthID, EmployeeCardID: employeeID, Date: date, Status: status}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
}
func dailyIssue(t *testing.T, db *gorm.DB, agreementID, monthID, employeeID uint, amount, key string) models.DailyAllowancePayment {
	t.Helper()
	rows, err := CreateDailyAllowancePayments(db, agreementID, DailyAllowanceBatchInput{RequestKey: key, MonthID: monthID, Date: "2026-01-20", Payments: []DailyAllowancePaymentItem{{employeeID, amount}}})
	if err != nil || len(rows) != 1 {
		t.Fatalf("issue: %+v %v", rows, err)
	}
	return rows[0]
}

// --------- новый код chat: покрытие прошлого периода не включает более поздний долг
func TestDailyAllowancePastThroughDate(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-10": "12", "2026-01-20": "1", "2026-01-31": "П"})
	dailyEntries(t, db, agreement.ID, february.ID, employee.ID, map[string]string{"2026-02-01": "12"})
	check := func(date string, want int64, balance int64) {
		t.Helper()
		table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, date, dailyNow("2026-03-15"))
		if err != nil {
			t.Fatal(err)
		}
		row := dailyTableEmployee(t, table, employee.ID)
		if row.SuggestedKopecks != want || row.BalanceKopecks != balance {
			t.Fatalf("%s: suggested=%d balance=%d", date, row.SuggestedKopecks, row.BalanceKopecks)
		}
	}
	check("2026-01-09", 0, -160000)
	check("2026-01-10", 50000, -160000)
	check("2026-01-31", 600000, -160000)
	// Даже более поздняя выдача покрывает старые дни первой.
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "500", "past-through-issue-001")
	check("2026-01-31", 550000, -110000)
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "1000", "past-through-issue-002")
	check("2026-01-31", 450000, -10000)
	// Остаточный долг февраля округляется по ставке выбранного январского табеля.
	check("2026-02-28", 1900000, -10000)
	if _, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-02-30", dailyNow("2026-03-15")); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("invalid date accepted")
	}
}

// --------- конец нового кода chat

// --------- новый код chat: состав таблицы суточных совпадает с выбранным табелем
func TestDailyAllowanceSelectedMonthRoster(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	outside := models.EmployeeCard{Name: "Вне табеля", AgreementID: agreement.ID}
	if err := db.Create(&outside).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&february).Association("Employees").Append(&outside); err != nil {
		t.Fatal(err)
	}
	dailyEntries(t, db, agreement.ID, february.ID, outside.ID, map[string]string{"2026-02-01": "12"})
	dailyIssue(t, db, agreement.ID, february.ID, outside.ID, "600", "roster-outside-001")
	dailyEntries(t, db, agreement.ID, february.ID, employee.ID, map[string]string{"2026-02-02": "12"})
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "1000", "roster-member-001")
	// Перевод не скрывает сотрудника, пока он остаётся в выбранном табеле.
	other := models.Agreement{Name: "После перевода", Number: "ROSTER-2"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&employee).Update("agreement_id", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Employees) != 1 || table.Employees[0].EmployeeCardID != employee.ID || table.Employees[0].IsAssigned || table.Employees[0].BalanceKopecks != 40000 {
		t.Fatalf("roster or balance: %+v", table.Employees)
	}
	if err := db.Model(&january).Association("Employees").Clear(); err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil || len(table.Employees) != 0 {
		t.Fatalf("empty month: %+v %v", table.Employees, err)
	}
	history, err := GetDailyAllowancePayments(db, agreement.ID)
	if err != nil || len(history) != 2 {
		t.Fatal("history must remain available", err)
	}
}

// --------- конец нового кода chat

// --------- новый код chat: каждому сотруднику свой отсчёт от рабочего дня выбранного табеля
func TestDailyAllowanceSuggestedFromIndividualLastWork(t *testing.T) {
	db, agreement, first, january, _ := dailyFixture(t)
	second := models.EmployeeCard{Name: "Второй", AgreementID: agreement.ID}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&january).Association("Employees").Append(&second); err != nil {
		t.Fatal(err)
	}
	dailyEntries(t, db, agreement.ID, january.ID, first.ID, map[string]string{"2026-01-10": "12"})
	dailyEntries(t, db, agreement.ID, january.ID, second.ID, map[string]string{"2026-01-15": "6"})
	// Первый уже имеет 2 дня вперёд; второй — 1 день.
	dailyIssue(t, db, agreement.ID, january.ID, first.ID, "1500", "individual-start-001")
	dailyIssue(t, db, agreement.ID, january.ID, second.ID, "1000", "individual-start-002")
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-01-20", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := dailyTableEmployee(t, table, first.ID), dailyTableEmployee(t, table, second.ID)
	// 11–20 января: 10 дней минус 2 оплаченных; 16–20 января: 5 минус 1.
	if a.SuggestedKopecks != 400000 || b.SuggestedKopecks != 200000 {
		t.Fatalf("different starts: %d %d", a.SuggestedKopecks, b.SuggestedKopecks)
	}
	if a.BalanceKopecks != 100000 || b.BalanceKopecks != 50000 {
		t.Fatal("actual balances changed")
	}
	// Порог голубой подсветки совпадает с предложением по дате.
	if a.Cells[19].PreviewRequiredKopecks != a.SuggestedKopecks || b.Cells[19].PreviewRequiredKopecks != b.SuggestedKopecks {
		t.Fatal("preview and suggestion disagree")
	}
}

// --------- конец нового кода chat

func TestDailyAllowanceBalanceCalendarAndForecast(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-29": "1", "2026-01-30": "П", "2026-01-31": "12"})
	dailyEntries(t, db, agreement.ID, february.ID, employee.ID, map[string]string{"2026-02-01": "6", "2026-02-02": "Б", "2026-02-03": "0", "2026-02-05": "12"})
	// Выплата покрывает сначала два январских дня, затем часть февраля.
	payment := dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "1500", "balance-forecast-001")
	table, err := GetDailyAllowanceTable(db, agreement.ID, february.ID, "2026-02-06", dailyNow("2026-02-05"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != -10000 || row.SuggestedKopecks != 300000 {
		t.Fatalf("balance/rounding: %+v", row)
	}
	if row.Cells[0].State != "unpaid" || row.Cells[0].CoveredKopecks != 50000 || row.Cells[0].Mark != "" {
		t.Fatal("partial day must stay red", row.Cells[0])
	}
	if row.Cells[1].State != "letter" || row.Cells[1].Mark != "Б" || row.Cells[2].State != "empty" {
		t.Fatal("letter/zero states")
	}
	if row.Cells[4].State == "paid" {
		t.Fatal("today treated as actual")
	}
	old, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-02-05"))
	if err != nil {
		t.Fatal(err)
	}
	oldRow := dailyTableEmployee(t, old, employee.ID)
	if oldRow.Cells[28].State != "paid" || oldRow.Cells[28].Mark != "С" || oldRow.Cells[29].State != "letter" || oldRow.Cells[30].State != "paid" {
		t.Fatal("FIFO past month")
	}
	var count int64
	db.Model(&models.DailyAllowanceAccrual{}).Count(&count)
	if count != 4 {
		t.Fatalf("initial backfill should include future numeric row once: %d", count)
	}
	if _, err := GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-02-05")); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.DailyAllowanceAccrual{}).Count(&count)
	if count != 4 {
		t.Fatal("duplicate backfill")
	}
	dailyIssue(t, db, agreement.ID, february.ID, employee.ID, "1800", "balance-forecast-002")
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "2026-02-06", dailyNow("2026-02-05"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 170000 || row.SuggestedKopecks != 120000 || row.Cells[0].State != "paid" || row.Cells[2].State != "reserved" || row.Cells[3].State != "reserved" || row.Cells[4].State == "reserved" { // новый код chat: после работы 1-го пропускаем Б 2-го и покрываем 3-е и 4-е
		t.Fatalf("reservation: %+v", row)
	}
	// Следующий день превращает вчерашнюю числовую отметку в фактическое начисление.
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-02-06"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 110000 || row.Cells[4].State != "paid" {
		t.Fatal("day boundary")
	}
	if err := DeleteDailyAllowancePayment(db, agreement.ID, payment.ID); err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-02-06"))
	if err != nil {
		t.Fatal(err)
	}
	if dailyTableEmployee(t, table, employee.ID).BalanceKopecks != -40000 {
		t.Fatal("delete payment balance")
	}
}

func TestDailyAllowanceFutureRatesAndIsolation(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-02-02", dailyNow("2026-01-31"))
	if err != nil {
		t.Fatal(err)
	}
	// новый код chat: три дня по ставке выбранного январского табеля: 3 × 500 = 1500.
	if dailyTableEmployee(t, table, employee.ID).SuggestedKopecks != 150000 {
		t.Fatal("cross-month forecast")
	}
	if _, err := GetDailyAllowanceTable(db, agreement.ID, february.ID, "2026-03-01", dailyNow("2026-02-28")); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("missing future month accepted", err)
	}
	if err := db.Model(&february).Update("daily_allowance_kopecks", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-02-02", dailyNow("2026-01-31")); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("zero future rate accepted")
	}
	other := models.Agreement{Name: "Другой", Number: "B"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := GetDailyAllowanceTable(db, other.ID, january.ID, "", dailyNow("2026-01-31")); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("cross contract month")
	}
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "500", "before-transfer-001")
	if err := db.Model(&employee).Update("agreement_id", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-01-31"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.IsAssigned || row.BalanceKopecks != 50000 {
		t.Fatal("transferred history")
	}
	_, err = CreateDailyAllowancePayments(db, agreement.ID, DailyAllowanceBatchInput{RequestKey: "after-transfer-001", MonthID: january.ID, Date: "2026-01-31", Payments: []DailyAllowancePaymentItem{{employee.ID, "500"}}})
	if !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("new issue for transferred employee")
	}
}

func TestDailyAllowanceIdempotencyEditsAndRollback(t *testing.T) {
	db, agreement, employee, january, _ := dailyFixture(t)
	input := DailyAllowanceBatchInput{RequestKey: "idempotent-request-001", MonthID: january.ID, Date: "2026-01-20", Payments: []DailyAllowancePaymentItem{{employee.ID, "1000"}}}
	first, err := CreateDailyAllowancePayments(db, agreement.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateDailyAllowancePayments(db, agreement.ID, input)
	if err != nil || len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatal("retry duplicated")
	}
	changed := input
	changed.Date = "2026-01-21"
	if _, err := CreateDailyAllowancePayments(db, agreement.ID, changed); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("key reused with changed payload")
	}
	if err := db.Model(&january).Update("daily_allowance_kopecks", 60000).Error; err != nil {
		t.Fatal(err)
	}
	// 1000 больше не кратны 600, но изменение даты допустимо.
	if _, err := UpdateDailyAllowancePayment(db, agreement.ID, first[0].ID, DailyAllowancePaymentUpdate{"2026-01-21", "1000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateDailyAllowancePayment(db, agreement.ID, first[0].ID, DailyAllowancePaymentUpdate{"2026-01-21", "1100"}); !errors.Is(err, ErrDailyAllowanceInput) {
		t.Fatal("nonmultiple edit")
	}
	if _, err := UpdateDailyAllowancePayment(db, agreement.ID, first[0].ID, DailyAllowancePaymentUpdate{"2026-01-21", "1200"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteDailyAllowancePayment(db, agreement.ID, first[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDailyAllowancePayments(db, agreement.ID, input); err != nil {
		t.Fatal(err)
	}
	live, err := GetDailyAllowancePayments(db, agreement.ID)
	if err != nil || len(live) != 0 {
		t.Fatal("retry resurrected deleted payment")
	}
	employee2 := models.EmployeeCard{Name: "Пётр", AgreementID: agreement.ID}
	if err := db.Create(&employee2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(fmt.Sprintf(`CREATE TRIGGER reject_second_daily BEFORE INSERT ON daily_allowance_payments WHEN NEW.employee_card_id = %d BEGIN SELECT RAISE(ABORT, 'failure'); END`, employee2.ID)).Error; err != nil {
		t.Fatal(err)
	}
	batch := DailyAllowanceBatchInput{RequestKey: "atomic-rollback-001", MonthID: january.ID, Date: "2026-01-20", Payments: []DailyAllowancePaymentItem{{employee.ID, "600"}, {employee2.ID, "600"}}}
	if _, err := CreateDailyAllowancePayments(db, agreement.ID, batch); err == nil {
		t.Fatal("expected rollback")
	}
	live, err = GetDailyAllowancePayments(db, agreement.ID)
	if err != nil || len(live) != 0 {
		t.Fatal("partial batch committed")
	}
}

func TestDailyAllowanceSynchronizesChanges(t *testing.T) {
	db, agreement, employee, january, _ := dailyFixture(t)
	entries := []models.TimeSheet{{TimeSheetMonthID: january.ID, EmployeeCardID: employee.ID, Date: "2026-01-10", Status: "1"}}
	if err := ChangeTimeSheetEntries(db, entries, "create"); err != nil {
		t.Fatal(err)
	}
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "500", "sync-change-issue-001")
	if err := db.Model(&january).Update("daily_allowance_kopecks", 70000).Error; err != nil {
		t.Fatal(err)
	}
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	if dailyTableEmployee(t, table, employee.ID).BalanceKopecks != -20000 {
		t.Fatal("new rate not applied")
	}
	entries[0].Status = "П"
	if err := ChangeTimeSheetEntries(db, entries, "update"); err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 50000 || row.Cells[9].Mark != "П" {
		t.Fatal("letter should refund charge")
	}
	entries[0].Status = "12"
	if err := ChangeTimeSheetEntries(db, entries, "update"); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&january).Association("Employees").Clear(); err != nil {
			return err
		}
		return SyncDailyAllowanceAccruals(tx, january.ID)
	}); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.DailyAllowanceAccrual{}).Count(&count)
	if count != 0 {
		t.Fatal("removed employee still accrued")
	}
	payments, err := GetDailyAllowancePayments(db, agreement.ID)
	if err != nil || len(payments) != 1 {
		t.Fatal("actual payment removed")
	}
}

// --------- конец нового кода chat
