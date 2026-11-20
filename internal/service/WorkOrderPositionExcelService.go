// --------- новый код chat: месячная сводка работ по позициям в одной таблице Excel
package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type positionExcelGroup struct {
	name, unit                    string
	count                         int
	quantity, hours               int64
	missingQuantity, missingHours bool
	materials                     map[string]int64
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
			group := sections[position][key]
			if group == nil {
				group = &positionExcelGroup{name: task.Name, unit: workKey(task.UnitID), materials: map[string]int64{}}
				sections[position][key] = group
			}
			group.count++
			group.quantity += task.QuantityHundredths
			group.missingQuantity = group.missingQuantity || task.QuantityHundredths == 0
			group.missingHours = group.missingHours || task.HoursNotSpecified
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
	base := style(&excelize.Style{Font: &excelize.Font{Size: 11}, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	header := style(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11}, Fill: excelize.Fill{Type: "pattern", Color: []string{"24476B"}, Pattern: 1}, Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	sectionStyle := style(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 12}, Fill: excelize.Fill{Type: "pattern", Color: []string{"E2EDF7"}, Pattern: 1}, Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	format := "#,##0.00"
	number := style(&excelize.Style{CustomNumFmt: &format, Alignment: &excelize.Alignment{Vertical: "top"}})
	red := style(&excelize.Style{CustomNumFmt: &format, Font: &excelize.Font{Color: "DC3545"}, Alignment: &excelize.Alignment{Vertical: "top"}})
	write := func(row int, values []interface{}) { check(f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values)) }
	merge := func(row int, text string, format int) {
		write(row, []interface{}{text})
		check(f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row)))
		check(f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), format))
	}
	merge(1, fmt.Sprintf("Работы по позициям · %02d.%d · договор № %d", state.Book.Month, state.Book.Year, agreementID), header)
	merge(2, "Красные суммы содержат незаполненные значения, принятые за 0. Работы с разными единицами не объединяются.", base)
	write(3, []interface{}{"№", "Наименование работы", "Кол-во записей", "Объём", "Ед.", "Чел.-ч", "Материалы"})
	check(f.SetCellStyle(sheet, "A3", "G3", header))
	keys := []string{}
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return label("positions", keys[i]) < label("positions", keys[j]) })
	row := 4
	for _, position := range keys {
		title := "Позиция " + label("positions", position)
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
		sort.Strings(names)
		totalHours := int64(0)
		missing := false
		totals := map[string]int64{}
		for index, key := range names {
			group := groups[key]
			text := materialText(group.materials)
			write(row, []interface{}{index + 1, group.name, group.count, float64(group.quantity) / 100, label("units", group.unit), float64(group.hours) / 100, text})
			check(f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), base))
			for _, cell := range []struct {
				column  string
				missing bool
			}{{"D", group.missingQuantity}, {"F", group.missingHours}} {
				id := number
				if cell.missing {
					id = red
				}
				address := fmt.Sprintf("%s%d", cell.column, row)
				check(f.SetCellStyle(sheet, address, address, id))
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
			totalHours += group.hours
			missing = missing || group.missingHours
			for id, quantity := range group.materials {
				totals[id] += quantity
			}
			row++
		}
		write(row, []interface{}{"", "Итого по позиции", "", "", "", float64(totalHours) / 100, materialText(totals)})
		check(f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("G%d", row), sectionStyle))
		id := number
		if missing {
			id = red
		}
		check(f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), id))
		height := float64((strings.Count(materialText(totals), "\n")+1)*32 + 10)
		if height > 409 {
			height = 409
		}
		check(f.SetRowHeight(sheet, row, height))
		row += 2
	}
	if len(keys) == 0 {
		merge(4, "В книге пока нет работ.", base)
	}
	for _, col := range []struct {
		name  string
		width float64
	}{{"A", 6}, {"B", 52}, {"C", 17}, {"D", 14}, {"E", 10}, {"F", 14}, {"G", 65}} {
		check(f.SetColWidth(sheet, col.name, col.name, col.width))
	}
	check(f.SetRowHeight(sheet, 1, 30))
	check(f.SetRowHeight(sheet, 2, 32))
	check(f.SetRowHeight(sheet, 3, 30))
	check(f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 3, TopLeftCell: "A4", ActivePane: "bottomLeft"}))
	if writeErr != nil {
		f.Close()
		return nil, writeErr
	}
	return f, nil
}

// --------- конец нового кода chat
