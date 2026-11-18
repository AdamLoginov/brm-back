package handlers

import (
	"net/http"
	"os"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service" // новый код chat
)

func DownloadAttachmenthandler(w http.ResponseWriter, r *http.Request) {
	var attachment models.Attachment
	var idAttachment = r.PathValue("id")

	if err := database.DB.First(&attachment, idAttachment).Error; err != nil {
		http.Error(w, "Ошибка при поиске файла в БД", http.StatusInternalServerError)
		return
	}

	// --------- новый код chat: единое хранилище и совместимость
	filePath, err := service.FileLocation(database.DB, attachment.StoredFileID, attachment.FilePath)
	if err != nil {
		http.Error(w, "Не удалось найти файл", 404)
		return
	}
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, "Физический файл отсутствует на сервере", http.StatusNotFound)
		return
	}

	service.SetFileResponseHeaders(w, attachment.FileName, attachment.MimeType, true) // новый код chat: просмотр безопасных типов и скачивание остальных

	http.ServeFile(w, r, filePath) // новый код chat
}
