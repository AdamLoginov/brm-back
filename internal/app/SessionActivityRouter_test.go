// --------- новый код chat: три дня, продление и запрет восстановления просроченного входа
package app

import (
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/handlers"
	"project/internal/models"
	"project/internal/service"
	"testing"
	"time"
)

func TestSessionActivityRefresh(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "session.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldKey := database.DB, config.JWTKey
	database.DB, config.JWTKey = db, []byte("session-activity-test")
	t.Cleanup(func() { database.DB, config.JWTKey = oldDB, oldKey; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Login: "session-test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token, err := handlers.GenerateToken(user.ID, user.Login)
	if err != nil {
		t.Fatal(err)
	}
	checkLifetime := func(value string) {
		t.Helper()
		parsed, err := jwt.Parse(value, func(*jwt.Token) (any, error) { return config.JWTKey, nil }, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			t.Fatal(err)
		}
		exp, err := parsed.Claims.GetExpirationTime()
		if err != nil || exp == nil {
			t.Fatal("missing expiration")
		}
		remaining := time.Until(exp.Time)
		if remaining < 72*time.Hour-5*time.Second || remaining > 72*time.Hour {
			t.Fatalf("lifetime %v", remaining)
		}
	}
	checkLifetime(token)
	router := NewRouter()
	call := func(token, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/session/refresh", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	sign := func(id uint, exp time.Time) string {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": id, "exp": exp.Unix()}).SignedString(config.JWTKey)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	res := call(sign(user.ID, time.Now().Add(time.Hour)), "")
	if res.Code != 200 {
		t.Fatalf("refresh: %d %s", res.Code, res.Body.String())
	}
	var result map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	checkLifetime(result["token"])
	cookies := res.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != service.FileCookieName || !cookies[0].HttpOnly || cookies[0].MaxAge < 259195 {
		t.Fatal("file cookie not renewed")
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("refresh cached")
	}
	for _, invalid := range []string{"", "invalid", sign(user.ID, time.Now().Add(-time.Second)), sign(99999, time.Now().Add(time.Hour))} {
		if res := call(invalid, ""); res.Code != 401 {
			t.Fatalf("invalid session accepted: %d", res.Code)
		}
	}
	if res := call(token, "https://foreign.example"); res.Code != 403 {
		t.Fatalf("foreign origin accepted: %d", res.Code)
	}
}

// --------- конец нового кода chat
