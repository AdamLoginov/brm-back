// --------- новый код chat: месячная сводка работ по позициям в одной таблице Excel
package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type positionExcelGroup struct {
	accounted       bool // новый код chat
	name, unit      string
	count           int
	quantity, hours int64
	materials       map[string]int64
}

func BuildWorkOrderPositionExcel(db *gorm.DB, agreementID, bookID uint) (*excelize.File, error) {
	var state workExcelState
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		state, err = loadWorkExcelState(tx, agreementID, bookID)
		return err
	})
	if err != nil {
		return nil, err
	}
	label := func(kind, id string) string {
		value := state.Refs[kind][id]
		if _, name, ok := strings.Cut(value, " | "); ok {
			return name
		}
		if id == "" || id == "0" {
			return "—"
		}
		return "№ " + id
	}
	sections := map[string]map[string]*positionExcelGroup{}
	for _, shift := range state.Book.Shifts {
		for _, task := range shift.Tasks {
			position := workKey(task.PositionID)
			if sections[position] == nil {
				sections[position] = map[string]*positionExcelGroup{}
			}
			key := fmt.Sprintf("%s\x00%d", strings.ToLower(strings.Join(strings.Fields(task.Name), " ")), task.UnitID)
			if task.Accounted {
				key = fmt.Sprintf("accounted:%d", task.ID)
			} // новый код chat: индивидуальная строка
			group := sections[position][key]
			if group == nil {
				group = &positionExcelGroup{accounted: task.Accounted, name: task.Name, unit: workKey(task.UnitID), materials: map[string]int64{}} // новый код chat
				sections[position][key] = group
			}
			group.count++
			group.quantity += workQuantityThousandths(task.WorkOrderTask) // новый код chat
			if !task.HoursNotSpecified {
				group.hours += task.HoursHundredths
			}
			for _, usage := range task.Materials {
				group.materials[workKey(usage.MaterialID)] += usage.QuantityThousandths
			}
		}
	}
	materialUnits := map[string]string{}
	for _, material := range state.Stock {
		materialUnits[workKey(material.ID)] = label("units", workKey(material.UnitID))
	}
	materialText := func(totals map[string]int64) string {
		keys := []string{}
		for key := range totals {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return label("materials", keys[i]) < label("materials", keys[j]) })
		lines := []string{}
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s — %s %s", label("materials", key), strings.ReplaceAll(workDecimal(totals[key], 3), ".", ","), materialUnits[key]))
		}
		if len(lines) == 0 {
			return "—"
		}
		return strings.Join(lines, "\n")
	}
	f := excelize.NewFile()
	const sheet = "Работы по позициям"
	var writeErr error
	check := func(err error) {
		if writeErr == nil {
			writeErr = err
		}
	}
	check(f.SetSheetName("Sheet1", sheet))
	style := func(value *excelize.Style) int { id, err := f.NewStyle(value); check(err); return id }
	// новый код chat: чёрный текст, белый фон и тонкие границы всех ячеек.
	borders := []excelize.Border{{Type: "left", Color: "000000", Style: 1}, {Type: "right", Color: "000000", Style: 1}, {Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 1}}
	base := style(&excelize.Style{Border: borders, Font: &excelize.Font{Size: 11, Color: "000000"}, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	header := style(&excelize.Style{Border: borders, Font: &excelize.Font{Bold: true, Size: 11, Color: "000000"}, Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	sectionStyle := header
	format := "#,##0.00#" // новый код chat: прежние два знака и третий при необходимости
	number := style(&excelize.Style{Border: borders, CustomNumFmt: &format, Font: &excelize.Font{Size: 11, Color: "000000"}, Alignment: &excelize.Alignment{Vertical: "top"}})
	// новый код chat: только учтённые работы выделены бледно-красным.
	accountedStyle := style(&excelize.Style{Border: borders, Font: &excelize.Font{Size: 11, Color: "000000"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"FCE8E8"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	accountedNumber := style(&excelize.Style{Border: borders, CustomNumFmt: &format, Font: &excelize.Font{Size: 11, Color: "000000"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"FCE8E8"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "top"}})
	write := func(row int, values []interface{}) { check(f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values)) }
	merge := func(row int, text string, format int) {
		write(row, []interface{}{text})
		check(f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row)))
		check(f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), format))
	}
	// новый код chat: таблица начинается сразу с названий колонок.
	write(1, []interface{}{"№", "Наименование работы", "Кол-во записей", "Объём", "Ед.", "Чел.-ч", "Материалы"})
	check(f.SetCellStyle(sheet, "A1", "G1", header))
	keys := []string{}
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return label("positions", keys[i]) < label("positions", keys[j]) })
	row := 2
	// новый код chat: позиция, блок, цех — без внутренних ID справочников.
	refIDs := regexp.MustCompile(`\d+ \| `)
	for _, position := range keys {
		title := "Позиция " + refIDs.ReplaceAllString(label("positions", position), "")
		if position == "0" {
			title = "Без позиции"
		}
		merge(row, title, sectionStyle)
		check(f.SetRowHeight(sheet, row, 36))
		row++
		groups := sections[position]
		names := []string{}
		for key := range groups {
			names = append(names, key)
		}
		sort.Slice(names, func(i, j int) bool {
			if groups[names[i]].accounted != groups[names[j]].accounted {
				return !groups[names[i]].accounted
			}
			return names[i] < names[j]
		}) // новый код chat
		for index, key := range names {
			group := groups[key]
			text := materialText(group.materials)
			write(row, []interface{}{index + 1, group.name, group.count, float64(group.quantity) / 1000, label("units", group.unit), float64(group.hours) / 100, text})
			rowStyle, numberStyle := base, number
			if group.accounted {
				rowStyle, numberStyle = accountedStyle, accountedNumber
			} // новый код chat
			check(f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), rowStyle))
			for _, column := range []string{"D", "F"} {
				address := fmt.Sprintf("%s%d", column, row)
				check(f.SetCellStyle(sheet, address, address, numberStyle)) // новый код chat
			}
			lines := len([]rune(group.name))/48 + 1
			materialLines := 0
			for _, line := range strings.Split(text, "\n") {
				materialLines += len([]rune(line))/55 + 1
			}
			if materialLines > lines {
				lines = materialLines
			}
			height := float64(lines*16 + 10)
			if height > 409 {
				height = 409
			}
			check(f.SetRowHeight(sheet, row, height))
			row++
		}
	}
	if len(keys) == 0 {
		merge(2, "В книге пока нет работ.", base)
	}
	for _, col := range []struct {
		name  string
		width float64
	}{{"A", 6}, {"B", 52}, {"C", 17}, {"D", 14}, {"E", 10}, {"F", 14}, {"G", 65}} {
		check(f.SetColWidth(sheet, col.name, col.name, col.width))
	}
	check(f.SetRowHeight(sheet, 1, 30))
	check(f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}))
	if writeErr != nil {
		f.Close()
		return nil, writeErr
	}
	return f, nil
}

// --------- конец нового кода chat
