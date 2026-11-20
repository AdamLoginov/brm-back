// --------- новый код chat: предварительный расчёт изменений и атомарное подтверждение Excel
package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"project/internal/models"
)

type WorkImportOptions struct {
	Mode            string `json:"mode"`
	ReplaceShifts   []uint `json:"replace_shifts"`
	ReplaceReceipts bool   `json:"replace_receipts"`
}
type WorkImportChange struct {
	Sheet   string   `json:"sheet"`
	Context string   `json:"context"`
	Key     string   `json:"key"`
	Action  string   `json:"action"`
	Columns []string `json:"columns"`
	Before  []string `json:"before"`
	After   []string `json:"after"`
}
type WorkImportStock struct {
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type WorkImportReport struct {
	Changes  []WorkImportChange `json:"changes"`
	Warnings []string           `json:"warnings"`
	Errors   []string           `json:"errors"`
	Stock    []WorkImportStock  `json:"stock"`
	Token    string             `json:"token"`
	Applied  bool               `json:"applied"`
}

var workImportSigningKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}()
var workImportPreviewRollback = errors.New("work import preview rollback")

func workImportToken(s workExcelState, data []byte, options WorkImportOptions) string {
	h := hmac.New(sha256.New, workImportSigningKey)
	h.Write([]byte(workExcelFingerprint(s)))
	h.Write(data)
	raw, _ := json.Marshal(options)
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}
func workTableMap(rows [][]string) map[string][]string {
	result := map[string][]string{}
	for _, r := range rows {
		result[r[0]] = r
	}
	return result
}
func workMapRows(rows map[string][]string) [][]string {
	keys := []string{}
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := [][]string{}
	for _, k := range keys {
		result = append(result, rows[k])
	}
	return result
}
func workRowEqual(a, b []string) bool { return strings.Join(a, "\x00") == strings.Join(b, "\x00") }
func workUint(s string) uint          { n, _ := strconv.ParseUint(s, 10, 32); return uint(n) }

func mergeWorkExcel(s workExcelState, incoming workExcelTables, options WorkImportOptions) (workExcelTables, []string) {
	var final workExcelTables
	problems := []string{}
	if options.Mode != "merge" && options.Mode != "replace" {
		return final, []string{"Неизвестный режим импорта"}
	}
	if options.Mode == "merge" && (len(options.ReplaceShifts) > 0 || options.ReplaceReceipts) {
		return final, []string{"Удаление разрешено только в режиме замены"}
	}
	oldShifts := workTableMap(s.Tables[0])
	oldTasks := workTableMap(s.Tables[2])
	selected := map[string]bool{}
	for _, id := range options.ReplaceShifts {
		k := workKey(id)
		if oldShifts[k] == nil {
			problems = append(problems, "Выбрана смена другого месяца или договора")
		}
		selected[k] = true
	}
	for ti := range workExcelSheets {
		old := workTableMap(s.Tables[ti])
		next := workTableMap(s.Tables[ti])
		file := workTableMap(incoming[ti])
		for _, r := range incoming[ti] {
			previous, exists := old[r[0]]
			if !exists && !strings.HasPrefix(r[0], "new:") {
				problems = append(problems, workExcelSheets[ti].Name+": неизвестный ID "+r[0]+"; для новой строки используйте new:...")
			}
			if exists {
				immutable := []int{}
				switch ti {
				case 0:
					immutable = []int{1, 2, 3}
				case 1, 2, 3:
					immutable = []int{1}
				case 4:
					immutable = []int{2}
				}
				for _, c := range immutable {
					if r[c] != previous[c] {
						problems = append(problems, fmt.Sprintf("%s, ключ %s: нельзя менять «%s» у существующей строки", workExcelSheets[ti].Name, r[0], workExcelSheets[ti].Columns[c].Name))
					}
				}
			}
			next[r[0]] = r
		}
		if options.Mode == "replace" {
			for k, r := range old {
				remove := false
				switch ti {
				case 0:
					remove = selected[k]
				case 1, 2:
					remove = selected[r[1]]
				case 3:
					if task := oldTasks[r[1]]; task != nil {
						remove = selected[task[1]]
					}
				case 4:
					remove = options.ReplaceReceipts
				}
				if remove && file[k] == nil {
					delete(next, k)
				}
			}
		}
		final[ti] = workMapRows(next)
	}
	shifts := workTableMap(final[0])
	tasks := workTableMap(final[2])
	for ti := 1; ti <= 3; ti++ {
		for _, r := range final[ti] {
			parents := shifts
			if ti == 3 {
				parents = tasks
			}
			if parents[r[1]] == nil {
				problems = append(problems, fmt.Sprintf("%s, ключ %s: отсутствует связанная строка %s", workExcelSheets[ti].Name, r[0], r[1]))
			}
		}
	}
	return final, problems
}
func workExcelChanges(s workExcelState, final workExcelTables) []WorkImportChange {
	changes := []WorkImportChange{}
	for ti, sheet := range workExcelSheets {
		before, after := workTableMap(s.Tables[ti]), workTableMap(final[ti])
		all := map[string][]string{}
		for k, r := range before {
			all[k] = r
		}
		for k, r := range after {
			all[k] = r
		}
		for _, r := range workMapRows(all) {
			k := r[0]
			a, b := before[k], after[k]
			if workRowEqual(a, b) {
				continue
			}
			change := WorkImportChange{Sheet: sheet.Name, Key: k, Action: "update", Before: []string{}, After: []string{}, Columns: []string{}}
			contextTables, contextRow := final, b
			if b == nil {
				contextTables, contextRow = s.Tables, a
			}
			change.Context = workExcelRowContext(s, contextTables, ti, contextRow)
			for _, c := range sheet.Columns {
				change.Columns = append(change.Columns, c.Name)
			}
			if a == nil {
				change.Action = "create"
			} else {
				change.Before = workExcelDisplay(s, ti, a)
			}
			if b == nil {
				change.Action = "delete"
			} else {
				change.After = workExcelDisplay(s, ti, b)
			}
			changes = append(changes, change)
		}
	}
	return changes
}

func workExcelRowContext(s workExcelState, tables workExcelTables, ti int, row []string) string {
	if ti == 4 {
		return row[1]
	}
	shiftKey := row[0]
	workName := ""
	if ti == 1 || ti == 2 {
		shiftKey = row[1]
	}
	if ti == 3 {
		for _, task := range tables[2] {
			if task[0] == row[1] {
				shiftKey = task[1]
				workName = " · " + task[2]
				break
			}
		}
	}
	for _, shift := range tables[0] {
		if shift[0] == shiftKey {
			return shift[1] + " · " + s.Refs["shops"][shift[2]] + " · смена " + shift[3] + workName
		}
	}
	return ""
}
func workShiftRows(t workExcelTables, key string) workExcelTables {
	var result workExcelTables
	tasks := map[string]bool{}
	for _, r := range t[0] {
		if r[0] == key {
			result[0] = append(result[0], r)
		}
	}
	for ti := 1; ti <= 2; ti++ {
		for _, r := range t[ti] {
			if r[1] == key {
				result[ti] = append(result[ti], r)
				if ti == 2 {
					tasks[r[0]] = true
				}
			}
		}
	}
	for _, r := range t[3] {
		if tasks[r[1]] {
			result[3] = append(result[3], r)
		}
	}
	for i := range result {
		result[i] = workMapRows(workTableMap(result[i]))
	}
	return result
}
func applyWorkExcel(tx *gorm.DB, s workExcelState, final workExcelTables, report *WorkImportReport) error {
	beforeShifts, afterShifts := workTableMap(s.Tables[0]), workTableMap(final[0])
	affected := map[string]bool{}
	for _, rows := range []map[string][]string{beforeShifts, afterShifts} {
		for key := range rows {
			a, _ := json.Marshal(workShiftRows(s.Tables, key))
			b, _ := json.Marshal(workShiftRows(final, key))
			if string(a) != string(b) {
				affected[key] = true
			}
		}
	}
	// Освобождаем списания и часы всех меняющихся смен до проверок новых данных.
	// Старые строки сотрудников с нулевыми часами сохраняют право исторического участия.
	for key := range affected {
		if beforeShifts[key] != nil {
			id := workUint(key)
			if err := tx.Where("task_id IN (?)", tx.Model(&models.WorkOrderTask{}).Select("id").Where("shift_id = ?", id)).Delete(&models.WorkOrderMaterial{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.WorkOrderEmployee{}).Where("shift_id = ?", id).UpdateColumn("hours_hundredths", 0).Error; err != nil {
				return err
			}
		}
	}
	// Поступления меняются вместе со списаниями; итоговый остаток проверяется после всего пакета.
	oldReceipts, newReceipts := workTableMap(s.Tables[4]), workTableMap(final[4])
	for k := range oldReceipts {
		if newReceipts[k] == nil {
			if err := tx.Delete(&models.WarehouseReceipt{}, workUint(k)).Error; err != nil {
				return err
			}
		}
	}
	for _, r := range final[4] {
		if workRowEqual(oldReceipts[r[0]], r) {
			continue
		}
		quantity, err := ParseWorkQuantity(r[3], 3, false)
		if err != nil {
			return err
		}
		row := models.WarehouseReceipt{MaterialID: workUint(r[2]), Date: r[1], QuantityThousandths: quantity}
		if oldReceipts[r[0]] != nil {
			row.ID = workUint(r[0])
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
	}
	shiftDetails := map[string]WorkShiftDetail{}
	for _, r := range s.Book.Shifts {
		shiftDetails[workKey(r.ID)] = r
	}
	days := map[string]uint{}
	for _, d := range s.Book.Days {
		days[d.Date] = d.ID
	}
	// Удаления выполняются раньше создания, чтобы освобождались уникальные дата/цех/смена.
	for key := range affected {
		if beforeShifts[key] != nil && afterShifts[key] == nil {
			if err := DeleteWorkShift(tx, s.Book.AgreementID, workUint(key), shiftDetails[key].Version); err != nil {
				return err
			}
		}
	}
	for _, r := range final[0] {
		key := r[0]
		if !affected[key] {
			continue
		}
		rows := workShiftRows(final, key)
		input := WorkShiftInput{DayID: days[r[1]], WorkshopID: workUint(r[2]), Number: int(workUint(r[3])), Version: shiftDetails[key].Version}
		for _, e := range rows[1] {
			input.Employees = append(input.Employees, WorkEmployeeInput{EmployeeCardID: workUint(e[2]), Hours: e[3]})
		}
		for _, t := range rows[2] {
			var position models.AgreementWorkPosition
			if err := tx.Where("id = ? AND block_id = ? AND agreement_id = ?", workUint(t[7]), workUint(t[6]), s.Book.AgreementID).First(&position).Error; err != nil {
				return fmt.Errorf("Работы, ключ %s: позиция не принадлежит выбранному блоку", t[0])
			}
			task := WorkTaskInput{Name: t[2], UnitID: workUint(t[3]), Quantity: t[4], Hours: t[5], PositionID: workUint(t[7])}
			for _, m := range rows[3] {
				if m[1] == t[0] {
					task.Materials = append(task.Materials, WorkMaterialInput{MaterialID: workUint(m[2]), Quantity: m[3]})
				}
			}
			input.Tasks = append(input.Tasks, task)
		}
		id := uint(0)
		if beforeShifts[key] != nil {
			id = workUint(key)
		}
		_, warning, err := SaveWorkShift(tx, s.Book.AgreementID, id, input)
		if err != nil {
			return fmt.Errorf("Смена %s (%s): %w", key, r[1], err)
		}
		if warning != "" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s · смена %s: %s", r[1], key, warning))
		}
	}
	for _, m := range s.Stock {
		balance, err := WarehouseBalance(tx, m.ID)
		if err != nil {
			return err
		}
		if balance < 0 {
			return fmt.Errorf("Недостаточно материала «%s»: итоговый остаток отрицательный", m.Name)
		}
		if balance != m.Balance {
			report.Stock = append(report.Stock, WorkImportStock{Name: s.Refs["materials"][workKey(m.ID)], Before: workDecimal(m.Balance, 3), After: workDecimal(balance, 3)})
		}
	}
	return nil
}

// preview и commit используют одни и те же проверки. Preview всегда откатывает транзакцию.
// Подпись связывает подтверждение с файлом, настройками и состоянием базы; подменить план нельзя.
func ImportWorkOrderExcel(db *gorm.DB, agreementID, bookID uint, data []byte, options WorkImportOptions, token string, commit bool) (WorkImportReport, error) {
	report := WorkImportReport{Changes: []WorkImportChange{}, Warnings: []string{}, Errors: []string{}, Stock: []WorkImportStock{}}
	err := WorkTransaction(db, agreementID, func(tx *gorm.DB) error {
		s, err := loadWorkExcelState(tx, agreementID, bookID)
		if err != nil {
			return err
		}
		incoming, problems := parseWorkOrderExcel(data, s)
		if len(problems) > 0 {
			report.Errors = problems
			return workImportPreviewRollback
		}
		final, problems := mergeWorkExcel(s, incoming, options)
		if len(problems) > 0 {
			report.Errors = problems
			return workImportPreviewRollback
		}
		report.Changes = workExcelChanges(s, final)
		expected := workImportToken(s, data, options)
		if commit && !hmac.Equal([]byte(token), []byte(expected)) {
			return errors.New("Запись изменена или проверка не выполнена. Повторите предварительную проверку файла")
		}
		if err := applyWorkExcel(tx, s, final, &report); err != nil {
			report.Errors = append(report.Errors, err.Error())
			report.Stock = []WorkImportStock{}
			return workImportPreviewRollback
		}
		if !commit {
			report.Token = expected
			return workImportPreviewRollback
		}
		report.Applied = true
		return nil
	})
	if errors.Is(err, workImportPreviewRollback) {
		err = nil
	}
	if err != nil {
		report.Token = ""
		report.Applied = false
	}
	return report, err
}

// --------- конец нового кода chat
