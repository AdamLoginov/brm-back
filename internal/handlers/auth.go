package handlers

import (
	"encoding/json"
	"net/http"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func GenerateToken(userID uint, login string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"login":   login,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString(config.JWTKey)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
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
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"user_id": strconv.FormatUint(uint64(user.ID), 10),
	})
}
