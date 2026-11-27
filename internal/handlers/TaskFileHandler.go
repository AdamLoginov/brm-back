package handlers

// --------- новый код chat: файлы сохраняются вместе с изменениями задачи
import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"mime"
	"mime/multipart"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
)

func saveTaskRequest(w http.ResponseWriter, r *http.Request, actor uint, update bool) {
	var input models.Task
	var files []*multipart.FileHeader
	var removed []uint
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media == "multipart/form-data" {
		r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			http.Error(w, "Не удалось принять файлы. Максимальный размер запроса 128 МБ.", 400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if json.Unmarshal([]byte(r.FormValue("task")), &input) != nil || json.Unmarshal([]byte(r.FormValue("removed")), &removed) != nil {
			http.Error(w, "Некорректные данные задачи", 400)
			return
		}
		files = r.MultipartForm.File["files"]
		for _, file := range files {
			if file.Size == 0 || file.Size > service.MaxStoredFileSize {
				http.Error(w, "Размер каждого файла должен быть больше нуля и не более 64 МБ", 400)
				return
			}
		}
	} else if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		http.Error(w, "Некорректная задача", 400)
		return
	}
	if len(files) > 0 { // новый код chat: откат файлов завершается до следующей загрузки
		service.FileStorageMutex.Lock()
		defer service.FileStorageMutex.Unlock()
	}
	var batch service.FileStorageBatch
	defer batch.Close()
	var changes []func(*gorm.DB, uint) error
	if len(files)+len(removed) > 0 {
		changes = append(changes, func(tx *gorm.DB, id uint) error {
			for _, fid := range removed {
				result := tx.Where("id = ? AND task_id = ?", fid, id).Delete(&models.TaskAttachment{})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return gorm.ErrRecordNotFound
				}
			}
			for _, header := range files {
				src, err := header.Open()
				if err != nil {
					return err
				}
				stored, err := batch.Save(tx, src, header.Filename, "task", actor, func(fileID uint) string {
					return fmt.Sprintf("tasks/%d/%d_%s", id, fileID, service.SafeOriginalName(header.Filename))
				})
				src.Close()
				if err != nil {
					return err
				}
				if err = tx.Create(&models.TaskAttachment{TaskID: id, StoredFileID: stored.ID, Name: header.Filename, MIME: stored.MIMEType}).Error; err != nil {
					return err
				}
			}
			return nil
		})
	}
	var task models.Task
	var err error
	if update {
		task, err = service.UpdateTaskWithNotifications(database.DB, actor, input, false, changes...)
	} else {
		task, err = service.CreateTaskWithNotifications(database.DB, actor, input, changes...)
	}
	if err == nil {
		batch.Commit()
	}
	notificationResponse(w, task, err)
}

func TaskFileHandler(w http.ResponseWriter, r *http.Request) {
	id := taskRequestID(w, r)
	if id == 0 {
		return
	}
	var file models.TaskAttachment
	err := database.DB.Joins("JOIN tasks ON tasks.id = task_attachments.task_id AND tasks.deleted_at IS NULL").Where("task_attachments.id = ?", id).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Ошибка получения файла", 500)
		return
	}
	path, err := service.FileLocation(database.DB, &file.StoredFileID, "")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	// новый код chat: разрешённые изображения и PDF открываются в браузере
	mediaType, _, _ := mime.ParseMediaType(file.MIME)
	download := true
	switch mediaType {
	case "application/pdf", "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif":
		download = false
	}
	service.SetFileResponseHeaders(w, file.Name, file.MIME, download)
	http.ServeFile(w, r, path)
}
