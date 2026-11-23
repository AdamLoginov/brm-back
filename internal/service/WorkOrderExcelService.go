// --------- новый код chat: снимок данных для проверки импорта общего журнала
package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

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
	{"Работы", []workExcelColumn{{"Ключ работы", "key"}, {"Ключ смены", "parent"}, {"Наименование работы", "text"}, {"Единица", "units"}, {"Количество", "q3"}, {"Человеко-часы", "hours"}, {"Блок", "blocks"}, {"Позиция", "positions"}, {"Учтён", "bool"}}}, // новый код chat
	{"Материалы работ", []workExcelColumn{{"Ключ строки", "key"}, {"Ключ работы", "parent"}, {"Материал", "materials"}, {"Количество", "q3"}}},
}

type workExcelTables [4][][]string
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
		tx.Select("id", "name", "surname", "middle_name").Order("id").Find(&employees), // новый код chat: весь список работающих и уволенных, без удалённых карточек
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
			// новый код chat: историческая запись не возвращает удалённую карточку в список импорта
			s.Tables[1] = append(s.Tables[1], []string{workKey(e.ID), workKey(shift.ID), workKey(e.EmployeeCardID), workDecimal(e.HoursHundredths, 2)})
		}
		for _, t := range shift.Tasks {
			// новый код chat: отсутствующие объём и единица остаются пустыми и в Excel.
			unit, quantity := "", ""
			if t.UnitID != 0 {
				unit = workKey(t.UnitID)
			}
			if workQuantityThousandths(t.WorkOrderTask) != 0 {
				quantity = workDecimal(workQuantityThousandths(t.WorkOrderTask), 3)
			}
			// новый код chat: пустые часы, блок и позиция сохраняются пустыми в Excel.
			hours, block, position := "", "", ""
			if !t.HoursNotSpecified {
				hours = workDecimal(t.HoursHundredths, 2)
			}
			blockID := t.BlockID
			if blockID == 0 {
				blockID = positionBlocks[t.PositionID]
			}
			if blockID != 0 {
				block = workKey(blockID)
			}
			if t.PositionID != 0 {
				position = workKey(t.PositionID)
			}
			s.Tables[2] = append(s.Tables[2], []string{workKey(t.ID), workKey(shift.ID), t.Name, unit, quantity, hours, block, position, strconv.FormatBool(t.Accounted)}) // новый код chat
			for _, m := range t.Materials {
				s.Tables[3] = append(s.Tables[3], []string{workKey(m.ID), workKey(t.ID), workKey(m.MaterialID), workDecimal(m.QuantityThousandths, 3)})
			}
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
