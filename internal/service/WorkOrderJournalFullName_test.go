// --------- новый код chat: полные ФИО различают одинаковые инициалы при импорте
package service

import (
	"project/internal/models"
	"testing"
)

func TestWorkJournalFullNamesSeparateHours(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	surname, middle := "Кондра", "Николаевич"
	if err := f.db.Model(&f.employee).Updates(map[string]any{"surname": surname, "name": "Александр", "middle_name": middle}).Error; err != nil {
		t.Fatal(err)
	}
	other := models.EmployeeCard{Name: "Андрей", Surname: &surname, MiddleName: &middle, AgreementID: f.a.ID}
	if err := f.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	journalSetRow(t, file, 3, []string{"2028-02-01", "Кондра Александр Николаевич", "11", "1", "1"})
	journalSetRow(t, file, 4, []string{"2028-02-01", "Кондра Андрей Николаевич", "7", "1", "1"})
	data := excelBytes(t, file)
	report := previewExcel(t, f, data)
	confirmExcel(t, f, data, report.Token)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Shifts) != 1 || len(book.Shifts[0].Employees) != 2 {
		t.Fatalf("unexpected shifts: %+v", book.Shifts)
	}
	hours := map[uint]int64{}
	for _, employee := range book.Shifts[0].Employees {
		hours[employee.EmployeeCardID] = employee.HoursHundredths
	}
	if hours[f.employee.ID] != 1100 || hours[other.ID] != 700 {
		t.Fatalf("hours assigned incorrectly: %v", hours)
	}
}

// --------- конец нового кода chat
