// --------- новый код chat: корректировка поступлений, возврат и конкурентное списание
package service

import (
	"project/internal/models"
	"sync"
	"testing"
)

func TestWarehouseReceiptCorrection(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	shift, _, err := SaveWorkShift(f.db, f.a.ID, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	var receipt models.WarehouseReceipt
	f.db.Where("material_id = ?", f.material.ID).First(&receipt)
	usages, err := WarehouseUsages(f.db, f.material.ID)
	if err != nil || len(usages) != 1 {
		t.Fatal(err)
	}
	change := WarehouseReceiptInput{Date: receipt.Date, Quantity: "50"}
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, receipt.ID, change, false); err == nil {
		t.Fatal("negative balance accepted")
	}
	var old models.WarehouseReceipt
	f.db.First(&old, receipt.ID)
	if old.QuantityThousandths != 100125 {
		t.Fatal("receipt changed on failure")
	}
	change.RemoveUsageIDs = []uint{usages[0].ID}
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, receipt.ID, change, false); err != nil {
		t.Fatal(err)
	}
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 50000 {
		t.Fatal(balance)
	}
	in.Version = shift.Version
	if _, _, err := SaveWorkShift(f.db, f.a.ID, shift.ID, in); err == nil {
		t.Fatal("stale form restored removed usage")
	}
	var taskCount int64
	f.db.Model(&models.WorkOrderTask{}).Count(&taskCount)
	if taskCount != 1 {
		t.Fatal("work removed")
	}
	if err := SaveWarehouseReceipt(f.db, f.a.ID, f.material.ID, receipt.ID, WarehouseReceiptInput{}, true); err != nil {
		t.Fatal(err)
	}
	balance, _ = WarehouseBalance(f.db, f.material.ID)
	if balance != 0 {
		t.Fatal(balance)
	}
	if err := DeleteWarehouseMaterial(f.db, f.a.ID, f.material.ID); err != nil {
		t.Fatal(err)
	}
}
func TestWarehouseMaterialDeletionAndConcurrentUse(t *testing.T) {
	f := newWorkFixture(t)
	in := f.input()
	in.Employees[0].Hours = "6"
	in.Tasks[0].Hours = "6"
	second := f.input()
	second.Number = 2
	second.Employees[0].Hours = "6"
	second.Tasks[0].Hours = "6"
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, input := range []WorkShiftInput{in, second} {
		wg.Add(1)
		go func(v WorkShiftInput) { defer wg.Done(); _, _, err := SaveWorkShift(f.db, f.a.ID, 0, v); errs <- err }(input)
	}
	wg.Wait()
	close(errs)
	success := 0
	for err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent successes %d", success)
	}
	balance, _ := WarehouseBalance(f.db, f.material.ID)
	if balance != 20000 {
		t.Fatal(balance)
	}
	if err := DeleteWarehouseMaterial(f.db, f.a.ID, f.material.ID); err != nil {
		t.Fatal(err)
	}
	var uses, receipts, tasks int64
	f.db.Model(&models.WorkOrderMaterial{}).Count(&uses)
	f.db.Model(&models.WarehouseReceipt{}).Count(&receipts)
	f.db.Model(&models.WorkOrderTask{}).Count(&tasks)
	if uses != 0 || receipts != 0 || tasks != 1 {
		t.Fatal("incorrect cascade", uses, receipts, tasks)
	}
}

// --------- конец нового кода chat
