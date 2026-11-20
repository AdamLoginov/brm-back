// --------- новый код chat: единый формат шаблона, экспорта и проверки Excel книги нарядов
package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"project/internal/models"
)

type workExcelColumn struct{ Name, Kind string }
type workExcelSheet struct {
	Name    string
	Columns []workExcelColumn
}

var workExcelSheets = []workExcelSheet{
	{"Смены", []workExcelColumn{{"Ключ смены", "key"}, {"Дата", "date"}, {"Цех", "shops"}, {"Смена (1 или 2)", "shift"}}},
	{"Сотрудники", []workExcelColumn{{"Ключ строки", "key"}, {"Ключ смены", "parent"}, {"Сотрудник", "employees"}, {"Часы", "q2"}}},
	{"Работы", []workExcelColumn{{"Ключ работы", "key"}, {"Ключ смены", "parent"}, {"Наименование работы", "text"}, {"Единица", "units"}, {"Количество", "q2"}, {"Человеко-часы", "hours"}, {"Блок", "blocks"}, {"Позиция", "positions"}}},
	{"Материалы работ", []workExcelColumn{{"Ключ строки", "key"}, {"Ключ работы", "parent"}, {"Материал", "materials"}, {"Количество", "q3"}}},
	{"Поступления материалов", []workExcelColumn{{"Ключ поступления", "key"}, {"Дата", "date"}, {"Материал", "materials"}, {"Количество", "q3"}}},
}

type workExcelTables [5][][]string
type workExcelState struct {
	Book       WorkBookDetail
	Tables     workExcelTables
	Refs       map[string]map[string]string
	Stock      []WarehouseRow
	Receipts   []models.WarehouseReceipt
	Usages     []models.WorkOrderMaterial
	DailyHours []models.WorkOrderEmployee
}

func workKey(id uint) string { return strconv.FormatUint(uint64(id), 10) }
func workDecimal(n int64, digits int) string {
	scale := int64(100)
	if digits == 3 {
		scale = 1000
	}
	return fmt.Sprintf("%d.%0*d", n/scale, digits, n%scale)
}
func loadWorkExcelState(tx *gorm.DB, agreementID, bookID uint) (workExcelState, error) {
	s := workExcelState{Refs: map[string]map[string]string{}}
	var err error
	s.Book, err = LoadWorkBook(tx, agreementID, bookID)
	if err != nil {
		return s, err
	}
	s.Stock, err = LoadWarehouse(tx, agreementID)
	if err != nil {
		return s, err
	}
	materials := tx.Model(&models.WarehouseMaterial{}).Select("id").Where("agreement_id = ?", agreementID)
	for _, q := range []*gorm.DB{
		tx.Where("material_id IN (?)", materials).Order("id").Find(&s.Receipts),
		tx.Where("material_id IN (?)", materials).Order("id").Find(&s.Usages),
		tx.Where("shift_id IN (?)", tx.Model(&models.WorkOrderShift{}).Select("id").Where("day_id IN (?)", tx.Model(&models.WorkOrderDay{}).Select("id").Where("date >= ? AND date <= ?", s.Book.Days[0].Date, s.Book.Days[len(s.Book.Days)-1].Date))).Order("id").Find(&s.DailyHours),
	} {
		if q.Error != nil {
			return s, q.Error
		}
	}
	var shops []models.AgreementWorkshop
	var blocks []models.AgreementWorkBlock
	var positions []models.AgreementWorkPosition
	var units []models.MeasurementUnit
	var employees []models.EmployeeCard
	for _, q := range []*gorm.DB{
		tx.Where("agreement_id = ?", agreementID).Order("id").Find(&shops),
		tx.Where("agreement_id = ?", agreementID).Order("id").Find(&blocks),
		tx.Where("agreement_id = ?", agreementID).Order("id").Find(&positions),
		tx.Order("id").Find(&units),
		tx.Select("id", "name", "surname", "middle_name").Where("agreement_id = ?", agreementID).Order("id").Find(&employees),
	} {
		if q.Error != nil {
			return s, q.Error
		}
	}
	add := func(kind string, id uint, name string) {
		if s.Refs[kind] == nil {
			s.Refs[kind] = map[string]string{}
		}
		s.Refs[kind][workKey(id)] = workKey(id) + " | " + name
	}
	for _, kind := range []string{"shops", "blocks", "positions", "units", "employees", "materials"} {
		s.Refs[kind] = map[string]string{}
	}
	for _, r := range shops {
		add("shops", r.ID, r.Name)
	}
	for _, r := range blocks {
		add("blocks", r.ID, r.Name+" / "+s.Refs["shops"][workKey(r.WorkshopID)])
	}
	positionBlocks := map[uint]uint{}
	for _, r := range positions {
		add("positions", r.ID, "№ "+r.Number+" "+r.Name+" / "+s.Refs["blocks"][workKey(r.BlockID)])
		positionBlocks[r.ID] = r.BlockID
	}
	for _, r := range units {
		add("units", r.ID, r.Name)
	}
	for _, r := range employees {
		name := r.Name
		if r.Surname != nil {
			name = *r.Surname + " " + name
		}
		if r.MiddleName != nil {
			name += " " + *r.MiddleName
		}
		add("employees", r.ID, name)
	}
	for _, r := range s.Stock {
		add("materials", r.ID, r.Name+" / "+s.Refs["shops"][workKey(r.WorkshopID)]+" / "+s.Refs["units"][workKey(r.UnitID)])
	}
	dates := map[uint]string{}
	for _, d := range s.Book.Days {
		dates[d.ID] = d.Date
	}
	for _, shift := range s.Book.Shifts {
		s.Tables[0] = append(s.Tables[0], []string{workKey(shift.ID), dates[shift.DayID], workKey(shift.WorkshopID), strconv.Itoa(shift.Number)})
		for _, e := range shift.Employees {
			if _, ok := s.Refs["employees"][workKey(e.EmployeeCardID)]; !ok {
				add("employees", e.EmployeeCardID, e.EmployeeName+" (переведён)")
			}
			s.Tables[1] = append(s.Tables[1], []string{workKey(e.ID), workKey(shift.ID), workKey(e.EmployeeCardID), workDecimal(e.HoursHundredths, 2)})
		}
		for _, t := range shift.Tasks {
			// новый код chat: отсутствующие объём и единица остаются пустыми и в Excel.
			unit, quantity := "", ""
			if t.UnitID != 0 {
				unit = workKey(t.UnitID)
			}
			if t.QuantityHundredths != 0 {
				quantity = workDecimal(t.QuantityHundredths, 2)
			}
			s.Tables[2] = append(s.Tables[2], []string{workKey(t.ID), workKey(shift.ID), t.Name, unit, quantity, workDecimal(t.HoursHundredths, 2), workKey(positionBlocks[t.PositionID]), workKey(t.PositionID)})
			for _, m := range t.Materials {
				s.Tables[3] = append(s.Tables[3], []string{workKey(m.ID), workKey(t.ID), workKey(m.MaterialID), workDecimal(m.QuantityThousandths, 3)})
			}
		}
	}
	for _, r := range s.Receipts {
		if strings.HasPrefix(r.Date, fmt.Sprintf("%04d-%02d-", s.Book.Year, s.Book.Month)) {
			s.Tables[4] = append(s.Tables[4], []string{workKey(r.ID), r.Date, workKey(r.MaterialID), workDecimal(r.QuantityThousandths, 3)})
		}
	}
	return s, nil
}
func workExcelFingerprint(s workExcelState) string {
	data, _ := json.Marshal(s)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func workExcelDisplay(s workExcelState, table int, row []string) []string {
	result := append([]string{}, row...)
	for i, c := range workExcelSheets[table].Columns {
		if refs, ok := s.Refs[c.Kind]; ok {
			result[i] = refs[row[i]]
		}
	}
	return result
}

// Шаблон формируется сервером для конкретной книги и содержит актуальные справочники.
func BuildWorkOrderExcel(db *gorm.DB, agreementID, bookID uint, template bool) (*excelize.File, error) {
	var s workExcelState
	err := db.Transaction(func(tx *gorm.DB) error { var e error; s, e = loadWorkExcelState(tx, agreementID, bookID); return e })
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "Инструкция")
	header, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11}, Fill: excelize.Fill{Type: "pattern", Color: []string{"24476B"}, Pattern: 1}, Alignment: &excelize.Alignment{WrapText: true, Vertical: "center"}})
	if err != nil {
		f.Close()
		return nil, err
	}
	input, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "1556A0", Size: 11}, Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}, NumFmt: 49})
	instructions := []string{
		fmt.Sprintf("Книга наряд-заданий · %02d.%d · договор № %d", s.Book.Month, s.Book.Year, agreementID),
		"Заполняйте строки со второй строки. Названия листов, заголовки и лист _Служебное не изменять.",
		"Новые строки: задайте уникальный ключ new:1, new:2 и т. д. в пределах листа. Существующие ключи не менять.",
		"Ключ смены связывает сотрудников и работы со строкой листа Смены. Ключ работы связывает материалы с работой.",
		"Справочники выбирайте из выпадающего списка либо копируйте целиком с листа Справочники. Создание справочников через Excel запрещено.",
		"Даты: ГГГГ-ММ-ДД, только выбранный месяц. Числа без формул; часы и работы до 2 знаков, материалы до 3. На листе Работы единицу и количество можно оставить пустыми.", // новый код chat
		"Для поступлений используйте лист Поступления материалов. Карточки материалов предварительно создаются на сайте.",
		"Добавить и обновить: отсутствующие строки сохраняются. Чтобы изменить строку, сохраните её ключ из выгрузки.",
		"Заменить: выберите на сайте смены. Отсутствующие строки этих смен будут удалены, отсутствующая смена удаляется целиком.",
		"Замена поступлений включается отдельно и касается ВСЕХ поступлений выбранного месяца этого договора, отсутствующих в файле.",
		"Перенос существующей строки в другую смену/работу и смена материала поступления запрещены. Добавьте новую строку и удалите старую в режиме замены.",
		"Часы работ выше или ниже часов сотрудников: предупреждение. Больше 24 часов сотруднику в сутки или отрицательный склад: ошибка.",
		"Сначала проверьте файл на сайте, затем подтвердите изменения. Ошибка отменяет весь импорт. Если данные изменились после проверки, повторите проверку; если изменились ключи строк — скачайте свежую выгрузку.",
		"Пример связей (не данные): Смены — new:1; Сотрудники — new:1, new:1; Работы — new:1, new:1; Материалы работ — new:1, new:1.",
	}
	for i, line := range instructions {
		if err = f.SetCellStr("Инструкция", fmt.Sprintf("A%d", i+1), line); err != nil {
			f.Close()
			return nil, err
		}
		f.SetRowHeight("Инструкция", i+1, 42)
	}
	f.SetColWidth("Инструкция", "A", "A", 115)
	f.SetCellStyle("Инструкция", "A1", "A1", header)
	f.SetCellStyle("Инструкция", "A2", "A14", input)
	f.NewSheet("Справочники")
	kinds := []string{"shops", "employees", "units", "blocks", "positions", "materials"}
	labels := []string{"Цеха", "Сотрудники", "Единицы", "Блоки", "Позиции", "Материалы"}
	for col, kind := range kinds {
		letter, _ := excelize.ColumnNumberToName(col + 1)
		f.SetCellStr("Справочники", letter+"1", labels[col])
		f.SetColWidth("Справочники", letter, letter, 48)
		keys := []string{}
		for k := range s.Refs[kind] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			f.SetCellStr("Справочники", fmt.Sprintf("%s%d", letter, i+2), s.Refs[kind][k])
		}
		end := len(keys) + 1
		if end < 2 {
			end = 2
		}
		f.SetDefinedName(&excelize.DefinedName{Name: "ref_" + kind, RefersTo: fmt.Sprintf("'Справочники'!$%s$2:$%s$%d", letter, letter, end)})
	}
	f.SetCellStyle("Справочники", "A1", "F1", header)
	f.SetRowHeight("Справочники", 1, 30)
	for ti, sheet := range workExcelSheets {
		f.NewSheet(sheet.Name)
		f.SetPanes(sheet.Name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
		end, _ := excelize.ColumnNumberToName(len(sheet.Columns))
		f.SetCellStyle(sheet.Name, "A2", end+"1001", input)
		for i, c := range sheet.Columns {
			letter, _ := excelize.ColumnNumberToName(i + 1)
			f.SetCellStr(sheet.Name, letter+"1", c.Name)
			width := 22.0
			if _, ok := s.Refs[c.Kind]; ok || c.Kind == "text" {
				width = 48
			}
			f.SetColWidth(sheet.Name, letter, letter, width)
			if _, ok := s.Refs[c.Kind]; ok {
				dv := excelize.NewDataValidation(true)
				dv.SetSqref(letter + "2:" + letter + "10001")
				dv.SetSqrefDropList("ref_" + c.Kind)
				if err = f.AddDataValidation(sheet.Name, dv); err != nil {
					f.Close()
					return nil, err
				}
			}
		}
		f.SetCellStyle(sheet.Name, "A1", end+"1", header)
		f.SetRowHeight(sheet.Name, 1, 32)
		if !template {
			for i, row := range s.Tables[ti] {
				for j, value := range workExcelDisplay(s, ti, row) {
					cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
					if err = f.SetCellStr(sheet.Name, cell, value); err != nil {
						f.Close()
						return nil, err
					}
				}
			}
		}
		last := len(s.Tables[ti]) + 1
		if template || last < 2 {
			last = 2
		}
		f.AutoFilter(sheet.Name, fmt.Sprintf("A1:%s%d", end, last), nil)
	}
	f.NewSheet("_Служебное")
	for i, v := range []string{"BRM-WORK-1", workKey(agreementID), workKey(bookID), workExcelFingerprint(s)} {
		f.SetCellStr("_Служебное", fmt.Sprintf("A%d", i+1), v)
	}
	f.SetSheetVisible("_Служебное", false)
	f.SetActiveSheet(0)
	return f, nil
}

func parseWorkOrderExcel(data []byte, s workExcelState) (workExcelTables, []string) {
	var tables workExcelTables
	problems := []string{}
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 8 << 20})
	if err != nil {
		return tables, []string{"Не удалось прочитать XLSX: " + err.Error()}
	}
	defer f.Close()
	for i, want := range []string{"BRM-WORK-1", workKey(s.Book.AgreementID), workKey(s.Book.ID)} {
		got, e := f.GetCellValue("_Служебное", fmt.Sprintf("A%d", i+1))
		if e != nil || got != want {
			return tables, []string{"Файл другого договора/месяца или повреждено описание формата. Скачайте шаблон для выбранной книги."}
		}
	}
	total := 0
	for ti, sheet := range workExcelSheets {
		rows, e := f.Rows(sheet.Name)
		if e != nil {
			problems = append(problems, "Нет листа «"+sheet.Name+"»")
			continue
		}
		keys := map[string]bool{}
		line := 0
		for rows.Next() {
			line++
			total++
			if total > 10000 {
				rows.Close()
				return tables, []string{"Максимум 10 000 строк на файл"}
			}
			// Читаем исходное число, а не округлённое отображение формата Excel.
			values, e := rows.Columns(excelize.Options{RawCellValue: true})
			if e != nil {
				problems = append(problems, e.Error())
				break
			}
			if line == 1 {
				for i, c := range sheet.Columns {
					if i >= len(values) || values[i] != c.Name {
						problems = append(problems, sheet.Name+": изменены заголовки")
						break
					}
				}
				continue
			}
			if strings.TrimSpace(strings.Join(values, "")) == "" {
				continue
			}
			row := make([]string, len(sheet.Columns))
			copy(row, values)
			if len(values) > len(row) {
				problems = append(problems, fmt.Sprintf("%s, строка %d: лишние столбцы", sheet.Name, line))
			}
			for i, c := range sheet.Columns {
				v := strings.TrimSpace(row[i])
				cell, _ := excelize.CoordinatesToCellName(i+1, line)
				fail := func(message string) {
					problems = append(problems, fmt.Sprintf("%s!%s (%s): %s", sheet.Name, cell, c.Name, message))
				}
				formula, fe := f.GetCellFormula(sheet.Name, cell)
				if fe != nil || formula != "" {
					fail("формулы не поддерживаются, вставьте значение")
				}
				// новый код chat: только единица и объём работы допускают пустое значение.
				if ti == 2 && (i == 3 || i == 4) && v == "" {
					row[i] = ""
					continue
				}
				if v == "" || len(v) > 2000 {
					fail("заполните значение (до 2000 байт)")
				}
				if refs, ok := s.Refs[c.Kind]; ok {
					id := strings.TrimSpace(strings.SplitN(v, "|", 2)[0])
					label, exists := refs[id]
					if !exists || v != label {
						fail("выберите существующее значение из справочника целиком")
					}
					v = id
				} else {
					switch c.Kind {
					case "date":
						d, e := time.Parse("2006-01-02", v)
						if e != nil || d.Year() != s.Book.Year || int(d.Month()) != s.Book.Month {
							fail("дата должна быть в месяце книги, формат ГГГГ-ММ-ДД")
						}
					case "q2", "q3", "hours":
						digits := 2
						if c.Kind == "q3" {
							digits = 3
						}
						n, e := ParseWorkQuantity(v, digits, c.Kind == "hours")
						if e != nil {
							fail(e.Error())
						} else {
							v = workDecimal(n, digits)
						}
					case "shift":
						if v != "1" && v != "2" {
							fail("укажите 1 или 2")
						}
					case "key", "parent":
						if !validWorkExcelKey(v) {
							fail("ключ: положительный ID из выгрузки или new:уникальное-имя")
						}
					}
				}
				row[i] = v
			}
			if keys[row[0]] {
				problems = append(problems, fmt.Sprintf("%s, строка %d: повтор ключа %s", sheet.Name, line, row[0]))
			}
			keys[row[0]] = true
			tables[ti] = append(tables[ti], row)
			if len(problems) >= 100 {
				rows.Close()
				return tables, problems
			}
		}
		if e := rows.Error(); e != nil {
			problems = append(problems, e.Error())
		}
		rows.Close()
		if line == 0 {
			problems = append(problems, sheet.Name+": пустой лист без заголовков")
		}
	}
	return tables, problems
}
func validWorkExcelKey(v string) bool {
	if strings.HasPrefix(v, "new:") {
		return len(v) > 4 && len(v) <= 80
	}
	n, e := strconv.ParseUint(v, 10, 32)
	return e == nil && n > 0 && strconv.FormatUint(n, 10) == v
}

// --------- конец нового кода chat
