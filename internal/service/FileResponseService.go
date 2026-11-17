// --------- новый код chat: безопасное открытие документов на домене приложения
package service

import (
	"mime"
	"net/http"
)

func SetFileResponseHeaders(w http.ResponseWriter, name, contentType string, download bool) {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "application/octet-stream"
	}
	inline := map[string]bool{"application/pdf": true, "image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true, "image/avif": true, "text/plain": true}
	disposition := "inline"
	if download || !inline[mediaType] {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
}

// --------- конец нового кода chat
