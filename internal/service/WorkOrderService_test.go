// --------- новый код chat: интеграция смен, часов и склада на временной SQLite
package service

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/models"
	"testing"
)

type workFixture struct {
	db          *gorm.DB
	a           models.Agreement
	shop, other models.AgreementWorkshop
	unit        models.MeasurementUnit
	position    models.AgreementWorkPosition
	employee    models.EmployeeCard
	material    models.WarehouseMaterial
	book        models.WorkOrderBook
	day         models.WorkOrderDay
}

func newWorkFixture(t *testing.T) workFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "work.db")+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	if err := db.AutoMigrate(&models.Agreement{}, &models.EmployeeCard{}, &models.MeasurementUnit{}, &models.AgreementWorkshop{}, &models.AgreementWorkBlock{}, &models.AgreementWorkPosition{}, &models.WorkOrderBook{}, &models.WorkOrderDay{}, &models.WorkOrderShift{}, &models.WorkOrderEmployee{}, &models.WorkOrderTask{}, &models.WarehouseMaterial{}, &models.WarehouseReceipt{}, &models.WorkOrderMaterial{}); err != nil {
		t.Fatal(err)
	}
	f := workFixture{db: db}
	f.a = models.Agreement{Name: "Договор"}
	db.Create(&f.a)
	f.unit = models.MeasurementUnit{Name: "м", NameKey: "м"}
	db.Create(&f.unit)
	f.shop = models.AgreementWorkshop{AgreementID: f.a.ID, Name: "Цех 1", NameKey: "цех 1"}
	f.other = models.AgreementWorkshop{AgreementID: f.a.ID, Name: "Цех 2", NameKey: "цех 2"}
	db.Create(&f.shop)
	db.Create(&f.other)
	b := models.AgreementWorkBlock{AgreementID: f.a.ID, WorkshopID: f.shop.ID, Name: "Блок", NameKey: "блок"}
	db.Create(&b)
	f.position = models.AgreementWorkPosition{AgreementID: f.a.ID, BlockID: b.ID, Number: "1013", Name: "Объект"}
	db.Create(&f.position)
	f.employee = models.EmployeeCard{Name: "Иван", AgreementID: f.a.ID}
	db.Create(&f.employee)
	f.material = models.WarehouseMaterial{AgreementID: f.a.ID, WorkshopID: f.shop.ID, UnitID: f.unit.ID, Name: "Кабель", NameKey: "кабель"}
	db.Create(&f.material)
	f.book, err = CreateWorkBook(db, f.a.ID, 2028, 2)
	if err != nil {
		t.Fatal(err)
	}
	db.Where("book_id = ?", f.book.ID).Order("date").First(&f.day)
	if err := SaveWarehouseReceipt(db, f.a.ID, f.material.ID, 0, WarehouseReceiptInput{Date: "2028-02-01", Quantity: "100.125"}, false); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f workFixture) input() WorkShiftInput {
	return WorkShiftInput{DayID: f.day.ID, WorkshopID: f.shop.ID, Number: 1, Employees: []WorkEmployeeInput{{EmployeeCardID: f.employee.ID, Hours: "12"}}, Tasks: []WorkTaskInput{{PositionID: f.position.ID, Name: "Монтаж", UnitID: f.unit.ID, Quantity: "2.5", Hours: "12", Materials: []WorkMaterialInput{{MaterialID: f.material.ID, Quantity: "80.125"}}}}}
}
func TestWorkShiftLifecycle(t *testing.T) {
	f := newWorkFixture(t)
	book, err := LoadWorkBook(f.db, f.a.ID, f.book.ID)
	if err != nil || len(book.Days) != 29 {
		t.Fatal("leap month", err, len(book.Days))
	}
	if _, err := CreateWorkBook(f.db, f.a.ID, 2028, 2); err == nil {
		t.Fatal("duplicate month")
	}
	in := f.input()
	shift, warning, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil || warning != "" {
		t.Fatal(err, warning)
	}
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 20000 {
		t.Fatal(balance)
	}
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err == nil {
		t.Fatal("duplicate shift")
	}
	in.Version = shift.Version
	in.Tasks[0].Hours = "13"
	// новый код chat: превышение сохраняется с предупреждением.
	shift, warning, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil || warning == "" {
		t.Fatal("over hours warning", err, warning)
	}
	in.Version = shift.Version
	balance, _ = WarehouseBalance(f.db, f.material.ID)
	if balance != 20000 {
		t.Fatal("failed save changed stock")
	}
	in.Tasks[0].Hours = "8"
	in.Tasks[0].Materials[0].Quantity = "100.126"
	if _, _, err := SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("overdraw accepted")
	}
	in.Tasks[0].Materials = nil
	shift, warning, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil || warning == "" {
		t.Fatal("under hours warning", err)
	}
	balance, _ = WarehouseBalance(f.db, f.material.ID)
	if balance != 100125 {
		t.Fatal("material not returned")
	}
	// Человек остаётся в старой смене после перевода, но не добавляется в новую.
	f.db.Model(&f.employee).Update("agreement_id", 0)
	in.Version = shift.Version
	shift, _, err = SaveWorkShift(f.db, f.a.ID, shift.ID, in)
	if err != nil {
		t.Fatal("historical employee lost", err)
	}
	in.Number = 2
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err == nil {
		t.Fatal("transferred employee added")
	}
	if err := DeleteWorkShift(f.db, f.a.ID, shift.ID, shift.Version-1); err == nil {
		t.Fatal("stale delete")
	}
	if err := DeleteWorkShift(f.db, f.a.ID, shift.ID, shift.Version); err != nil {
		t.Fatal(err)
	}
}
func TestWorkHoursAndWorkshopBoundaries(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Tasks = nil
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	in.WorkshopID = f.other.ID
	in.Number = 2
	in.Employees[0].Hours = "12.01"
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err == nil {
		t.Fatal("daily limit not checked")
	}
	in.Employees[0].Hours = "12"
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err != nil {
		t.Fatal(err)
	}
	otherAgreement := models.Agreement{Name: "Other"}
	f.db.Create(&otherAgreement)
	in = f.input()
	if _, _, err := SaveWorkShift(f.db, otherAgreement.ID, 0, in); err == nil {
		t.Fatal("foreign day accepted")
	}
	// Позиция другого цеха и материал другого цеха запрещены.
	in.WorkshopID = f.other.ID
	in.Number = 1
	in.Employees = nil
	in.Tasks[0].Hours = "0"
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err == nil {
		t.Fatal("foreign position accepted")
	}
	newDay := models.WorkOrderDay{}
	f.db.Where("book_id = ? AND id <> ?", f.book.ID, f.day.ID).First(&newDay)
	in = f.input()
	in.DayID = newDay.ID
	foreign := models.WarehouseMaterial{AgreementID: f.a.ID, WorkshopID: f.other.ID, Name: "Кабель", NameKey: "кабель", UnitID: f.unit.ID}
	f.db.Create(&foreign)
	in.Tasks[0].Materials[0].MaterialID = foreign.ID
	if _, _, err := SaveWorkShift(f.db, f.a.ID, 0, in); err == nil {
		t.Fatal("foreign material accepted")
	}
}

// --------- конец нового кода chat
