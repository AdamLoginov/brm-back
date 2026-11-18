// --------- новый код chat: атомарность сохранения входящего письма
package mailworker

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"project/internal/models"
	"strings"
	"testing"
)

func TestIncomingAttachmentStorage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("UPLOAD_ROOT", filepath.Join(dir, "uploads"))
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handle, _ := db.DB()
	defer handle.Close()
	if err := db.AutoMigrate(&models.StoredFile{}, &models.Email{}, &models.Attachment{}); err != nil {
		t.Fatal(err)
	}
	raw := "From: sender@example.com\r\nTo: receiver@example.com\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=test\r\n\r\n--test\r\nContent-Type: text/plain\r\n\r\nHello\r\n--test\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=passport.pdf\r\n\r\n%PDF-test\r\n--test--\r\n"
	email := models.Email{Title: "Test"}
	if err := saveIncomingEmail(db, strings.NewReader(raw), &email); err != nil {
		t.Fatal(err)
	}
	if len(email.Attachments) != 1 || email.Attachments[0].StoredFileID == nil {
		t.Fatal(email.Attachments)
	}
	att := email.Attachments[0]
	if !strings.Contains(filepath.ToSlash(att.FilePath), "mail/incoming/") {
		t.Fatal(att.FilePath)
	}
	if _, err := os.Stat(att.FilePath); err != nil {
		t.Fatal(err)
	}
	// Ошибка создания письма после сохранения вложения должна откатить файл и метаданные.
	if err := db.Callback().Create().Before("gorm:create").Register("fail_email", func(tx *gorm.DB) {
		if tx.Statement.Table == "emails" {
			tx.AddError(os.ErrPermission)
		}
	}); err != nil {
		t.Fatal(err)
	}
	failed := models.Email{Title: "Fail"}
	if err := saveIncomingEmail(db, strings.NewReader(raw), &failed); err == nil {
		t.Fatal("expected failure")
	}
	var count int64
	db.Model(&models.StoredFile{}).Count(&count)
	if count != 1 {
		t.Fatal("orphan metadata", count)
	}
	files := 0
	filepath.WalkDir(filepath.Join(dir, "uploads", "mail"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	})
	if files != 1 {
		t.Fatal("orphan files", files)
	}
}

// --------- конец нового кода chat
