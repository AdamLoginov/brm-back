// --------- новый код chat: проверки импорта журнала и необязательных полей
package service

import (
	"project/internal/models"
	"testing"
)

func previewExcel(t *testing.T, f workFixture, data []byte) WorkImportReport {
	t.Helper()
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, "", false)
	if e != nil || len(r.Errors) > 0 || r.Token == "" {
		t.Fatal("preview", e, r.Errors)
	}
	return r
}
func confirmExcel(t *testing.T, f workFixture, data []byte, token string) WorkImportReport {
	t.Helper()
	r, e := ImportWorkOrderExcel(f.db, f.a.ID, f.book.ID, data, token, true)
	if e != nil || len(r.Errors) > 0 || !r.Applied {
		t.Fatal("confirm", e, r.Errors)
	}
	return r
}

func TestWorkOptionalTaskQuantityAndUnit(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].UnitID = 0
	in.Tasks[0].Quantity = ""
	shift, _, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil {
		t.Fatal(err)
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
	in.Version = shift.Version
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

// --------- новый код chat: пустые часы и расположение, сохранение блока без позиции, история склада
func TestWorkOptionalHoursAndLocation(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks[0].PositionID = 0
	in.Tasks[0].Hours = ""
	shift, warning, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil || warning == "" {
		t.Fatal("optional fields", err, warning)
	}
	usages, err := WarehouseUsages(f.db, f.material.ID)
	if err != nil || len(usages) != 1 || usages[0].PositionNumber != "" {
		t.Fatal("usage without position missing", err, usages)
	}
	in.Version = shift.Version
	in.Tasks[0].BlockID = f.position.BlockID
	in.Tasks[0].Hours = "0"
	shift, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil {
		t.Fatal(err)
	}
	task := book.Shifts[0].Tasks[0]
	if task.BlockID != f.position.BlockID || task.PositionID != 0 || task.HoursNotSpecified || task.HoursHundredths != 0 {
		t.Fatal("block without position or explicit zero lost", task)
	}
	usages, err = WarehouseUsages(f.db, f.material.ID)
	if err != nil || len(usages) != 1 || usages[0].BlockName != "Блок" {
		t.Fatal("block in history", err, usages)
	}
	// новый код chat: повторное очищение необязательных полей
	in.Version = shift.Version
	in.Tasks[0].BlockID = 0
	in.Tasks[0].Hours = ""
	if _, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in); err != nil {
		t.Fatal(err)
	}
	book, _ = LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if book.Shifts[0].Tasks[0].BlockID != 0 || !book.Shifts[0].Tasks[0].HoursNotSpecified {
		t.Fatal("clear failed")
	}
	// Необязательность не позволяет выбрать чужой блок или несовместимую позицию.
	foreign := models.AgreementWorkBlock{AgreementID: f.a.ID, WorkshopID: f.other.ID, Name: "Другой", NameKey: "другой"}
	f.db.Create(&foreign)
	in.Version = book.Shifts[0].Version
	in.Tasks[0].BlockID = foreign.ID
	if _, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("foreign workshop block accepted")
	}
	in.Tasks[0].PositionID = f.position.ID
	if _, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("mismatched block and position accepted")
	}
}

// --------- конец нового кода chat
