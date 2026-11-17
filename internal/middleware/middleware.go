package middleware

import (
	"context" // новый код chat: пользователь из проверенного JWT
	"net/http"
	"project/internal/config"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// --------- новый код chat: ID пользователя не принимается из тела запроса
type authUserKey struct{}

func CurrentUserID(r *http.Request) uint { id, _ := r.Context().Value(authUserKey{}).(uint); return id }

// --------- конец нового кода chat
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return config.JWTKey, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// новый код chat: сохраняем пользователя для персональных операций
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if id, valid := claims["user_id"].(float64); valid && id > 0 && id == float64(uint(id)) {
				r = r.WithContext(context.WithValue(r.Context(), authUserKey{}, uint(id)))
			}
		}
		next(w, r)
	}
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Разрешаем Vue dev server
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, PATCH, OPTIONS, DELETE") // новый код chat: разрешить изменение статуса штрафа
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// обработка preflight запроса
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
