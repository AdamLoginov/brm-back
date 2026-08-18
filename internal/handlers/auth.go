package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = config.JWTKey

func GenerateToken(userID uint, login string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"login":   login,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString(jwtKey)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	json.NewDecoder(r.Body).Decode(&input)

	var user models.User
	resultLogin := database.DB.Where("login = ?", input.Login).First(&user)

	if resultLogin.Error != nil {
		http.Error(w, "Логин не найден", http.StatusUnauthorized)
		return
	}

	if user.Password != input.Password {
		http.Error(w, "Неверный пароль", http.StatusUnauthorized)
		return
	}

	fmt.Println(input)
	fmt.Println(user)

	token, _ := GenerateToken(user.ID, user.Login)
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"user_id": strconv.FormatUint(uint64(user.ID), 10),
	})
}
