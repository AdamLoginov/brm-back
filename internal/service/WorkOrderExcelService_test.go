// --------- новый код chat: проверка реальных XLSX, без изменения рабочей базы
package service

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func excelBytes(t *testing.T, f *excelize.File) []byte {
	t.Helper()
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func fixtureExcel(t *testing.T, f workFixture, template bool) *excelize.File {
	t.Helper()
	file, err := BuildWorkOrderExcel(f.db, f.a.ID, f.book.ID, template)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	return file
}
func fixtureState(t *testing.T, f workFixture) workExcelState {
	t.Helper()
	s, err := loadWorkExcelState(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func excelCell(t *testing.T, f *excelize.File, sheet, cell, value string) {
	t.Helper()
	if err := f.SetCellStr(sheet, cell, value); err != nil {
		t.Fatal(err)
	}
}

func TestWorkExcelRoundTripAndTemplate(t *testing.T) {
	f := newWorkFixture(t)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input()); err != nil {
		t.Fatal(err)
	}
	s := fixtureState(t, f)
	for _, template := range []bool{false, true} {
		file := fixtureExcel(t, f, template)
		rows, problems := parseWorkOrderExcel(excelBytes(t, file), s)
		if len(problems) > 0 {
			t.Fatal(problems)
		}
		if template {
			for _, table := range rows {
				if len(table) != 0 {
					t.Fatal("template contains live records")
				}
			}
		} else {
			for i := range rows {
				if len(rows[i]) != len(s.Tables[i]) {
					t.Fatal("row count", i)
				}
				for j := range rows[i] {
					if !workRowEqual(rows[i][j], s.Tables[i][j]) {
						t.Fatal("round trip", i, j)
					}
				}
			}
		}
		panes, err := file.GetPanes("Работы")
		if err != nil || !panes.Freeze {
			t.Fatal("freeze", err)
		}
		validations, err := file.GetDataValidations("Работы")
		if err != nil || len(validations) != 3 {
			t.Fatal("reference dropdowns", err, len(validations))
		}
		visible, err := file.GetSheetVisible("_Служебное")
		if err != nil || visible {
			t.Fatal("metadata must be hidden")
		}
	}
}
func TestWorkExcelReportsCellErrors(t *testing.T) {
	f := newWorkFixture(t)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input()); err != nil {
		t.Fatal(err)
	}
	s := fixtureState(t, f)
	for _, test := range []struct{ name, sheet, cell, value, want string }{
		{"unknown reference", "Сотрудники", "C2", "999 | Чужой", "Сотрудники!C2"},
		{"precision", "Материалы работ", "D2", "1.1234", "Материалы работ!D2"},
		{"other month", "Смены", "B2", "2028-03-01", "Смены!B2"},
		{"unknown key", "Работы", "A2", "bad-key", "Работы!A2"},
		{"negative", "Работы", "E2", "-1", "Работы!E2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := fixtureExcel(t, f, false)
			excelCell(t, file, test.sheet, test.cell, test.value)
			_, problems := parseWorkOrderExcel(excelBytes(t, file), s)
			if !strings.Contains(strings.Join(problems, " "), test.want) {
				t.Fatal(problems)
			}
		})
	}
	file := fixtureExcel(t, f, false)
	// Формат ячейки не должен скрывать лишнюю точность исходного числа.
	numericStyle, err := file.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		t.Fatal(err)
	}
	file.SetCellFloat("Работы", "E2", 1.234, 3, 64)
	file.SetCellStyle("Работы", "E2", "E2", numericStyle)
	_, numericProblems := parseWorkOrderExcel(excelBytes(t, file), s)
	if !strings.Contains(strings.Join(numericProblems, " "), "знаков после запятой") {
		t.Fatal(numericProblems)
	}
	file = fixtureExcel(t, f, false)
	if err := file.SetCellFormula("Работы", "E2", "1+2"); err != nil {
		t.Fatal(err)
	}
	_, problems := parseWorkOrderExcel(excelBytes(t, file), s)
	if !strings.Contains(strings.Join(problems, " "), "формулы") {
		t.Fatal(problems)
	}
	file = fixtureExcel(t, f, false)
	for col := 1; col <= 4; col++ {
		a, _ := excelize.CoordinatesToCellName(col, 2)
		b, _ := excelize.CoordinatesToCellName(col, 3)
		v, _ := file.GetCellValue("Смены", a)
		excelCell(t, file, "Смены", b, v)
	}
	_, problems = parseWorkOrderExcel(excelBytes(t, file), s)
	if !strings.Contains(strings.Join(problems, " "), "повтор ключа") {
		t.Fatal(problems)
	}
}

// --------- конец нового кода chat
