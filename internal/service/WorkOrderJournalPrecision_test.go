// --------- новый код chat: общий список сотрудников и точность объёмов импорта
package service

import (
	"project/internal/models"
	"strings"
	"testing"
)

func TestWorkJournalDismissedEmployeeAndThousandths(t *testing.T) {
	for _, dismissed := range []bool{false, true} {
		f := newWorkFixture(t)
		file := journalFixtureFile(t, f)
		if err := f.db.Model(&f.employee).Updates(map[string]any{"agreement_id": 0, "status": dismissed, "dismissal_date": "2028-02-15"}).Error; err != nil { // новый код chat: увольнение вместо удаления
			t.Fatal(err)
		}
		excelCell(t, file, "Sheet1", "J3", "1,125")
		data := excelBytes(t, file)
		report := previewExcel(t, f, data)
		confirmExcel(t, f, data, report.Token)
		book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
		if err != nil {
			t.Fatal(err)
		}
		task := book.Shifts[0].Tasks[0]
		if workQuantityThousandths(task.WorkOrderTask) != 1125 {
			t.Fatalf("lost precision: %+v", task)
		}
		var employee models.EmployeeCard
		if err := f.db.First(&employee, f.employee.ID).Error; err != nil {
			t.Fatal(err)
		}
		if employee.AgreementID != 0 || employee.Status != dismissed || employee.DismissalDate != "2028-02-15" {
			t.Fatal("employee employment or agreement changed")
		}
		if len(previewExcel(t, f, data).Changes) != 0 {
			t.Fatal("repeat import changed data")
		}
		input := WorkTaskInput{Name: task.Name, UnitID: task.UnitID, Quantity: "1.125", Hours: "", BlockID: task.BlockID, PositionID: task.PositionID}
		updated, _, err := UpdateWorkTask(f.db, f.a.ID, task.ID, book.Shifts[0].Version, &input, nil)
		if err != nil {
			t.Fatal(err)
		}
		if workQuantityThousandths(updated.WorkOrderTask) != 1125 {
			t.Fatal("editing rounded quantity")
		}
		statement, err := BuildWorkOrderPositionStatement(f.db, f.a.ID, f.book.ID, f.position.ID)
		if err != nil {
			t.Fatal(err)
		}
		value, err := statement.GetCellValue(statement.GetSheetList()[0], "D14")
		statement.Close()
		if err != nil || value != "1.125" {
			t.Fatalf("statement rounded: %s %v", value, err)
		}
		export, err := BuildWorkOrderPositionExcel(f.db, f.a.ID, f.book.ID)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := export.GetRows("Работы по позициям")
		export.Close()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if len(row) > 3 && row[1] == task.Name {
				found = true
				if row[3] != "1.125" {
					t.Fatalf("export rounded: %v", row)
				}
			}
		}
		if !found {
			t.Fatal("export work missing")
		}
	}
}

func TestWorkJournalQuantityPrecisionLimit(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	excelCell(t, file, "Sheet1", "J3", "1.1234")
	report, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
	if err != nil || !strings.Contains(strings.Join(report.Errors, " "), "объём работы: Слишком много знаков") {
		t.Fatal(err, report.Errors)
	}
	excelCell(t, file, "Sheet1", "J3", "1.123")
	excelCell(t, file, "Sheet1", "K3", "1.123")
	report, err = ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), "", false)
	if err != nil || !strings.Contains(strings.Join(report.Errors, " "), "человеко-часы: Слишком много знаков") {
		t.Fatal(err, report.Errors)
	}
	if workQuantityThousandths(models.WorkOrderTask{QuantityHundredths: 125}) != 1250 {
		t.Fatal("legacy quantity changed")
	}
}

// --------- новый код chat: удалённая карточка не доступна даже через историю смены
func TestWorkJournalExcludesDeletedEmployee(t *testing.T) {
	f := newWorkFixture(t)
	file := journalFixtureFile(t, f)
	data := excelBytes(t, file)
	report := previewExcel(t, f, data)
	confirmExcel(t, f, data, report.Token)
	if err := f.db.Delete(&f.employee).Error; err != nil {
		t.Fatal(err)
	}
	report, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(report.Errors, " "), "найдено совпадений 0") {
		t.Fatalf("deleted employee found through history: %v", report.Errors)
	}
}

// --------- конец нового кода chat

// --------- конец нового кода chat
