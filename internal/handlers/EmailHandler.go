package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/mailer"
	"project/internal/middleware"
	"project/internal/models"
	"project/internal/service"
	"strconv"
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
	// новый код chat: ограничение размера и очистка временных частей
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxStoredFileSize+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "Ошибка при парсинге формы", http.StatusBadRequest)
		return
	}

	defer r.MultipartForm.RemoveAll()
	service.FileStorageMutex.Lock()
	defer service.FileStorageMutex.Unlock()
	batch := &service.FileStorageBatch{}
	defer batch.Close()
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

	// --------- новый код chat: вложение исходящего письма
	file, header, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		attachment, saveErr := service.SaveMailAttachment(tx, batch, header.Filename, file, "outgoing", middleware.CurrentUserID(r))
		if saveErr != nil {
			log.Printf("[UPLOAD ERROR] %v", saveErr)
			http.Error(w, "Не удалось сохранить вложение", 400)
			return
		}
		email.Attachments = append(email.Attachments, attachment)
	} else if !errors.Is(err, http.ErrMissingFile) {
		http.Error(w, "Некорректное вложение", 400)
		return
	}
	// --------- конец нового кода chat

	if err := tx.Create(&email).Error; err != nil {
		http.Error(w, "Ошибка при сохранении данных в БД", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Ошибка фиксации транзакции", http.StatusInternalServerError)
		return
	}

	batch.Commit() // новый код chat

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
