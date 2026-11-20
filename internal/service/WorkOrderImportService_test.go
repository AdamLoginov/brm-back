// --------- новый код chat: preview/confirm, откаты, замена и защита от устаревшего подтверждения
package service

import (
	"github.com/xuri/excelize/v2"
	"project/internal/models"
	"strings"
	"testing"
)

func previewExcel(t *testing.T, f workFixture, data []byte, options WorkImportOptions) WorkImportReport {
	t.Helper()
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, options, "", false)
	if e != nil || len(r.Errors) > 0 || r.Token == "" {
		t.Fatal("preview", e, r.Errors)
	}
	return r
}
func confirmExcel(t *testing.T, f workFixture, data []byte, options WorkImportOptions, token string) WorkImportReport {
	t.Helper()
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, options, token, true)
	if e != nil || len(r.Errors) > 0 || !r.Applied {
		t.Fatal("confirm", e, r.Errors)
	}
	return r
}

func TestWorkExcelPreviewCommitAndStale(t *testing.T) {
	f := newWorkFixture(t)
	shift, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input())
	if err != nil {
		t.Fatal(err)
	}
	file := fixtureExcel(t, f, false)
	excelCell(t, file, "Работы", "F2", "15")
	data := excelBytes(t, file)
	options := WorkImportOptions{Mode: "merge"}
	before := workExcelFingerprint(fixtureState(t, f))
	report := previewExcel(t, f, data, options)
	if len(report.Changes) != 1 || len(report.Warnings) != 1 || report.Changes[0].Action != "update" {
		t.Fatal(report)
	}
	if before != workExcelFingerprint(fixtureState(t, f)) {
		t.Fatal("preview persisted writes")
	}
	if _, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, options, "invalid-token", true); err == nil {
		t.Fatal("unsigned confirmation")
	}
	excelCell(t, file, "Работы", "F2", "16")
	if _, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), options, report.Token, true); err == nil {
		t.Fatal("file replaced after preview")
	}
	confirmExcel(t, f, data, options, report.Token)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil || book.Shifts[0].Tasks[0].HoursHundredths != 1500 || book.Shifts[0].Version != shift.Version+1 {
		t.Fatal(book, err)
	}
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, options, report.Token, true)
	if e == nil && len(r.Errors) == 0 {
		t.Fatal("old workbook accepted twice")
	}
}

func TestWorkExcelAtomicReceiptAndUsageEdit(t *testing.T) {
	f := newWorkFixture(t)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input()); err != nil {
		t.Fatal(err)
	}
	file := fixtureExcel(t, f, false)
	excelCell(t, file, "Поступления материалов", "D2", "20.125")
	excelCell(t, file, "Материалы работ", "D2", "10.125")
	data := excelBytes(t, file)
	options := WorkImportOptions{Mode: "merge"}
	r := previewExcel(t, f, data, options)
	if len(r.Stock) != 1 || r.Stock[0].After != "10.000" {
		t.Fatal(r.Stock)
	}
	confirmExcel(t, f, data, options, r.Token)
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 10000 {
		t.Fatal(balance)
	}
	file = fixtureExcel(t, f, false)
	excelCell(t, file, "Поступления материалов", "D2", "1")
	before := workExcelFingerprint(fixtureState(t, f))
	r, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), options, "", false)
	if err != nil || len(r.Errors) == 0 || r.Token != "" || before != workExcelFingerprint(fixtureState(t, f)) {
		t.Fatal("overdraw must rollback", r, err)
	}
}

func TestWorkExcelReplacementScopeAndDeletes(t *testing.T) {
	f := newWorkFixture(t)
	first, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input())
	if err != nil {
		t.Fatal(err)
	}
	in := f.input()
	in.Number = 2
	in.Tasks = nil
	second, _, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, 0, WarehouseReceiptInput{Date: "2028-03-01", Quantity: "7"}, false); err != nil {
		t.Fatal(err)
	}
	file := fixtureExcel(t, f, true)
	data := excelBytes(t, file)
	r := previewExcel(t, f, data, WorkImportOptions{Mode: "merge"})
	if len(r.Changes) != 0 {
		t.Fatal("empty merge deletes")
	}
	options := WorkImportOptions{Mode: "replace", ReplaceShifts: []uint{first.ID}, ReplaceReceipts: true}
	r = previewExcel(t, f, data, options)
	if len(r.Changes) != 5 {
		t.Fatal("expected shift, employee, task, usage, receipt", r.Changes)
	}
	for _, c := range r.Changes {
		if c.Action != "delete" {
			t.Fatal(c)
		}
	}
	confirmExcel(t, f, data, options, r.Token)
	book, _ := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if len(book.Shifts) != 1 || book.Shifts[0].ID != second.ID {
		t.Fatal("unselected shift removed", book)
	}
	var receipts []models.WarehouseReceipt
	f.db.Find(&receipts)
	if len(receipts) != 1 || receipts[0].Date != "2028-03-01" {
		t.Fatal("other month affected", receipts)
	}
}

func TestWorkExcelCreateAllSheets(t *testing.T) {
	f := newWorkFixture(t)
	file := fixtureExcel(t, f, true)
	s := fixtureState(t, f)
	rows := workExcelTables{
		{{"new:s", "2028-02-02", workKey(f.shop.ID), "1"}},
		{{"new:e", "new:s", workKey(f.employee.ID), "11.00"}},
		{{"new:w", "new:s", "Монтаж", workKey(f.unit.ID), "2.50", "12.00", workKey(f.position.BlockID), workKey(f.position.ID)}},
		{{"new:m", "new:w", workKey(f.material.ID), "101.250"}},
		{{"new:r", "2028-02-02", workKey(f.material.ID), "10.125"}},
	}
	for ti, table := range rows {
		for ri, row := range table {
			for ci, value := range workExcelDisplay(s, ti, row) {
				cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
				excelCell(t, file, workExcelSheets[ti].Name, cell, value)
			}
		}
	}
	data := excelBytes(t, file)
	options := WorkImportOptions{Mode: "merge"}
	r := previewExcel(t, f, data, options)
	if len(r.Changes) != 5 || len(r.Warnings) != 1 {
		t.Fatal(r)
	}
	confirmExcel(t, f, data, options, r.Token)
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 9000 {
		t.Fatal(balance)
	}
	book, _ := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if len(book.Shifts) != 1 || len(book.Shifts[0].Tasks) != 1 || len(book.Shifts[0].Tasks[0].Materials) != 1 {
		t.Fatal(book)
	}
}

func TestWorkExcelHistoricalEmployeeAndDailyLimit(t *testing.T) {
	f := newWorkFixture(t)
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, f.input()); err != nil {
		t.Fatal(err)
	}
	f.db.Model(&f.employee).Update("agreement_id", 0)
	file := fixtureExcel(t, f, false)
	excelCell(t, file, "Работы", "C2", "Новое название")
	data := excelBytes(t, file)
	options := WorkImportOptions{Mode: "merge"}
	r := previewExcel(t, f, data, options)
	confirmExcel(t, f, data, options, r.Token)
	// Исторический участник сохраняется в своей смене, но не добавляется в новую.
	file = fixtureExcel(t, f, false)
	s := fixtureState(t, f)
	for i, v := range []string{"new:s", f.day.Date, s.Refs["shops"][workKey(f.shop.ID)], "2"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		excelCell(t, file, "Смены", cell, v)
	}
	for i, v := range []string{"new:e", "new:s", s.Refs["employees"][workKey(f.employee.ID)], "13"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		excelCell(t, file, "Сотрудники", cell, v)
	}
	r, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), options, "", false)
	if err != nil || len(r.Errors) == 0 {
		t.Fatal("transferred new participant", r, err)
	}
	f.db.Model(&f.employee).Update("agreement_id", f.a.ID)
	file = fixtureExcel(t, f, false)
	s = fixtureState(t, f)
	for i, v := range []string{"new:s", f.day.Date, s.Refs["shops"][workKey(f.shop.ID)], "2"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		excelCell(t, file, "Смены", cell, v)
	}
	for i, v := range []string{"new:e", "new:s", s.Refs["employees"][workKey(f.employee.ID)], "13"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		excelCell(t, file, "Сотрудники", cell, v)
	}
	r, err = ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, excelBytes(t, file), options, "", false)
	if err != nil || !strings.Contains(strings.Join(r.Errors, " "), "24") {
		t.Fatal("daily limit", r, err)
	}
}

func TestWorkExcelStaleStockAndForeignBook(t *testing.T) {
	f := newWorkFixture(t)
	file := fixtureExcel(t, f, false)
	data := excelBytes(t, file)
	options := WorkImportOptions{Mode: "merge"}
	r := previewExcel(t, f, data, options)
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, 0, WarehouseReceiptInput{Date: "2028-02-03", Quantity: "1"}, false); err != nil {
		t.Fatal(err)
	}
	result, err := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, options, r.Token, true)
	if err == nil && len(result.Errors) == 0 {
		t.Fatal("stale stock accepted")
	}
	other, err := CreateWorkBook(f.db, f.a.ID, 2028, 3)
	if err != nil {
		t.Fatal(err)
	}
	result, err = ImportWorkOrderExcel(f.db, f.a.ID, other.ID, data, options, "", false)
	if err == nil && len(result.Errors) == 0 {
		t.Fatal("foreign book accepted")
	}
}

// --------- новый код chat: необязательные поля проходят форму, Excel и повторное открытие
func TestWorkExcelOptionalTaskQuantityAndUnit(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].UnitID = 0
	in.Tasks[0].Quantity = ""
	shift, _, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	file := fixtureExcel(t, f, false)
	for _, cell := range []string{"D2", "E2"} {
		value, err := file.GetCellValue("Работы", cell)
		if err != nil || value != "" {
			t.Fatal("optional cell not blank", cell, value, err)
		}
	}
	options := WorkImportOptions{Mode: "merge"}
	report := previewExcel(t, f, excelBytes(t, file), options)
	if len(report.Changes) != 0 {
		t.Fatal("blank round trip changed data", report.Changes)
	}
	// Каждое поле можно заполнить независимо от другого.
	in.Version = shift.Version
	in.Tasks[0].Quantity = "2.5"
	shift, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = shift.Version
	in.Tasks[0].UnitID = f.unit.ID
	in.Tasks[0].Quantity = ""
	shift, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	file = fixtureExcel(t, f, false)
	excelCell(t, file, "Работы", "D2", "")
	data := excelBytes(t, file)
	report = previewExcel(t, f, data, options)
	confirmExcel(t, f, data, options, report.Token)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil || book.Shifts[0].Tasks[0].UnitID != 0 || book.Shifts[0].Tasks[0].QuantityHundredths != 0 {
		t.Fatal("clear optional fields", err)
	}
	in.Version = book.Shifts[0].Version
	in.Tasks[0].UnitID = 999999
	if _, _, err := SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("unknown unit accepted")
	}
	in.Tasks[0].UnitID = 0
	in.Tasks[0].Quantity = "-1"
	if _, _, err := SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("negative quantity accepted")
	}
}

// --------- конец нового кода chat
