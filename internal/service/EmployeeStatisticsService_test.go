// --------- новый код chat: смены по положительным отметкам, часы и все буквенные значения
package service

import (
	"project/internal/models"
	"testing"
)

func TestEmployeeYearStatistics(t *testing.T) {
	db := salaryTestDB(t)
	a := models.Agreement{Name: "Первый"}
	db.Create(&a)
	b := models.Agreement{Name: "Второй"}
	db.Create(&b)
	e := models.EmployeeCard{Name: "Иван", AgreementID: a.ID}
	db.Create(&e)
	months := []models.TimeSheetMonth{
		{AgreementID: a.ID, Year: 2026, Month: 1, ShiftHours: "12", HiddenEmployeeIDs: []uint{e.ID}},
		{AgreementID: b.ID, Year: 2026, Month: 1, ShiftHours: "9"},
		{AgreementID: a.ID, Year: 2026, Month: 2, ShiftHours: "12"},
		{AgreementID: a.ID, Year: 2025, Month: 1, ShiftHours: "12"},
	}
	for i := range months {
		if err := db.Create(&months[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	add := func(month int, date, status string) models.TimeSheet {
		row := models.TimeSheet{EmployeeCardID: e.ID, AgreementID: months[month].AgreementID, TimeSheetMonthID: months[month].ID, Date: date, Status: status}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	add(0, "2026-01-01", "12")
	add(0, "2026-01-02", "П")
	add(1, "2026-01-01", "9")
	add(1, "2026-01-03", "П")
	add(2, "2026-02-01", "1,5")
	add(0, "2026-01-04", "1") // Короткий рабочий день также считается одной сменой.
	add(0, "2026-01-05", "П")
	add(0, "2026-01-05", "Б") // Последняя отметка заменяет исторический дубль.
	add(0, "2026-01-06", "Н")
	add(0, "2026-01-07", "25")
	add(0, "2026-01-09", "В")
	add(0, "2026-01-10", "Д")
	add(0, "2026-01-11", "0")
	add(0, "2026-01-12", "")
	add(2, "2026-02-02", "Б")
	add(2, "2026-02-03", "Н")
	deleted := add(0, "2026-01-08", "П")
	db.Delete(&deleted)
	add(3, "2025-01-01", "П")
	result, err := GetEmployeeStatistics(db, e.ID, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Months) != 12 || result.Shifts != "4" || result.Absences != 2 || result.Months[0].Shifts != "3" || result.Months[1].Shifts != "1" {
		t.Fatalf("statistics: %+v", result)
	}
	if result.Hours != "23.500" || result.Months[0].Hours != "22.000" || result.Months[1].Hours != "1.500" ||
		result.SickDays != 2 || result.DaysOff != 1 || result.TravelDays != 1 || result.OffSiteDays != 2 ||
		result.Months[0].SickDays != 1 || result.Months[1].OffSiteDays != 1 {
		t.Fatalf("hours and letter marks: %+v", result)
	}
	empty, err := GetEmployeeStatistics(db, e.ID, 2027)
	// --------- новый код chat: календарь, суммы разных договоров и исключённые отметки
	if len(result.Days) != 365 || result.Days[0].Date != "2026-01-01" || result.Days[364].Date != "2026-12-31" ||
		result.Days[0].Hours != "21.000" || len(result.Days[0].Marks) != 2 ||
		len(result.Days[4].Marks) != 1 || result.Days[4].Marks[0] != "Б" {
		t.Fatalf("calendar aggregation: %+v", result.Days[:5])
	}
	for _, index := range []int{6, 7, 10, 11} {
		if len(result.Days[index].Marks) != 0 || result.Days[index].Hours != "0.000" {
			t.Fatalf("invalid/empty/deleted day: %+v", result.Days[index])
		}
	}
	leap, leapErr := GetEmployeeStatistics(db, e.ID, 2024)
	if leapErr != nil || len(leap.Days) != 366 || leap.Days[59].Date != "2024-02-29" {
		t.Fatal("leap calendar", leapErr)
	}
	// --------- конец нового кода chat
	if err != nil || empty.Shifts != "0" || empty.Hours != "0.000" || empty.Absences != 0 || len(empty.Months) != 12 {
		t.Fatal("empty year", empty, err)
	}
	db.Delete(&months[1])
	result, err = GetEmployeeStatistics(db, e.ID, 2026)
	if err != nil || result.Absences != 1 || result.Shifts != "3" || result.Hours != "14.500" {
		t.Fatal("deleted month counted", result, err)
	}
	if result.Days[0].Hours != "12.000" || len(result.Days[0].Marks) != 1 || len(result.Days[2].Marks) != 0 {
		t.Fatal("deleted month in calendar", result.Days[:3])
	}
	if _, err = GetEmployeeStatistics(db, e.ID, 0); err == nil {
		t.Fatal("invalid year")
	}
	if _, err = GetEmployeeStatistics(db, e.ID+99, 2026); err == nil {
		t.Fatal("missing employee")
	}
	// --------- новый код chat: отбор годов учитывает только действующие положительные отметки
	if len(result.Years) != 1 || result.Years[0] != 2026 {
		t.Fatal("letter-only year included", result.Years)
	}
	add(3, "2025-01-02", "0")
	add(3, "2025-01-03", "12")
	add(3, "2025-01-03", "Б") // Последняя запись отменяет рабочую отметку.
	removed := add(3, "2025-01-04", "8")
	db.Delete(&removed)
	result, err = GetEmployeeStatistics(db, e.ID, 2026)
	if err != nil || len(result.Years) != 1 {
		t.Fatal("invalid worked years", result.Years, err)
	}
	add(3, "2025-01-05", "0,5")
	result, err = GetEmployeeStatistics(db, e.ID, 2026)
	if err != nil || len(result.Years) != 2 || result.Years[0] != 2026 || result.Years[1] != 2025 {
		t.Fatal("worked year order", result.Years, err)
	}
	db.Delete(&months[3])
	result, err = GetEmployeeStatistics(db, e.ID, 2026)
	if err != nil || len(result.Years) != 1 {
		t.Fatal("deleted year included", result.Years, err)
	}
	other := models.EmployeeCard{Name: "Без отметок", AgreementID: a.ID}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	result, err = GetEmployeeStatistics(db, other.ID, 2026)
	if err != nil || result.Years == nil || len(result.Years) != 0 {
		t.Fatal("empty years", result.Years, err)
	}
	// --------- конец нового кода chat
}

// --------- конец нового кода chat
