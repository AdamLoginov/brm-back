// --------- новый код chat: импорт общего журнала, изоляция смен и строгая проверка пропусков
package service

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"project/internal/models"
	"strings"
	"testing"
	"time"
)

func journalFixtureFile(t *testing.T, f workFixture) *excelize.File {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { file.Close() })
	headers := []string{"Дата", "Сотрудник", "Часы", "смена", "цех", "блок", "поз.", "Наменование", "ед изм", "кол-во", "чел-час", "Материалы", "ед. изм", "кол-во"}
	for i, value := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		excelCell(t, file, "Sheet1", cell, value)
	}
	surname, middle := "Иванов", "Иванович"
	f.db.Model(&f.employee).Updates(map[string]interface{}{"surname": surname, "middle_name": middle})
	excelCell(t, file, "Sheet1", "A2", "Заголовок таблицы") // новый код chat: произвольный заголовок второй строки игнорируется
	journalSetRow(t, file, 3, []string{"2028-02-01", "Иванов И.И. (мастер)", "11", "1", "1", "Блок", "1013", "Монтаж", "м", "2.5", "", "Кабель", "м", "3.125"})
	return file
}
func journalSetRow(t *testing.T, file *excelize.File, row int, values []string) {
	t.Helper()
	complete := make([]string, 14)
	copy(complete, values)
	values = complete
	for i, value := range values {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		excelCell(t, file, "Sheet1", cell, value)
	}
}

// новый код chat: подтверждение привязано к файлу и актуальным остаткам склада.
func TestWorkJournalRejectsStaleConfirmation(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	data := excelBytes(t, file)
	report := previewExcel(t, f, data)
	excelCell(t, file, "Sheet1", "H3", "Изменённая работа")
	if _, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), report.Token, true); err == nil {
		t.Fatal("changed file accepted")
	}
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, 0, WarehouseReceiptInput{Date: "2028-02-03", Quantity: "1"}, false); err != nil {
		t.Fatal(err)
	}
	before := workExcelFingerprint(fixtureState(t, f))
	if _, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, report.Token, true); err == nil {
		t.Fatal("stale stock accepted")
	}
	if before != workExcelFingerprint(fixtureState(t, f)) {
		t.Fatal("rejected confirmation changed database")
	}
}

// новый код chat: повторный импорт не снимает сохранённую отметку.
func TestWorkJournalPreservesAccounted(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	data := excelBytes(t, file)
	report := previewExcel(t, f, data)
	confirmExcel(t, f, data, report.Token)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	task := book.Shifts[0].Tasks[0]
	if err := f.db.Model(&models.WorkOrderTask{}).Where("id = ?", task.ID).Update("accounted", true).Error; err != nil {
		t.Fatal(err)
	}
	excelCell(t, file, "Sheet1", "J3", "4")
	data = excelBytes(t, file)
	report = previewExcel(t, f, data)
	confirmExcel(t, f, data, report.Token)
	book, err = LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil || !book.Shifts[0].Tasks[0].Accounted || book.Shifts[0].Tasks[0].QuantityHundredths != 400 {
		t.Fatal("accounted flag lost", err, book)
	}
}

func TestWorkJournalImportLifecycle(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	// Вторая строка той же даты читает объединённую дату; один сотрудник не дублируется.
	journalSetRow(t, file, 4, []string{"", "Иванов И. И.", "11", "1", "Цех 1", "", "", "Уборка", "", "", "0"})
	if err := file.MergeCell("Sheet1", "A3", "A4"); err != nil {
		t.Fatal(err)
	}
	in := f.input()
	in.Number = 2
	in.Tasks = nil
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	data := excelBytes(t, file)
	before := workExcelFingerprint(fixtureState(t, f))
	report := previewExcel(t, f, data)
	if before != workExcelFingerprint(fixtureState(t, f)) {
		t.Fatal("preview wrote database")
	}
	confirmExcel(t, f, data, report.Token)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil || len(book.Shifts) != 2 {
		t.Fatal(book, err)
	}
	var imported WorkShiftDetail
	for _, s := range book.Shifts {
		if s.Number == 1 {
			imported = s
		}
	}
	if len(imported.Employees) != 1 || len(imported.Tasks) != 2 || !imported.Tasks[0].HoursNotSpecified || imported.Tasks[1].HoursNotSpecified {
		t.Fatal(imported)
	}
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 97000 {
		t.Fatal(balance)
	}
	repeat := previewExcel(t, f, data)
	if len(repeat.Changes) != 0 {
		t.Fatal("repeated import duplicates", repeat.Changes)
	}
	// Удаление работы из журнала меняет только первую смену и возвращает материал.
	file.UnmergeCell("Sheet1", "A3", "A4")
	journalSetRow(t, file, 3, []string{"2028-02-01", "Иванов И.И.", "10", "1", "1", "", "", "Уборка", "", "", ""})
	for col := 1; col <= 14; col++ {
		cell, _ := excelize.CoordinatesToCellName(col, 4)
		excelCell(t, file, "Sheet1", cell, "")
	}
	data = excelBytes(t, file)
	report = previewExcel(t, f, data)
	deleted := false
	for _, c := range report.Changes {
		if c.Action == "delete" {
			deleted = true
		}
	}
	if !deleted {
		t.Fatal("no deletion preview")
	}
	confirmExcel(t, f, data, report.Token)
	book, _ = LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if len(book.Shifts) != 2 {
		t.Fatal("unrepresented shift lost")
	}
	balance, _ = WarehouseBalance(f.db, f.material.ID)
	if balance != 100125 {
		t.Fatal("material not returned", balance)
	}
}

func TestWorkJournalErrorsBlockImport(t *testing.T) {
	for _, test := range []struct{ cell, value, want string }{
		{"C3", "", "часы сотрудника"}, {"E3", "", "не указан цех"}, {"B3", "Неизвестный А.Б.", "совпадений 0"},
		{"G3", "9999", "позиция"}, {"A3", "2028-03-01", "месяцу"}, {"I3", "неизвестно", "единица работы"},
		{"N3", "200", "Недостаточно"}, {"C3", "25", "24"},
	} {
		t.Run(test.cell+test.value, func(t *testing.T) {
			f := newWorkFixture(t)
			file := journalFixtureFile(t, f)
			excelCell(t, file, "Sheet1", test.cell, test.value)
			before := workExcelFingerprint(fixtureState(t, f))
			r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
			if e != nil || !strings.Contains(strings.Join(r.Errors, " "), test.want) || r.Token != "" {
				t.Fatal(e, r.Errors)
			}
			if before != workExcelFingerprint(fixtureState(t, f)) {
				t.Fatal("failed import changed data")
			}
		})
	}
}
func TestWorkJournalAmbiguousEmployeeAndSerialDate(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	// Excel хранит даты числом; проверяем систему дат 1900.
	serial := time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24
	file.SetCellFloat("Sheet1", "A3", serial, 0, 64)
	previewExcel(t, f, excelBytes(t, file))
	surname, middle := "Иванов", "Иванович"
	other := models.EmployeeCard{Name: "Игорь", Surname: &surname, MiddleName: &middle, AgreementID: f.a.ID}
	f.db.Create(&other)
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
	if e != nil || !strings.Contains(strings.Join(r.Errors, " "), "совпадений 2") {
		t.Fatal(e, r.Errors)
	}
}

// новый код chat: список ошибок не обрезается на сотне и уменьшается после исправлений.
func TestWorkJournalAllErrorsReported(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	for row := 3; row <= 122; row++ {
		journalSetRow(t, file, row, []string{"2028-02-01", "Иванов И.И.", "", "1", "1"})
	}
	report, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
	if err != nil || len(report.Errors) != 120 {
		t.Fatalf("expected 120 errors, got %d: %v", len(report.Errors), err)
	}
	for row := 3; row <= 12; row++ {
		excelCell(t, file, "Sheet1", fmt.Sprintf("C%d", row), "11")
	}
	report, err = ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
	if err != nil || len(report.Errors) != 110 {
		t.Fatalf("expected 110 errors after corrections, got %d: %v", len(report.Errors), err)
	}
}

func TestWorkJournalManyRowsKeepOrder(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	for i := 0; i < 20; i++ {
		person, hours := "", ""
		if i == 0 {
			person, hours = "Иванов И.И.", "11"
		}
		journalSetRow(t, file, i+3, []string{"2028-02-01", person, hours, "1", "1", "", "", fmt.Sprintf("Работа %02d", i+1)})
	}
	data := excelBytes(t, file)
	r := previewExcel(t, f, data)
	confirmExcel(t, f, data, r.Token)
	book, _ := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	for i, task := range book.Shifts[0].Tasks {
		if task.Name != fmt.Sprintf("Работа %02d", i+1) {
			t.Fatal("row order", i, task.Name)
		}
	}
	r = previewExcel(t, f, data)
	if len(r.Changes) != 0 {
		t.Fatal("repeated import changed order", r.Changes)
	}
}

// --------- конец нового кода chat
