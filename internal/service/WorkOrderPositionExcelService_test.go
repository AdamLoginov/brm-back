// --------- новый код chat: сводка всех позиций, точные суммы и пропуски
package service

import (
	"bytes"
	"github.com/xuri/excelize/v2"
	"strings"
	"testing"
)

func TestWorkPositionExcel(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].Materials[0].Quantity = "1.125"
	second := in.Tasks[0]
	second.Name = " монтаж "
	second.Quantity = ""
	second.Hours = ""
	second.Materials = []WorkMaterialInput{{MaterialID: f.material.ID, Quantity: "2.250"}}
	otherUnit := second
	otherUnit.UnitID = 0
	otherUnit.Materials = nil
	noPosition := otherUnit
	noPosition.PositionID = 0
	noPosition.Name = "Без привязки"
	in.Tasks = append(in.Tasks, second, otherUnit, noPosition)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	file, err := BuildWorkOrderPositionExcel(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	opened, err := excelize.OpenReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	rows, err := opened.GetRows("Работы по позициям")
	if err != nil {
		t.Fatal(err)
	}
	found, unassigned := false, false
	// новый код chat: первая строка — заголовки, итогов и цветных выделений нет.
	if len(rows) == 0 || len(rows[0]) < 2 || rows[0][1] != "Наименование работы" {
		t.Fatal("missing column headings", rows)
	}
	for index, row := range rows {
		if len(row)>0 && strings.HasPrefix(row[0],"Позиция ") && row[0]!="Позиция № 1013 Объект / Блок / Цех 1" { t.Fatal("wrong section title",row[0]) } // новый код chat
		if strings.Contains(strings.Join(row, " "), "Итого по позиции") {
			t.Fatal("unexpected totals")
		}
		if len(row) > 0 && row[0] == "Без позиции" {
			unassigned = true
		}
		if len(row) < 7 || row[2] != "2" {
			continue
		}
		found = true
		if row[3] != "2.50" || row[5] != "12.00" || !strings.Contains(row[6], "3,375") {
			t.Fatal("wrong sums", row)
		}
		for _, column := range []string{"D", "F"} {
			cell, _ := excelize.CoordinatesToCellName(map[string]int{"D": 4, "F": 6}[column], index+1)
			styleID, err := opened.GetCellStyle("Работы по позициям", cell)
			if err != nil {
				t.Fatal(err)
			}
			style, err := opened.GetStyle(styleID)
			if err != nil || style.Font == nil || style.Font.Color != "000000" || len(style.Border) != 4 {
				t.Fatal("expected black text and borders", style, err)
			}
		}
	}
	if !found || !unassigned {
		t.Fatal("missing grouped or unassigned work", rows)
	}
	if _, err := BuildWorkOrderPositionExcel(f.db, f.a.ID+999, f.book.ID); err == nil {
		t.Fatal("foreign agreement book exported")
	}
}

// --------- конец нового кода chat
