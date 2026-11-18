// --------- новый код chat: проверки файлового хранилища и миграции
package service

import (
	"errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"project/internal/models"
	"strings"
	"testing"
)

func storageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	root := t.TempDir()
	t.Setenv("UPLOAD_ROOT", filepath.Join(root, "uploads"))
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.StoredFile{}, &models.EmployeeCard{}, &models.CategoryEmployeeDocument{}, &models.EmployeeDocuments{}, &models.Attachment{}); err != nil {
		t.Fatal(err)
	}
	return db
}
func fixtureEmployee(t *testing.T, db *gorm.DB) (models.EmployeeCard, models.CategoryEmployeeDocument) {
	t.Helper()
	surname := "Иванов"
	employee := models.EmployeeCard{Name: "Иван", Surname: &surname}
	category := models.CategoryEmployeeDocument{CategoryName: "Паспорт"}
	if err := db.Create(&employee).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	return employee, category
}
func TestStoragePathsAndRollback(t *testing.T) {
	db := storageTestDB(t)
	for _, key := range []string{"../escape", "../../etc/passwd", "..\\escape", ""} {
		if _, err := StoragePath(key); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
	if got := SafeOriginalName("../../Паспорт:Иванов.pdf"); got != "Паспорт_Иванов.pdf" {
		t.Fatal(got)
	}
	employee, category := fixtureEmployee(t, db)
	batch := &FileStorageBatch{}
	var doc *models.EmployeeDocuments
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		doc, err = SaveEmployeeDocument(tx, batch, employee, category, "Паспорт.pdf", strings.NewReader("%PDF-test"), 7)
		if err != nil {
			return err
		}
		return errors.New("rollback")
	})
	batch.Close()
	if err == nil || doc == nil {
		t.Fatal("expected rollback after save", err)
	}
	if _, err := os.Stat(doc.FilePath); !os.IsNotExist(err) {
		t.Fatal("file survived rollback", err)
	}
	var count int64
	db.Model(&models.StoredFile{}).Count(&count)
	if count != 0 {
		t.Fatal("metadata survived rollback")
	}
}
func TestStorageRenameAndCollision(t *testing.T) {
	db := storageTestDB(t)
	employee, category := fixtureEmployee(t, db)
	batch := &FileStorageBatch{}
	defer batch.Close()
	var doc *models.EmployeeDocuments
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		doc, err = SaveEmployeeDocument(tx, batch, employee, category, "Паспорт.pdf", strings.NewReader("%PDF-test"), 7)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	batch.Commit()
	oldPath := doc.FilePath
	surname := "Петров"
	employee.Surname = &surname
	rename := &FileStorageBatch{}
	defer rename.Close()
	if err := db.Transaction(func(tx *gorm.DB) error { return SyncEmployeeStorage(tx, rename, employee) }); err != nil {
		t.Fatal(err)
	}
	rename.Commit()
	db.First(doc, doc.ID)
	if !strings.Contains(doc.FilePath, "Петров") {
		t.Fatal(doc.FilePath)
	}
	for _, path := range []string{oldPath, doc.FilePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	var stored models.StoredFile
	db.First(&stored, *doc.StoredFileID)
	if stored.UploadedBy != 7 || len(stored.SHA256) != 64 {
		t.Fatal(stored)
	}
	if err := os.WriteFile(doc.FilePath, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	surname = "Сидоров"
	employee.Surname = &surname
	failed := &FileStorageBatch{}
	defer failed.Close()
	if err := db.Transaction(func(tx *gorm.DB) error { return SyncEmployeeStorage(tx, failed, employee) }); err == nil {
		t.Fatal("changed file accepted")
	}
}
func TestMigrationPreviewApplyIdempotentAndMissing(t *testing.T) {
	db := storageTestDB(t)
	employee, category := fixtureEmployee(t, db)
	path := filepath.Join(UploadRoot(), "documents", "old.pdf")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("%PDF-legacy"), 0600)
	doc := models.EmployeeDocuments{EmployeeCardID: employee.ID, CategoryEmployeeDocumentID: category.ID, FileName: "Старый паспорт.pdf", FilePath: path}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatal(err)
	}
	missing := models.EmployeeDocuments{EmployeeCardID: employee.ID, FileName: "Нет.pdf", FilePath: filepath.Join(UploadRoot(), "missing.pdf")}
	db.Create(&missing)
	rows, err := MigrateStoredFiles(db, false)
	if err != nil || len(rows) != 2 || rows[0].Status != "ready" || rows[1].Status != "missing" {
		t.Fatal(rows, err)
	}
	var count int64
	db.Model(&models.StoredFile{}).Count(&count)
	if count != 0 {
		t.Fatal("preview mutated DB")
	}
	rows, err = MigrateStoredFiles(db, true)
	if err != nil || rows[0].Status != "migrated" {
		t.Fatal(rows, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original removed", err)
	}
	db.First(&doc, doc.ID)
	resolved, err := FileLocation(db, doc.StoredFileID, doc.FilePath)
	if err != nil || !strings.Contains(resolved, "Иванов") {
		t.Fatal(resolved, err)
	}
	rows, err = MigrateStoredFiles(db, true)
	if err != nil || rows[0].Status != "already_migrated" {
		t.Fatal(rows, err)
	}
	db.Model(&models.StoredFile{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate metadata")
	}
}

// --------- конец нового кода chat
