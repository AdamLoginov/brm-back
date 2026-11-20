// --------- новый код chat: проверка реальных XLSX, без изменения рабочей базы
package service

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func excelBytes(t *testing.T, f *excelize.File) []byte {
	t.Helper()
	b, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func fixtureState(t *testing.T, f workFixture) workExcelState {
	t.Helper()
	s, err := loadWorkExcelState(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func excelCell(t *testing.T, f *excelize.File, sheet, cell, value string) {
	t.Helper()
	if err := f.SetCellStr(sheet, cell, value); err != nil {
		t.Fatal(err)
	}
}
