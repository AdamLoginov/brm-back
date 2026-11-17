// --------- новый код chat: выдача файловой cookie по действующему Bearer и выход
package handlers

import (
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
)

func FileSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !service.FileRequestOriginAllowed(r) {
		http.Error(w, "Origin not allowed", 403)
		return
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		http.Error(w, "Unauthorized", 401)
		return
	}
	token, err := jwt.Parse(strings.TrimPrefix(header, "Bearer "), func(t *jwt.Token) (any, error) { return config.JWTKey, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", 401)
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	id, ok := claims["user_id"].(float64)
	if !ok || id <= 0 || id != float64(uint(id)) {
		http.Error(w, "Unauthorized", 401)
		return
	}
	var user models.User
	if err = database.DB.Select("id").First(&user, uint(id)).Error; err != nil {
		http.Error(w, "Unauthorized", 401)
		return
	}
	expires, err := claims.GetExpirationTime()
	if err != nil || expires == nil {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if err = service.SetFileCookie(w, r, user.ID, expires.Time); err != nil {
		http.Error(w, "Cookie error", 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func FileLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if !service.FileRequestOriginAllowed(r) {
		http.Error(w, "Origin not allowed", 403)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	service.ClearFileCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// --------- конец нового кода chat
