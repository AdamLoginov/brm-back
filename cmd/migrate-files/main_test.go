// --------- новый код chat: проверка команды и резервной копии SQLite
package main

import (
	"flag"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestCommandPreviewAndApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	root := filepath.Join(dir, "uploads")
	t.Setenv("UPLOAD_ROOT", root)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.EmployeeCard{}, &models.EmployeeDocuments{}, &models.Attachment{}); err != nil {
		t.Fatal(err)
	}
	employee := models.EmployeeCard{Name: "Иван"}
	db.Create(&employee)
	os.MkdirAll(root, 0700)
	source := filepath.Join(root, "original.pdf")
	os.WriteFile(source, []byte("%PDF-original"), 0600)
	doc := models.EmployeeDocuments{EmployeeCardID: employee.ID, FileName: "Паспорт.pdf", FilePath: source}
	db.Create(&doc)
	sqlDB, _ := db.DB()
	sqlDB.Close()
	oldFlags, oldArgs := flag.CommandLine, os.Args
	defer func() { flag.CommandLine = oldFlags; os.Args = oldArgs }()
	for _, apply := range []bool{false, true} {
		flag.CommandLine = flag.NewFlagSet("migration", flag.ContinueOnError)
		os.Args = []string{"migration", "-db", path, "-root", root, "-report", filepath.Join(dir, "report.json")}
		if apply {
			os.Args = append(os.Args, "-apply")
		}
		if err := run(); err != nil {
			t.Fatal(err)
		}
		backups, err := filepath.Glob(path + ".before-file-migration-*.bak")
		if err != nil {
			t.Fatal(err)
		}
		if !apply && len(backups) != 0 {
			t.Fatal("preview created backup")
		}
		if apply {
			if len(backups) != 1 {
				t.Fatal("missing backup")
			}
			backup, err := gorm.Open(sqlite.Open(backups[0]), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if backup.Migrator().HasTable(&models.StoredFile{}) {
				t.Fatal("backup taken after schema changes")
			}
			var original models.EmployeeDocuments
			backup.First(&original, doc.ID)
			if original.StoredFileID != nil || original.FilePath != source {
				t.Fatal("invalid backup")
			}
			handle, _ := backup.DB()
			handle.Close()
		}
	}
}

// --------- конец нового кода chat
