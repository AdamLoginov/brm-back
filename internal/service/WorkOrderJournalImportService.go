// --------- новый код chat: общий журнал с сотрудниками и работами на одном листе
package service

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"project/internal/models"
)

var journalNotes = regexp.MustCompile(`\([^)]*\)`)

func journalText(v string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(strings.ToLower(v), "ё", "е")), " ")
}
func journalRef(v, prefix string) string {
	v = journalText(v)
	v = strings.TrimSpace(strings.TrimPrefix(v, prefix))
	return strings.TrimSpace(strings.TrimPrefix(v, "№"))
}
func journalPerson(v string) string {
	words := strings.FieldsFunc(journalText(journalNotes.ReplaceAllString(v, "")), func(r rune) bool { return !unicode.IsLetter(r) })
	return strings.Join(words, "")
}
func journalLabel(v string) string {
	parts := strings.SplitN(v, " | ", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return v
}
func journalMatch(value string, candidates map[uint]string, normalize func(string) string) (uint, error) {
	var found uint
	for id, name := range candidates {
		if normalize(value) == normalize(name) {
			if found != 0 {
				return 0, fmt.Errorf("несколько совпадений для «%s»", value)
			}
			found = id
		}
	}
	if found == 0 {
		return 0, fmt.Errorf("не найдено «%s» в справочнике", value)
	}
	return found, nil
}

func parseWorkOrderJournal(tx *gorm.DB, data []byte, s workExcelState) (workExcelTables, []uint, []string) {
	var result workExcelTables
	selected := []uint{}
	problems := []string{}
	fail := func(row int, message string) {
		// новый код chat: возвращаем все замечания, чтобы счётчик не ограничивался сотней.
		problems = append(problems, fmt.Sprintf("Журнал, строка %d: %s", row, message))
	}
	file, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 8 << 20})
	if err != nil {
		return result, selected, []string{"Не удалось прочитать XLSX: " + err.Error()}
	}
	defer file.Close()
	sheet := ""
	for _, name := range file.GetSheetList() {
		a, _ := file.GetCellValue(name, "A1")
		b, _ := file.GetCellValue(name, "B1")
		h, _ := file.GetCellValue(name, "H1")
		if journalText(a) == "дата" && journalText(b) == "сотрудник" && (journalText(h) == "наменование" || journalText(h) == "наименование" || journalText(h) == "наименование работы") {
			if sheet != "" {
				return result, selected, []string{"Найдено несколько листов журнала. Оставьте один лист для загрузки."}
			}
			sheet = name
		}
	}
	if sheet == "" {
		return result, selected, []string{"Не найден общий журнал. Ожидаются колонки A–N: дата, сотрудник, часы, смена, цех, блок, позиция, работа, единица, количество, человеко-часы, материал, единица, количество."}
	}
	expected := map[string][]string{"C1": {"часы"}, "D1": {"смена"}, "E1": {"цех"}, "F1": {"блок"}, "G1": {"поз.", "поз", "позиция"}, "I1": {"ед изм", "ед. изм", "ед. изм."}, "J1": {"кол-во", "количество"}, "K1": {"чел-час", "чел.-ч", "человеко-часы"}, "L1": {"материалы", "материал"}, "M1": {"ед. изм", "ед. изм.", "ед изм"}, "N1": {"кол-во", "количество"}}
	for cell, allowed := range expected {
		value, _ := file.GetCellValue(sheet, cell)
		ok := false
		for _, label := range allowed {
			if journalText(value) == label {
				ok = true
			}
		}
		if !ok {
			return result, selected, []string{"Неожиданный заголовок " + cell + ": " + value}
		}
	}
	var shops []models.AgreementWorkshop
	var blocks []models.AgreementWorkBlock
	var positions []models.AgreementWorkPosition
	var units []models.MeasurementUnit
	for _, q := range []*gorm.DB{tx.Where("agreement_id = ?", s.Book.AgreementID).Find(&shops), tx.Where("agreement_id = ?", s.Book.AgreementID).Find(&blocks), tx.Where("agreement_id = ?", s.Book.AgreementID).Find(&positions), tx.Find(&units)} {
		if q.Error != nil {
			return result, selected, []string{q.Error.Error()}
		}
	}
	shopNames, unitNames := map[uint]string{}, map[uint]string{}
	for _, v := range shops {
		shopNames[v.ID] = v.Name
	}
	for _, v := range units {
		unitNames[v.ID] = v.Name
	}
	matchPerson := func(value string) (uint, error) {
		matches := map[uint]bool{}
		input := journalPerson(value)
		for id, label := range s.Refs["employees"] {
			name := journalLabel(label)
			words := strings.Fields(journalText(journalNotes.ReplaceAllString(name, "")))
			short := ""
			if len(words) > 0 {
				short = words[0]
				for _, w := range words[1:] {
					r := []rune(w)
					if len(r) > 0 {
						short += string(r[0])
					}
				}
			}
			if input == journalPerson(name) || input == journalPerson(short) {
				matches[workUint(id)] = true
			}
		}
		if len(matches) != 1 {
			return 0, fmt.Errorf("сотрудник «%s»: найдено совпадений %d; уточните ФИО и закрепление за договором", value, len(matches))
		}
		for id := range matches {
			return id, nil
		}
		return 0, nil
	}
	dates := map[uint]string{}
	for _, d := range s.Book.Days {
		dates[d.ID] = d.Date
	}
	shiftKeys := map[string]string{}
	oldEmployees := map[string]map[uint]string{}
	oldTasks := map[string][]WorkTaskDetail{}
	for _, shift := range s.Book.Shifts {
		key := workKey(shift.ID)
		shiftKeys[fmt.Sprintf("%s/%d/%d", dates[shift.DayID], shift.WorkshopID, shift.Number)] = key
		oldEmployees[key] = map[uint]string{}
		for _, e := range shift.Employees {
			oldEmployees[key][e.EmployeeCardID] = workKey(e.ID)
		}
		oldTasks[key] = shift.Tasks
	}
	represented := map[string]bool{}
	seenEmployees := map[string]string{}
	taskCount := map[string]int{}
	lastTask := map[string]string{}
	taskMaterials := map[string]map[uint]string{}
	seenMaterials := map[string]bool{} // новый код chat: повтор не должен молча заменять ранее прочитанную строку
	workbookProps, _ := file.GetWorkbookProps()
	date1904 := workbookProps.Date1904 != nil && *workbookProps.Date1904
	merges, err := file.GetMergeCells(sheet)
	if err != nil {
		return result, selected, []string{err.Error()}
	}
	masters := map[string]string{}
	for _, merge := range merges {
		c1, r1, e1 := excelize.CellNameToCoordinates(merge.GetStartAxis())
		c2, r2, e2 := excelize.CellNameToCoordinates(merge.GetEndAxis())
		if e1 != nil || e2 != nil || r2 > 10000 || c2 > 14 {
			return result, selected, []string{"Некорректный или слишком большой объединённый диапазон"}
		}
		for c := c1; c <= c2; c++ {
			if c != 1 && c != 4 && c != 5 {
				continue
			}
			for r := r1; r <= r2; r++ {
				cell, _ := excelize.CoordinatesToCellName(c, r)
				masters[cell] = merge.GetStartAxis()
			}
		}
	}
	rows, err := file.Rows(sheet)
	if err != nil {
		return result, selected, []string{err.Error()}
	}
	defer rows.Close()
	rowNumber := 0
	for rows.Next() {
		rowNumber++
		if rowNumber > 10000 {
			return result, selected, []string{"Максимум 10 000 строк журнала"}
		}
		if rowNumber <= 2 { // новый код chat: обе первые строки — заголовок журнала
			continue
		}
		raw, e := rows.Columns(excelize.Options{RawCellValue: true})
		if e != nil {
			fail(rowNumber, e.Error())
			break
		}
		v := make([]string, 14)
		copy(v, raw)
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		if strings.TrimSpace(strings.Join(append(append([]string{}, v[1:3]...), v[5:]...), "")) == "" {
			continue
		}
		if len(raw) > 14 {
			fail(rowNumber, "есть данные за пределами колонок A–N")
		}
		for i := range v {
			cell, _ := excelize.CoordinatesToCellName(i+1, rowNumber)
			if master := masters[cell]; master != "" {
				cell = master
				v[i], _ = file.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
			}
			formula, _ := file.GetCellFormula(sheet, cell)
			if formula != "" {
				fail(rowNumber, "формула в "+cell+"; вставьте её значение")
			}
		}
		date := time.Time{}
		for _, layout := range []string{"2006-01-02", "02.01.2006"} {
			if d, e := time.Parse(layout, v[0]); e == nil {
				date = d
				break
			}
		}
		if date.IsZero() {
			if n, e := strconv.ParseFloat(v[0], 64); e == nil && n > 0 && n == float64(int64(n)) {
				date, _ = excelize.ExcelDateToTime(n, date1904)
			}
		}
		if date.IsZero() || date.Year() != s.Book.Year || int(date.Month()) != s.Book.Month {
			fail(rowNumber, "дата отсутствует или не относится к месяцу выбранной книги")
			continue
		}
		if v[3] != "1" && v[3] != "2" {
			fail(rowNumber, "укажите смену 1 или 2")
			continue
		}
		if v[4] == "" {
			fail(rowNumber, "не указан цех")
			continue
		}
		shop, e := journalMatch(v[4], shopNames, func(v string) string { return journalRef(v, "цех") })
		if e != nil {
			fail(rowNumber, "цех: "+e.Error())
			continue
		}
		group := fmt.Sprintf("%s/%d/%s", date.Format("2006-01-02"), shop, v[3])
		key := shiftKeys[group]
		if key == "" {
			key = fmt.Sprintf("new:journal-shift-%d", rowNumber)
			shiftKeys[group] = key
		}
		if !represented[key] {
			represented[key] = true
			result[0] = append(result[0], []string{key, date.Format("2006-01-02"), workKey(shop), v[3]})
			if !strings.HasPrefix(key, "new:") {
				selected = append(selected, workUint(key))
			}
		}
		if v[1] != "" {
			id, e := matchPerson(v[1])
			if e != nil {
				fail(rowNumber, e.Error())
			} else {
				hours, e := ParseWorkQuantity(v[2], 2, false)
				if e != nil {
					fail(rowNumber, "заполните корректные часы сотрудника «"+v[1]+"»")
				} else {
					h := workDecimal(hours, 2)
					ek := key + "/" + workKey(id)
					if previous, ok := seenEmployees[ek]; ok {
						if previous != h {
							fail(rowNumber, "сотрудник повторён с разными часами")
						}
					} else {
						seenEmployees[ek] = h
						rowKey := oldEmployees[key][id]
						if rowKey == "" {
							rowKey = fmt.Sprintf("new:journal-employee-%d", rowNumber)
						}
						result[1] = append(result[1], []string{rowKey, key, workKey(id), h})
					}
				}
			}
		}
		if v[1] == "" && v[2] != "" {
			fail(rowNumber, "часы сотрудника указаны без ФИО")
		}
		if v[7] != "" {
			blockNames := map[uint]string{}
			for _, b := range blocks {
				if b.WorkshopID == shop {
					blockNames[b.ID] = b.Name
				}
			}
			block := uint(0)
			if v[5] != "" {
				block, e = journalMatch(v[5], blockNames, func(v string) string { return journalRef(v, "блок") })
				if e != nil {
					fail(rowNumber, "блок: "+e.Error())
				}
			}
			position := uint(0)
			if v[6] != "" {
				names := map[uint]string{}
				for _, p := range positions {
					if _, ok := blockNames[p.BlockID]; ok && (block == 0 || block == p.BlockID) {
						names[p.ID] = p.Number
					}
				}
				position, e = journalMatch(v[6], names, func(v string) string { return journalRef(v, "") })
				if e != nil {
					fail(rowNumber, "позиция: "+e.Error())
				} else {
					for _, p := range positions {
						if p.ID == position {
							block = p.BlockID
						}
					}
				}
			}
			unit := ""
			if v[8] != "" {
				id, e := journalMatch(v[8], unitNames, journalText)
				if e != nil {
					fail(rowNumber, "единица работы: "+e.Error())
				} else {
					unit = workKey(id)
				}
			}
			quantity, hours := "", ""
			for _, pair := range []struct {
				index int
				dest  *string
				zero  bool
			}{{9, &quantity, false}, {10, &hours, true}} {
				if v[pair.index] != "" {
					n, e := ParseWorkQuantity(v[pair.index], 2, pair.zero)
					if e != nil {
						fail(rowNumber, e.Error())
					} else {
						*pair.dest = workDecimal(n, 2)
					}
				}
			}
			if len(v[7]) > 2000 {
				fail(rowNumber, "слишком длинное наименование работы")
			}
			taskKey := fmt.Sprintf("new:journal-task-%05d", rowNumber) // новый код chat: сохраняем порядок строк Excel
			index := taskCount[key]
			taskCount[key]++
			if index < len(oldTasks[key]) {
				taskKey = workKey(oldTasks[key][index].ID)
				taskMaterials[taskKey] = map[uint]string{}
				for _, m := range oldTasks[key][index].Materials {
					taskMaterials[taskKey][m.MaterialID] = workKey(m.ID)
				}
			}
			b, p := "", ""
			if block != 0 {
				b = workKey(block)
			}
			if position != 0 {
				p = workKey(position)
			}
			// новый код chat: отметка сайта сохраняется у сопоставленной записи при повторном импорте.
			accounted := "false"
			if index < len(oldTasks[key]) && oldTasks[key][index].Accounted {
				accounted = "true"
			}
			result[2] = append(result[2], []string{taskKey, key, v[7], unit, quantity, hours, b, p, accounted})
			lastTask[key] = taskKey
		} else if strings.TrimSpace(strings.Join(append(append([]string{}, v[5:7]...), v[8:11]...), "")) != "" {
			fail(rowNumber, "поля работы заполнены без наименования")
		}
		if v[11] != "" {
			taskKey := lastTask[key]
			if taskKey == "" {
				fail(rowNumber, "материал указан без работы в этой смене")
				continue
			}
			names := map[uint]string{}
			for _, m := range s.Stock {
				if m.WorkshopID == shop {
					names[m.ID] = m.Name
				}
			}
			id, e := journalMatch(v[11], names, journalText)
			if e != nil {
				fail(rowNumber, "материал: "+e.Error())
				continue
			}
			materialKey := taskKey + "/" + workKey(id)
			if seenMaterials[materialKey] {
				fail(rowNumber, "материал повторён в одной работе; объедините количество в одну строку")
				continue
			}
			seenMaterials[materialKey] = true
			for _, m := range s.Stock {
				if m.ID == id && journalText(v[12]) != journalText(unitNames[m.UnitID]) {
					fail(rowNumber, "единица материала не совпадает со складом")
				}
			}
			n, e := ParseWorkQuantity(v[13], 3, false)
			if e != nil {
				fail(rowNumber, "количество материала: "+e.Error())
				continue
			}
			mk := taskMaterials[taskKey][id]
			if mk == "" {
				mk = fmt.Sprintf("new:journal-material-%d", rowNumber)
			}
			result[3] = append(result[3], []string{mk, taskKey, workKey(id), workDecimal(n, 3)})
		} else if v[12] != "" || v[13] != "" {
			fail(rowNumber, "количество или единица материала указаны без материала")
		}
	}
	if e := rows.Error(); e != nil {
		problems = append(problems, e.Error())
	}
	if len(result[0]) == 0 && len(problems) == 0 {
		problems = append(problems, "В журнале нет данных для загрузки")
	}
	return result, selected, problems
}

// --------- конец нового кода chat
