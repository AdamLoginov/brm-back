package handlers

import (
	"encoding/json"
	"net/http"
	"project/internal/config"
	"project/internal/database"
	"project/internal/middleware" // новый код chat: проверенный пользователь для продления
	"project/internal/models"
	"project/internal/service" // новый код chat
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const sessionLifetime = 72 * time.Hour // новый код chat: три дня без активности

func GenerateToken(userID uint, login string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"login":   login,
		"exp":     time.Now().Add(sessionLifetime).Unix(), // новый код chat
	})

	return token.SignedString(config.JWTKey)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// новый код chat: не устанавливаем файловую cookie через посторонний сайт
	if !service.FileRequestOriginAllowed(r) {
		http.Error(w, "Origin not allowed", 403)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var user models.User
	resultLogin := database.DB.Where("login = ?", input.Login).First(&user)

	if resultLogin.Error != nil {
		http.Error(w, "Логин не найден", http.StatusUnauthorized)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)) != nil {
		http.Error(w, "Неверный пароль", http.StatusUnauthorized)
		return
	}

	token, err := GenerateToken(user.ID, user.Login)
	if err != nil {
		http.Error(w, "Unable to create token", http.StatusInternalServerError)
		return
	}
	// новый код chat: API по-прежнему получает Bearer, а файлы — отдельную cookie
	if err := service.SetFileCookie(w, r, user.ID, time.Now().Add(sessionLifetime)); err != nil { // новый код chat
		http.Error(w, "Cookie error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"user_id": strconv.FormatUint(uint64(user.ID), 10),
	})
}

// --------- новый код chat: продление только действующей сессии по активности пользователя
func RefreshSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !service.FileRequestOriginAllowed(r) {
		http.Error(w, "Origin not allowed", http.StatusForbidden)
		return
	}
	id := middleware.CurrentUserID(r)
	if id == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := GenerateToken(user.ID, user.Login)
	if err != nil {
		http.Error(w, "Unable to create token", http.StatusInternalServerError)
		return
	}
	if err := service.SetFileCookie(w, r, user.ID, time.Now().Add(sessionLifetime)); err != nil {
		http.Error(w, "Cookie error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// --------- конец нового кода chat
