// --------- новый код chat: перенос старых файлов без удаления оригиналов
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"project/internal/models"
	"strings"
	"time"
)

type FileMigrationRow struct {
	Kind        string `json:"kind"`
	ID          uint   `json:"id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// Команда запускается при остановленном backend; предварительный просмотр ничего не меняет.
func MigrateStoredFiles(db *gorm.DB, apply bool) ([]FileMigrationRow, error) {
	FileStorageMutex.Lock()
	defer FileStorageMutex.Unlock()
	if apply {
		if err := db.AutoMigrate(&models.StoredFile{}, &models.EmployeeDocuments{}, &models.Attachment{}); err != nil {
			return nil, err
		}
	}
	rows := []FileMigrationRow{}
	migrate := func(kind string, id uint, source, key, original string, storedID *uint, update func(*gorm.DB, *models.StoredFile) error) {
		row := FileMigrationRow{Kind: kind, ID: id, Source: source, Destination: key}
		if storedID != nil {
			path, err := FileLocation(db, storedID, "")
			var stored models.StoredFile
			if err == nil {
				err = db.First(&stored, *storedID).Error
			}
			if err == nil {
				row.Destination = stored.StorageKey
				row.SHA256, _, err = fileDigest(path)
				if err == nil && row.SHA256 != stored.SHA256 {
					err = errors.New("контрольная сумма отличается от БД")
				}
			}
			row.Status = "already_migrated"
			if err != nil {
				row.Status = "error"
				row.Error = err.Error()
			}
			rows = append(rows, row)
			return
		}
		// Старые относительные пути разрешаются от родителя uploads, независимо от cwd команды.
		root, err := filepath.Abs(UploadRoot())
		if err == nil {
			source = filepath.FromSlash(strings.ReplaceAll(source, "\\", "/"))
			if !filepath.IsAbs(source) {
				source = filepath.Join(filepath.Dir(root), source)
			}
			var rel string
			rel, err = filepath.Rel(root, source)
			if err == nil {
				source, err = StoragePath(filepath.ToSlash(rel))
			}
		}
		row.Source = source
		if err == nil {
			row.SHA256, _, err = fileDigest(source)
		}
		if err != nil {
			row.Status = "error"
			if os.IsNotExist(err) {
				row.Status = "missing"
			}
			row.Error = err.Error()
			rows = append(rows, row)
			return
		}
		row.Status = "ready"
		if apply {
			batch := &FileStorageBatch{}
			err = db.Transaction(func(tx *gorm.DB) error {
				stored, err := batch.Copy(tx, source, key, original, kind, 0)
				if err != nil {
					return err
				}
				if stored.SHA256 != row.SHA256 {
					return errors.New("исходный файл изменился во время переноса")
				}
				if update != nil {
					return update(tx, stored)
				}
				return nil
			})
			if err == nil {
				batch.Commit()
				row.Status = "migrated"
			} else {
				row.Status = "error"
				row.Error = err.Error()
			}
			batch.Close()
		}
		rows = append(rows, row)
	}
	var docs []models.EmployeeDocuments
	if err := db.Unscoped().Find(&docs).Error; err != nil {
		return rows, err
	}
	for _, doc := range docs {
		var employee models.EmployeeCard
		if err := db.Unscoped().First(&employee, doc.EmployeeCardID).Error; err != nil {
			rows = append(rows, FileMigrationRow{Kind: "employee", ID: doc.ID, Source: doc.FilePath, Status: "error", Error: "карточка сотрудника отсутствует"})
			continue
		}
		var category models.CategoryEmployeeDocument
		if doc.CategoryEmployeeDocumentID != 0 {
			err := db.Unscoped().First(&category, doc.CategoryEmployeeDocumentID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return rows, err
			}
		}
		migrate("employee", doc.ID, doc.FilePath, EmployeeDocumentKey(employee, category, doc), doc.FileName, doc.StoredFileID, func(tx *gorm.DB, file *models.StoredFile) error {
			path, err := StoragePath(file.StorageKey)
			if err != nil {
				return err
			}
			return tx.Unscoped().Model(&models.EmployeeDocuments{}).Where("id = ?", doc.ID).Updates(map[string]interface{}{"stored_file_id": file.ID, "file_path": path, "file_size": file.Size, "mime_type": file.MIMEType}).Error
		})
	}
	var attachments []models.Attachment
	if err := db.Unscoped().Find(&attachments).Error; err != nil {
		return rows, err
	}
	for _, att := range attachments {
		date := att.CreatedAt
		if date.IsZero() {
			date = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		key := fmt.Sprintf("mail/legacy/%s/%d_%s", date.Format("2006/01"), att.ID, SafeOriginalName(att.FileName))
		migrate("mail_legacy", att.ID, att.FilePath, key, att.FileName, att.StoredFileID, func(tx *gorm.DB, file *models.StoredFile) error {
			path, err := StoragePath(file.StorageKey)
			if err != nil {
				return err
			}
			return tx.Unscoped().Model(&models.Attachment{}).Where("id = ?", att.ID).Updates(map[string]interface{}{"stored_file_id": file.ID, "file_path": path, "file_size": file.Size, "mime_type": file.MIMEType}).Error
		})
	}
	var company models.StoredFile
	var companyID *uint
	if db.Migrator().HasTable(&models.StoredFile{}) {
		err := db.Where("kind = ?", "company").First(&company).Error
		if err == nil {
			companyID = &company.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return rows, err
		}
	}
	source, err := filepath.Abs(filepath.Join(UploadRoot(), "company_card.pdf"))
	if err != nil {
		return rows, err
	}
	if _, err := os.Stat(source); err == nil || companyID != nil {
		migrate("company", 0, source, "company/company_card.pdf", "company_card.pdf", companyID, nil)
	}
	return rows, nil
}

// --------- конец нового кода chat
