// --------- новый код chat: заполнение ведомости, исключение учтённых и сохранение шаблона
package service

import (
	"archive/zip" // новый код chat
	"bytes"
	"fmt"
	"github.com/xuri/excelize/v2"
	"project/internal/models" // новый код chat
	"strings"
	"testing"
)

// новый код chat: отдельные файлы ZIP, одинаковые номера в разных блоках, дробные часы.
func TestAllWorkPositionStatements(t *testing.T) {
	f := newWorkFixture(t)
	if _, err := BuildAllWorkOrderPositionStatements(f.db, f.a.ID, f.book.ID); err == nil {
		t.Fatal("empty export accepted")
	}
	b := models.AgreementWorkBlock{AgreementID: f.a.ID, WorkshopID: f.shop.ID, Name: "Блок 2", NameKey: "блок 2"}
	f.db.Create(&b)
	p := models.AgreementWorkPosition{AgreementID: f.a.ID, BlockID: b.ID, Number: f.position.Number, Name: "Вторая позиция"}
	f.db.Create(&p)
	in := f.input()
	in.Tasks[0].Materials = nil
	second := in.Tasks[0]
	second.PositionID = p.ID
	second.Name = "Вторая работа"
	second.Hours = "1.25"
	marked := in.Tasks[0]
	marked.Name = "Не выгружать"
	marked.Accounted = true
	in.Tasks = append(in.Tasks, second, marked)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	fout, err := BuildAllWorkOrderPositionStatements(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(fout), int64(len(fout)))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		t.Fatal("wrong file count", len(archive.File))
	}
	for i, entry := range archive.File {
		if i == 1 && entry.Name != "Ведомость объемов работ Б2 поз.1013 Вторая позиция.xlsx" {
			t.Fatal(entry.Name)
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		file, err := excelize.OpenReader(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if len(file.GetSheetList()) != 1 {
			t.Fatal("expected one sheet")
		}
		sheet := file.GetSheetName(0)
		name, _ := file.GetCellValue(sheet, "B14")
		hours, _ := file.GetCellValue(sheet, "E14")
		if i == 0 && (name != "Монтаж" || hours != "12 чел. час") {
			t.Fatal(name, hours)
		}
		if i == 1 && (name != "Вторая работа" || hours != "1,25 чел. час") {
			t.Fatal(name, hours)
		}
		signature, _ := file.GetCellValue(sheet, "F26")
		if signature != "Р.А. Петров " {
			t.Fatal("signature missing", signature)
		}
		rows, _ := file.GetRows(sheet)
		for _, row := range rows {
			if strings.Contains(strings.Join(row, " "), "Не выгружать") {
				t.Fatal("accounted included")
			}
		}
		merges, _ := file.GetMergeCells(sheet)
		if len(merges) != 5 {
			t.Fatal("lost template merges", merges)
		}
		area, titles := false, false
		for _, defined := range file.GetDefinedName() {
			if defined.Scope == sheet {
				area = area || defined.Name == "_xlnm.Print_Area"
				titles = titles || defined.Name == "_xlnm.Print_Titles"
			}
		}
		if !area || !titles {
			t.Fatal("missing print settings", sheet, file.GetDefinedName())
		}
	}
}

// новый код chat: ведомость «Нет» содержит только неучтённые работы выбранного блока без позиции.
func TestWorkBlockStatement(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].Materials = nil
	noPosition := in.Tasks[0]
	noPosition.PositionID = 0
	noPosition.BlockID = f.position.BlockID
	noPosition.Name = "Без позиции в блоке"
	unassigned := noPosition
	unassigned.BlockID = 0
	unassigned.Name = "Без блока"
	marked := noPosition
	marked.Accounted = true
	marked.Name = "Учтённая"
	in.Tasks = append(in.Tasks, noPosition, unassigned, marked)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	file, err := BuildWorkOrderBlockStatement(f.db, f.a.ID, f.book.ID, f.position.BlockID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sheet := file.GetSheetName(0)
	title, _ := file.GetCellValue(sheet, "A10")
	if strings.Contains(title, "Поз.") || strings.Contains(title, "Объект") || !strings.HasSuffix(title, "Блок.") {
		t.Fatal(title)
	}
	name, _ := file.GetCellValue(sheet, "B14")
	if name != noPosition.Name {
		t.Fatal(name)
	}
	name, _ = file.GetCellValue(sheet, "B15")
	if name != "" {
		t.Fatal("unexpected work", name)
	}
	if _, err := BuildWorkOrderBlockStatement(f.db, f.a.ID, f.book.ID, 99999); err == nil {
		t.Fatal("unknown block accepted")
	}
}

func TestWorkPositionStatement(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].Materials = nil
	second := in.Tasks[0]
	second.Quantity = "1.25"
	second.Hours = ""
	marked := second
	marked.Accounted = true
	marked.Name = "Учтённая работа"
	in.Tasks = append(in.Tasks, second, marked)
	for i := 0; i < 10; i++ {
		task := second
		task.Name = fmt.Sprintf("Работа %02d", i)
		in.Tasks = append(in.Tasks, task)
	}
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	file, err := BuildWorkOrderPositionStatement(f.db, f.a.ID, f.book.ID, f.position.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	check, err := excelize.OpenReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	sheet := check.GetSheetName(0)
	value := func(cell string) string {
		v, e := check.GetCellValue(sheet, cell)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	if v := value("A9"); v != "ВЕДОМОСТЬ ОБЪЕМОВ РАБОТ от 29.02.2028г." {
		t.Fatal(v)
	}
	if v := value("A10"); v != `ГМЗ ПАО "ППГХО". Зд. 622. Цех № 1. Блок. Объект. Поз. № 1013.` {
		t.Fatal(v)
	}
	if value("B14") != "Монтаж" || value("D14") != "3.75" || value("E14") != "12 чел. час" {
		t.Fatal("wrong totals", value("B14"), value("D14"), value("E14"))
	}
	for row := 14; row <= 24; row++ {
		if value(fmt.Sprintf("F%d", row)) != "" {
			t.Fatal("materials exported")
		}
	}
	if value("F28") != "Р.А. Петров " || value("F31") != "М.Г. Смирнов" {
		t.Fatal("signatures moved incorrectly", value("F28"), value("F31"))
	}
	rows, _ := check.GetRows(sheet)
	for _, row := range rows {
		if strings.Contains(strings.Join(row, " "), "Учтённая работа") {
			t.Fatal("accounted work exported")
		}
	}
	if _, err := BuildWorkOrderPositionStatement(f.db, f.a.ID+99, f.book.ID, f.position.ID); err == nil {
		t.Fatal("foreign book")
	}
	if _, err := BuildWorkOrderPositionStatement(f.db, f.a.ID, f.book.ID, 0); err == nil {
		t.Fatal("virtual position")
	}
}
