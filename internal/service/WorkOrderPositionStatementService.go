// --------- новый код chat: ведомость выбранной позиции в исходном шаблоне
package service

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"project/internal/models"
)

//go:embed templates/work_order_position_statement.xlsx
var workPositionStatementTemplate []byte

func BuildWorkOrderPositionStatement(db *gorm.DB, agreementID, bookID, positionID uint) (*excelize.File, error) {
	return buildWorkOrderStatement(db, agreementID, bookID, positionID, 0)
}

// новый код chat: ведомость работ блока без позиции.
func BuildWorkOrderBlockStatement(db *gorm.DB, agreementID, bookID, blockID uint) (*excelize.File, error) {
	return buildWorkOrderStatement(db, agreementID, bookID, 0, blockID)
}

func buildWorkOrderStatement(db *gorm.DB, agreementID, bookID, positionID, blockID uint) (*excelize.File, error) {
	var book WorkBookDetail
	var position models.AgreementWorkPosition
	var block models.AgreementWorkBlock
	var shop models.AgreementWorkshop
	var units []models.MeasurementUnit
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		book, err = LoadWorkBook(tx, agreementID, bookID)
		if err != nil {
			return err
		}
		if positionID == 0 && blockID == 0 {
			return gorm.ErrRecordNotFound
		}
		if positionID != 0 {
			if err = tx.Where("id = ? AND agreement_id = ?", positionID, agreementID).First(&position).Error; err != nil {
				return err
			}
		} else {
			position.BlockID = blockID
		} // новый код chat
		if err = tx.Where("id = ? AND agreement_id = ?", position.BlockID, agreementID).First(&block).Error; err != nil {
			return err
		}
		if err = tx.Where("id = ? AND agreement_id = ?", block.WorkshopID, agreementID).First(&shop).Error; err != nil {
			return err
		}
		return tx.Find(&units).Error
	})
	if err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(workPositionStatementTemplate))
	if err != nil {
		return nil, err
	}
	if err = fillWorkPositionStatement(f, f.GetSheetName(0), book, position, block, shop, units); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// новый код chat: единое заполнение листа для отдельной и общей выгрузки.
func fillWorkPositionStatement(f *excelize.File, sheet string, book WorkBookDetail, position models.AgreementWorkPosition, block models.AgreementWorkBlock, shop models.AgreementWorkshop, units []models.MeasurementUnit) error {
	unitNames := map[uint]string{}
	for _, u := range units {
		unitNames[u.ID] = u.Name
	}
	type work struct {
		name            string
		unit            uint
		quantity, hours int64
	}
	groups := map[string]*work{}
	for _, shift := range book.Shifts {
		for _, task := range shift.Tasks {
			if task.PositionID != position.ID || task.Accounted {
				continue
			}
			if position.ID == 0 && task.BlockID != block.ID {
				continue
			} // новый код chat: не смешиваем работы разных блоков
			key := fmt.Sprintf("%s\x00%d", strings.ToLower(strings.Join(strings.Fields(task.Name), " ")), task.UnitID)
			if groups[key] == nil {
				groups[key] = &work{name: task.Name, unit: task.UnitID}
			}
			groups[key].quantity += workQuantityThousandths(task.WorkOrderTask) // новый код chat
			if !task.HoursNotSpecified {
				groups[key].hours += task.HoursHundredths
			}
		}
	}
	keys := []string{}
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var writeErr error
	check := func(err error) {
		if writeErr == nil {
			writeErr = err
		}
	}
	// Шаблон содержит девять строк-примеров. Дополнительные строки сдвигают подписи вниз.
	extra := len(keys) - 9
	if extra < 0 {
		extra = 0
	}
	for i := 0; i < extra; i++ {
		check(f.DuplicateRowTo(sheet, 22, 23))
	}
	for row := 14; row <= 22+extra; row++ {
		for col := 1; col <= 6; col++ {
			cell, _ := excelize.CoordinatesToCellName(col, row)
			check(f.SetCellStr(sheet, cell, ""))
		}
	}
	lastDay := time.Date(book.Year, time.Month(book.Month)+1, 0, 0, 0, 0, 0, time.UTC)
	check(f.SetCellStr(sheet, "A9", "ВЕДОМОСТЬ ОБЪЕМОВ РАБОТ от "+lastDay.Format("02.01.2006")+"г."))
	// Убираем повторяющиеся префиксы из названий справочников: «Цех 1» -> «Цех № 1».
	label := func(name, prefix string) string {
		pattern := regexp.MustCompile(`(?i)^` + prefix + `\s*(?:№\s*)?`)
		value := strings.TrimSpace(pattern.ReplaceAllString(strings.TrimSpace(name), ""))
		if value == "" {
			return strings.TrimSpace(name)
		} // новый код chat: название без номера тоже допустимо
		return prefix + " № " + value
	}
	heading := fmt.Sprintf("ГМЗ ПАО \"ППГХО\". Зд. 622. %s. %s. %s. Поз. № %s.", label(shop.Name, "Цех"), label(block.Name, "Блок"), position.Name, position.Number)
	if position.ID == 0 {
		heading = fmt.Sprintf("ГМЗ ПАО \"ППГХО\". Зд. 622. %s. %s.", label(shop.Name, "Цех"), label(block.Name, "Блок"))
	} // новый код chat
	check(f.SetCellStr(sheet, "A10", heading))
	height := float64((len([]rune(heading))/85 + 1) * 18)
	if height < 45.6 {
		height = 45.6
	}
	if height > 409 {
		height = 409
	}
	check(f.SetRowHeight(sheet, 10, height))
	for index, key := range keys {
		row := 14 + index
		item := groups[key]
		unit := unitNames[item.unit]
		if unit == "" {
			unit = "—"
		}
		// новый код chat: целые часы без дробной части, дробные — с запятой без лишних нулей.
		hoursText := strings.TrimRight(strings.TrimRight(workDecimal(item.hours, 2), "0"), ".")
		values := []interface{}{index + 1, item.name, unit, float64(item.quantity) / 1000, strings.ReplaceAll(hoursText, ".", ",") + " чел. час", ""}
		check(f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values))
		// Сохраняем стили исходного шаблона, увеличивая высоту для длинных названий.
		lines := 0
		for _, line := range strings.Split(item.name, "\n") {
			lines += len([]rune(line))/28 + 1
		}
		h := float64(lines*16 + 8)
		if h < 24 {
			h = 24
		}
		if h > 409 {
			h = 409
		}
		check(f.SetRowHeight(sheet, row, h))
	}
	// Область печати включает все работы и неизменённый блок подписей.
	for _, name := range f.GetDefinedName() {
		if name.Name == "_xlnm.Print_Area" && name.Scope == sheet {
			check(f.DeleteDefinedName(&name))
		}
	}
	check(f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", Scope: sheet, RefersTo: fmt.Sprintf("'%s'!$A$1:$G$%d", strings.ReplaceAll(sheet, "'", "''"), 34+extra)}))
	return writeErr
}

// новый код chat: все ведомости месяца отдельными файлами в ZIP-архиве.
func BuildAllWorkOrderPositionStatements(db *gorm.DB, agreementID, bookID uint) ([]byte, error) {
	var book WorkBookDetail
	var positions []models.AgreementWorkPosition
	var blocks []models.AgreementWorkBlock
	var shops []models.AgreementWorkshop
	var units []models.MeasurementUnit
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		book, err = LoadWorkBook(tx, agreementID, bookID)
		if err != nil {
			return err
		}
		for _, q := range []*gorm.DB{tx.Where("agreement_id = ?", agreementID).Order("block_id, number, id").Find(&positions), tx.Where("agreement_id = ?", agreementID).Find(&blocks), tx.Where("agreement_id = ?", agreementID).Find(&shops), tx.Find(&units)} {
			if q.Error != nil {
				return q.Error
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	used := map[uint]bool{}
	for _, shift := range book.Shifts {
		for _, task := range shift.Tasks {
			if !task.Accounted && task.PositionID != 0 {
				used[task.PositionID] = true
			}
		}
	}
	selected := []models.AgreementWorkPosition{}
	for _, position := range positions {
		if used[position.ID] {
			selected = append(selected, position)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("В книге нет работ по позициям без отметки «Учтён»")
	}
	blockMap := map[uint]models.AgreementWorkBlock{}
	for _, block := range blocks {
		blockMap[block.ID] = block
	}
	shopMap := map[uint]models.AgreementWorkshop{}
	for _, shop := range shops {
		shopMap[shop.ID] = shop
	}
	// новый код chat: каждый файл создаётся из чистого шаблона и помещается в ZIP.
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	seen := map[string]bool{}
	for _, position := range selected {
		block, ok := blockMap[position.BlockID]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		shop, ok := shopMap[block.WorkshopID]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		file, err := excelize.OpenReader(bytes.NewReader(workPositionStatementTemplate))
		if err != nil {
			return nil, err
		}
		err = fillWorkPositionStatement(file, file.GetSheetName(0), book, position, block, shop, units)
		if err != nil {
			file.Close()
			return nil, err
		}
		buffer, err := file.WriteToBuffer()
		file.Close()
		if err != nil {
			return nil, err
		}
		base := workStatementFilename(block.Name, position.Number, position.Name)
		name := base
		for suffix := 2; seen[strings.ToLower(name)]; suffix++ {
			name = fmt.Sprintf("%s (%d).xlsx", strings.TrimSuffix(base, ".xlsx"), suffix)
		}
		seen[strings.ToLower(name)] = true
		entry, err := archive.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = entry.Write(buffer.Bytes()); err != nil {
			return nil, err
		}
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// новый код chat: читаемые безопасные имена файлов, без путей и недопустимых символов.
func workStatementFilename(block, number, name string) string {
	block = regexp.MustCompile(`(?i)^блок\s*(?:№\s*)?`).ReplaceAllString(strings.TrimSpace(block), "")
	value := fmt.Sprintf("Ведомость объемов работ Б%s поз.%s %s", block, number, name)
	value = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`\\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, value)
	value = strings.TrimRight(strings.Join(strings.Fields(value), " "), ". ")
	runes := []rune(value)
	if len(runes) > 180 {
		value = string(runes[:180])
	}
	return value + ".xlsx"
}

// --------- конец нового кода chat
