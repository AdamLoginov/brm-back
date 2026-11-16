// --------- новый код chat: подсветка остатка после последнего рабочего дня
package service

import (
	"project/internal/models"
	"testing"
)

// --------- новый код chat: пороги предварительного покрытия учитывают долг и не создают выдач
func TestDailyAllowanceInputPreviewThresholds(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-30": "12", "2026-01-31": "В"})
	dailyEntries(t, db, agreement.ID, february.ID, employee.ID, map[string]string{"2026-02-01": "П"})
	table, err := GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	// Сначала долг 500, затем дни по 600; буквенные отметки не расходуют деньги.
	if row.BalanceKopecks != -50000 || row.Cells[0].PreviewRequiredKopecks != 0 || row.Cells[1].PreviewRequiredKopecks != 110000 || row.Cells[2].PreviewRequiredKopecks != 170000 {
		t.Fatalf("thresholds: %+v", row)
	}
	for _, cell := range row.Cells {
		if cell.State == "reserved" {
			t.Fatal("preview changed real coverage")
		}
	}
	history, err := GetDailyAllowancePayments(db, agreement.ID)
	if err != nil || len(history) != 0 {
		t.Fatal("preview created payment")
	}
	past, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if dailyTableEmployee(t, past, employee.ID).Cells[29].PreviewRequiredKopecks != 50000 {
		t.Fatal("old debt threshold")
	}
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "1500", "preview-existing-001")
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	if row.Cells[1].State != "reserved" || row.Cells[1].PreviewRequiredKopecks != 0 || row.Cells[2].PreviewRequiredKopecks != 20000 || row.Cells[3].PreviewRequiredKopecks != 80000 {
		t.Fatal("existing balance not applied to preview")
	}
}

// --------- конец нового кода chat

func TestDailyAllowanceReserveAfterLastWork(t *testing.T) {
	db, agreement, employee, january, _ := dailyFixture(t)
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-10": "12", "2026-01-11": "П", "2026-01-13": "Б"})
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "2000", "reserve-after-work-001")
	table, err := GetDailyAllowanceTable(db, agreement.ID, january.ID, "2026-01-31", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 150000 || row.SuggestedKopecks != 800000 {
		t.Fatal("highlight changed balance or suggested amount")
	}
	for _, day := range []int{12, 14, 15} {
		if row.Cells[day-1].State != "reserved" || row.Cells[day-1].Mark != "" {
			t.Fatalf("day %d not reserved: %+v", day, row.Cells[day-1])
		}
	}
	if row.Cells[9].State != "paid" || row.Cells[10].State != "letter" || row.Cells[10].Mark != "П" || row.Cells[12].State != "letter" || row.Cells[15].State != "empty" {
		t.Fatal("letters or coverage boundary changed")
	}
	// Новая фактическая работа сдвигает начало покрытия, старые пустые дни не считаются долгом.
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-20": "1"})
	table, err = GetDailyAllowanceTable(db, agreement.ID, january.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 100000 || row.Cells[11].State != "empty" || row.Cells[20].State != "reserved" || row.Cells[21].State != "reserved" || row.Cells[22].State != "empty" {
		t.Fatal("last work did not move coverage")
	}
}

func TestDailyAllowanceReserveCrossesMonth(t *testing.T) {
	db, agreement, employee, january, february := dailyFixture(t)
	dailyEntries(t, db, agreement.ID, january.ID, employee.ID, map[string]string{"2026-01-30": "12", "2026-01-31": "В"})
	dailyEntries(t, db, agreement.ID, february.ID, employee.ID, map[string]string{"2026-02-01": "П"})
	dailyIssue(t, db, agreement.ID, january.ID, employee.ID, "1500", "reserve-cross-month-001")
	table, err := GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row := dailyTableEmployee(t, table, employee.ID)
	if row.BalanceKopecks != 100000 || row.Cells[0].State != "letter" || row.Cells[1].State != "reserved" || row.Cells[1].AmountKopecks != 60000 || row.Cells[2].State == "reserved" || row.Cells[2].CoveredKopecks != 40000 {
		t.Fatalf("cross-month coverage: %+v", row)
	}
	// Нулевой или отрицательный остаток не подсвечивает ни одного дня.
	if err := db.Where("agreement_id = ?", agreement.ID).Delete(&models.DailyAllowancePayment{}).Error; err != nil {
		t.Fatal(err)
	}
	table, err = GetDailyAllowanceTable(db, agreement.ID, february.ID, "", dailyNow("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	row = dailyTableEmployee(t, table, employee.ID)
	for _, cell := range row.Cells {
		if cell.State == "reserved" {
			t.Fatal("negative balance reserved day")
		}
	}
}

// --------- конец нового кода chat
