package handlers

import (
	"fmt"
	"net/http"
	"os"
	"project/internal/database"
	"project/internal/models"
)

func DownloadAttachmenthandler(w http.ResponseWriter, r *http.Request) {
	var attachment models.Attachment
	var idAttachment = r.PathValue("id")

	if err := database.DB.First(&attachment, idAttachment).Error; err != nil {
		http.Error(w, "Ошибка при поиске файла в БД", http.StatusInternalServerError)
		return
	}

	if _, err := os.Stat(attachment.FilePath); os.IsNotExist(err) {
		http.Error(w, "Физический файл отсутствует на сервере", http.StatusNotFound)
		return
	}

	if attachment.MimeType != "" {
		w.Header().Set("Content-Type", attachment.MimeType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", attachment.FileName))
	http.ServeFile(w, r, attachment.FilePath)
}
