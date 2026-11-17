// --------- новый код chat: отдельный токен только для чтения файлов
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/url"
	"os"
	"project/internal/config"
	"time"
)

const FileCookieName = "file_access"

func fileSigningKey() []byte {
	h := hmac.New(sha256.New, config.JWTKey)
	h.Write([]byte("brm-file-access-v1"))
	return h.Sum(nil)
}
func FileRequestOriginAllowed(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host == r.Host
}
func SetFileCookie(w http.ResponseWriter, r *http.Request, userID uint, expires time.Time) error {
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": userID, "aud": "files", "exp": expires.Unix()}).SignedString(fileSigningKey())
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: FileCookieName, Value: token, Path: "/api/files/", HttpOnly: true, Secure: os.Getenv("APP_ENV") == "production" || r.TLS != nil, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
	return nil
}
func ClearFileCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: FileCookieName, Value: "", Path: "/api/files/", HttpOnly: true, Secure: os.Getenv("APP_ENV") == "production" || r.TLS != nil, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
}
func FileCookieUser(r *http.Request) (uint, error) {
	c, err := r.Cookie(FileCookieName)
	if err != nil {
		return 0, err
	}
	token, err := jwt.Parse(c.Value, func(t *jwt.Token) (any, error) { return fileSigningKey(), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithAudience("files"))
	if err != nil || !token.Valid {
		return 0, errors.New("invalid file token")
	}
	claims := token.Claims.(jwt.MapClaims)
	id, ok := claims["user_id"].(float64)
	if !ok || id <= 0 || id != float64(uint(id)) {
		return 0, errors.New("invalid user")
	}
	return uint(id), nil
}

// --------- конец нового кода chat
