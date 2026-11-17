// --------- новый код chat: cookie принимается только файловыми GET/HEAD маршрутами
package middleware

import (
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
)

func FileAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", 405)
			return
		}
		id, err := service.FileCookieUser(r)
		if err != nil {
			http.Error(w, "Войдите в систему для просмотра файла", 401)
			return
		}
		var user models.User
		if err = database.DB.Select("id").First(&user, id).Error; err != nil {
			http.Error(w, "Пользователь недоступен", 401)
			return
		}
		next(w, r)
	}
}

// --------- конец нового кода chat
