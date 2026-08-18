package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"project/internal/database"
	"project/internal/mailer"
	"project/internal/models"
	"strconv"
	"time"
)

func GetAllEmailsHandler(w http.ResponseWriter, r *http.Request) {
	var emails []models.Email
	if err := database.DB.Preload("Dialog").Find(&emails).Error; err != nil {
		http.Error(w, "Получения данных!", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emails)
}

func GetDetailEmailHandler(w http.ResponseWriter, r *http.Request) {
	var emailId = r.PathValue("id")
	var email models.Email

	if err := database.DB.Preload("Attachments").First(&email, emailId).Error; err != nil {
		http.Error(w, "Ошибка в получении письма", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(email)
}

func CreateEmailHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Ошибка при парсинге формы", http.StatusBadRequest)
		return
	}

	tx := database.DB.Begin()
	defer tx.Rollback()

	dialogId, err := strconv.ParseUint(r.FormValue("dialog_id"), 10, 32)
	if err != nil {
		http.Error(w, "Ошибка в получении dialog_id", http.StatusBadRequest)
		return
	}

	email := models.Email{
		EmailTo:   r.FormValue("email_to"),
		EmailFrom: "mysnikov2000@mail.ru",
		Title:     r.FormValue("title"),
		Message:   r.FormValue("message"),
		IsRead:    true,
		DialogID:  uint(dialogId),
	}

	file, handler, err := r.FormFile("file")
	if err == nil {
		uniqueFileName := fmt.Sprintf("%d_%s", time.Now().Unix(), handler.Filename)
		filePath := filepath.Join("./uploads", uniqueFileName)
		dst, err := os.Create(filePath)
		if err != nil {
			log.Printf("[UPLOAD ERROR] Не удалось создать файл на диске: %v", err)
			http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Ошибка при записи файла", http.StatusInternalServerError)
			return
		}

		_, _ = file.Seek(0, 0)
		buffer := make([]byte, 512)
		_, _ = file.Read(buffer)

		// Go сам проанализирует байты и вернет точный MIME-тип (например "application/pdf")
		mimeType := http.DetectContentType(buffer)

		attachment := models.Attachment{
			FileName: handler.Filename,
			FilePath: filePath,
			FileSize: handler.Size,
			MimeType: mimeType,
		}

		email.Attachments = append(email.Attachments, attachment)
	}

	if err := tx.Create(&email).Error; err != nil {
		http.Error(w, "Ошибка при сохранении данных в БД", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Ошибка фиксации транзакции", http.StatusInternalServerError)
		return
	}

	go func(email models.Email) {
		if err := mailer.SendOrderEmail(email, false); err != nil {
			log.Printf("[ERROR] Не удалось отправить письмо для заказа №%d: %v", email.ID, err)
		} else {
			log.Printf("[SUCCESS] Письмо для заказа №%d успешно отправлено!", email.ID)
		}
	}(email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Заказ успешно создан!"})
}

func ReadEmailhandler(w http.ResponseWriter, r *http.Request) {
	var idEmail = r.PathValue("id")

	if err := database.DB.Model(&models.Email{}).Where("id = ?", idEmail).Update("isRead", true).Error; err != nil {
		log.Printf("[ERROR] Ошибка при обновлении поля isRead")
		http.Error(w, "[Error] ошибка при обновлении поля IsRead", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Поле isRead успешно обновлено"})
}
