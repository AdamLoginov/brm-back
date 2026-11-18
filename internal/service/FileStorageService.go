// --------- новый код chat: общее локальное хранилище с читаемыми путями и проверкой SHA-256
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"gorm.io/gorm"
	"project/internal/models"
)

const MaxStoredFileSize int64 = 64 << 20

var ErrFileTooLarge = errors.New("файл превышает 64 МБ")
var FileStorageMutex sync.Mutex

func UploadRoot() string {
	root := os.Getenv("UPLOAD_ROOT")
	if root == "" {
		root = "uploads"
	}
	return root
}

// Относительный ключ не зависит от расположения проекта на Windows или Linux.
func StoragePath(key string) (string, error) {
	root, err := filepath.Abs(UploadRoot())
	if err != nil {
		return "", err
	}
	key = strings.ReplaceAll(key, "\\", "/")
	if key == "" || filepath.IsAbs(key) {
		return "", errors.New("некорректный ключ файла")
	}
	target := filepath.Join(root, filepath.FromSlash(key))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("путь вне хранилища")
	}
	// Не допускаем символьные ссылки внутри хранилища.
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("символьная ссылка в пути файла")
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return target, nil
}

func SafeFilePart(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune("<>:\"/\\|?*", r) {
			return '_'
		}
		return r
	}, value)
	value = strings.Trim(strings.Join(strings.Fields(value), " "), ". ")
	if value == "" {
		value = "Без названия"
	}
	runes := []rune(value)
	if len(runes) > 80 {
		value = string(runes[:80])
	}
	stem := strings.ToUpper(strings.Split(value, ".")[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9') {
		value = "_" + value
	}
	return value
}

func SafeOriginalName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := filepath.Ext(name)
	if len(ext) > 12 {
		ext = ""
	}
	return SafeFilePart(strings.TrimSuffix(name, ext)) + SafeExtension(ext)
}
func SafeExtension(ext string) string {
	for _, r := range ext {
		if r != '.' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ""
		}
	}
	return ext
}

func EmployeeDocumentKey(employee models.EmployeeCard, category models.CategoryEmployeeDocument, document models.EmployeeDocuments) string {
	part := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	fullName := strings.Join(strings.Fields(part(employee.Surname)+" "+employee.Name+" "+part(employee.MiddleName)), " ")
	folder := fmt.Sprintf("employees/%d_%s", employee.ID, SafeFilePart(fullName))
	if strings.EqualFold(strings.TrimSpace(category.CategoryName), "Фото") {
		folder += "/Фото"
	} else {
		categoryName := category.CategoryName
		if categoryName == "" {
			categoryName = "Без категории"
		}
		folder += "/Документы/" + SafeFilePart(categoryName)
	}
	return fmt.Sprintf("%s/%d_%s", folder, document.ID, SafeOriginalName(document.FileName))
}

// Batch удаляет только созданные им копии при откате БД. Старые файлы никогда не удаляет.
type FileStorageBatch struct {
	created   []string
	committed bool
}

func (b *FileStorageBatch) Commit() { b.committed = true }
func (b *FileStorageBatch) Close() {
	if !b.committed {
		for _, p := range b.created {
			_ = os.Remove(p)
		}
	}
}

func fileDigest(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	h := sha256.New()
	n, err := io.Copy(h, file)
	return hex.EncodeToString(h.Sum(nil)), n, err
}

func (b *FileStorageBatch) Save(tx *gorm.DB, reader io.Reader, original, kind string, author uint, key func(uint) string) (*models.StoredFile, error) {
	tempDir, err := StoragePath("tmp")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(tempDir, 0700); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(tempDir, "upload-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(temp, h), io.LimitReader(reader, MaxStoredFileSize+1))
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if n > MaxStoredFileSize {
		return nil, ErrFileTooLarge
	}
	if n == 0 {
		return nil, errors.New("пустой файл")
	}
	if syncErr != nil {
		return nil, syncErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	sample, err := os.Open(temp.Name())
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	count, _ := sample.Read(buffer)
	sample.Close()
	record := models.StoredFile{OriginalName: original, Kind: kind, UploadedBy: author, Size: n, MIMEType: http.DetectContentType(buffer[:count]), SHA256: hex.EncodeToString(h.Sum(nil))}
	// Временный ключ уникален даже до присвоения ID.
	record.StorageKey = "tmp/" + filepath.Base(temp.Name())
	if err = tx.Create(&record).Error; err != nil {
		return nil, err
	}
	record.StorageKey = key(record.ID)
	if err = b.publish(temp.Name(), record.StorageKey, record.SHA256); err != nil {
		return nil, err
	}
	if err = tx.Model(&record).Update("storage_key", record.StorageKey).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (b *FileStorageBatch) publish(source, key, digest string) error {
	target, err := StoragePath(key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	// Hard link публикует завершённый файл атомарно, никогда не перезаписывая существующий.
	err = os.Link(source, target)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
		existing, _, e := fileDigest(target)
		if e != nil || existing != digest {
			return errors.New("целевой файл уже существует и отличается")
		}
		return nil
	}
	b.created = append(b.created, target)
	actual, _, err := fileDigest(target)
	if err != nil {
		return err
	}
	if actual != digest {
		return errors.New("контрольная сумма копии не совпала")
	}
	return nil
}

func (b *FileStorageBatch) Copy(tx *gorm.DB, source, key, original, kind string, author uint) (*models.StoredFile, error) {
	file, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return b.Save(tx, file, original, kind, author, func(uint) string { return key })
}

func FileLocation(tx *gorm.DB, storedID *uint, legacy string) (string, error) {
	if storedID != nil {
		var file models.StoredFile
		if err := tx.First(&file, *storedID).Error; err != nil {
			return "", err
		}
		return StoragePath(file.StorageKey)
	}
	// Совместимость со старыми записями до запуска отдельной миграции.
	if legacy == "" {
		return "", os.ErrNotExist
	}
	return filepath.Clean(legacy), nil
}

func SaveEmployeeDocument(tx *gorm.DB, b *FileStorageBatch, employee models.EmployeeCard, category models.CategoryEmployeeDocument, original string, reader io.Reader, author uint) (*models.EmployeeDocuments, error) {
	doc := models.EmployeeDocuments{EmployeeCardID: employee.ID, CategoryEmployeeDocumentID: category.ID, FileName: original}
	if err := tx.Create(&doc).Error; err != nil {
		return nil, err
	}
	file, err := b.Save(tx, reader, original, "employee", author, func(uint) string { return EmployeeDocumentKey(employee, category, doc) })
	if err != nil {
		return nil, err
	}
	doc.StoredFileID = &file.ID
	doc.FileSize = file.Size
	doc.MimeType = file.MIMEType
	doc.FilePath, _ = StoragePath(file.StorageKey)
	if err = tx.Omit("CategoryEmployeeDocument").Save(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func SaveMailAttachment(tx *gorm.DB, b *FileStorageBatch, original string, reader io.Reader, direction string, author uint) (models.Attachment, error) {
	if direction != "incoming" && direction != "outgoing" {
		return models.Attachment{}, errors.New("неверное направление почты")
	}
	file, err := b.Save(tx, reader, original, "mail_"+direction, author, func(id uint) string {
		return fmt.Sprintf("mail/%s/%s/%d_%s", direction, time.Now().UTC().Format("2006/01"), id, SafeOriginalName(original))
	})
	if err != nil {
		return models.Attachment{}, err
	}
	path, _ := StoragePath(file.StorageKey)
	return models.Attachment{StoredFileID: &file.ID, FileName: original, FilePath: path, FileSize: file.Size, MimeType: file.MIMEType}, nil
}

// При переименовании сотрудника меняются только уже перенесённые документы.
// Копии на старом месте сохраняются до отдельной контролируемой очистки.
func SyncEmployeeStorage(tx *gorm.DB, b *FileStorageBatch, employee models.EmployeeCard) error {
	if !tx.Migrator().HasColumn(&models.EmployeeDocuments{}, "stored_file_id") {
		return nil
	}
	var docs []models.EmployeeDocuments
	if err := tx.Unscoped().Where("employee_card_id = ? AND stored_file_id IS NOT NULL", employee.ID).Find(&docs).Error; err != nil {
		return err
	}
	for _, doc := range docs {
		var category models.CategoryEmployeeDocument
		if doc.CategoryEmployeeDocumentID != 0 {
			err := tx.Unscoped().First(&category, doc.CategoryEmployeeDocumentID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		var stored models.StoredFile
		if err := tx.First(&stored, *doc.StoredFileID).Error; err != nil {
			return err
		}
		key := EmployeeDocumentKey(employee, category, doc)
		if key == stored.StorageKey {
			continue
		}
		source, err := StoragePath(stored.StorageKey)
		if err != nil {
			return err
		}
		digest, _, err := fileDigest(source)
		if err != nil {
			return err
		}
		if digest != stored.SHA256 {
			return errors.New("контрольная сумма исходного документа изменилась")
		}
		// Не связываем новое имя напрямую со старым файлом: независимая проверенная копия.
		tempDir, err := StoragePath("tmp")
		if err != nil {
			return err
		}
		if err = os.MkdirAll(tempDir, 0700); err != nil {
			return err
		}
		temp, err := os.CreateTemp(tempDir, "rename-*")
		if err != nil {
			return err
		}
		src, err := os.Open(source)
		if err != nil {
			temp.Close()
			os.Remove(temp.Name())
			return err
		}
		_, err = io.Copy(temp, src)
		src.Close()
		syncErr := temp.Sync()
		closeErr := temp.Close()
		if err == nil {
			err = syncErr
		}
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = b.publish(temp.Name(), key, digest)
		}
		os.Remove(temp.Name())
		if err != nil {
			return err
		}
		if err = tx.Model(&stored).Update("storage_key", key).Error; err != nil {
			return err
		}
		path, _ := StoragePath(key)
		if err = tx.Unscoped().Model(&models.EmployeeDocuments{}).Where("id = ?", doc.ID).Update("file_path", path).Error; err != nil {
			return err
		}
	}
	return nil
}

func CompanyCardPath(tx *gorm.DB) (string, error) {
	if tx.Migrator().HasTable(&models.StoredFile{}) {
		var file models.StoredFile
		err := tx.Where("kind = ?", "company").Order("id DESC").First(&file).Error
		if err == nil {
			return StoragePath(file.StorageKey)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}
	return filepath.Join(UploadRoot(), "company_card.pdf"), nil
}

// --------- конец нового кода chat
