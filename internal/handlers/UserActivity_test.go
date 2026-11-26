// --------- новый код chat: посещения фиксируются только при входе и реальной активности
package handlers

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"strings"
	"testing"
	"time"
)

func TestUserLastActivity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "activity.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	password, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Login: "active-user", Password: string(password)}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	load := func() models.User {
		t.Helper()
		var row models.User
		if err := db.First(&row, user.ID).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	call := func(handler http.HandlerFunc, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://example.com/", strings.NewReader(body))
		req.SetPathValue("id", fmt.Sprint(user.ID))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		handler(res, req)
		return res
	}
	if load().LastActiveAt != nil {
		t.Fatal("new user has activity")
	}
	if res := call(LoginHandler, `{"login":"active-user","password":"wrong"}`, ""); res.Code != 401 {
		t.Fatal(res.Code)
	}
	if load().LastActiveAt != nil {
		t.Fatal("failed login changed activity")
	}
	start := time.Now().UTC()
	if res := call(LoginHandler, `{"login":"active-user","password":"test-password"}`, ""); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	row := load()
	if row.LastActiveAt == nil || row.LastActiveAt.Before(start) {
		t.Fatal("login not recorded")
	}
	earlier := time.Now().UTC().Add(-time.Hour)
	if err = db.Model(&user).UpdateColumn("last_active_at", earlier).Error; err != nil {
		t.Fatal(err)
	}
	token, err := GenerateToken(user.ID, user.Login)
	if err != nil {
		t.Fatal(err)
	}
	// Чтение списка и любой фоновый запрос через middleware не продлевают посещение.
	if res := call(middleware.AuthMiddleware(GetAllUserHandler), "", token); res.Code != 200 {
		t.Fatal(res.Code)
	}
	if !load().LastActiveAt.Equal(earlier) {
		t.Fatal("background request changed activity")
	}
	start = time.Now().UTC()
	if res := call(middleware.AuthMiddleware(RefreshSessionHandler), "", token); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	row = load()
	if row.LastActiveAt == nil || row.LastActiveAt.Before(start) {
		t.Fatal("refresh not recorded")
	}
	saved := *row.LastActiveAt
	if res := call(middleware.AuthMiddleware(RefreshSessionHandler), "", "invalid"); res.Code != 401 {
		t.Fatal(res.Code)
	}
	if !load().LastActiveAt.Equal(saved) {
		t.Fatal("invalid session changed activity")
	}
	if res := call(UpdateUserHandler, `{"login":"active-user","last_active_at":"2000-01-01T00:00:00Z"}`, ""); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	if !load().LastActiveAt.Equal(saved) {
		t.Fatal("editable activity")
	}
	if res := call(CreateUserHandler, `{"login":"new-user","password":"secret","last_active_at":"2000-01-01T00:00:00Z"}`, ""); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	var created models.User
	if err = db.Where("login = ?", "new-user").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.LastActiveAt != nil {
		t.Fatal("creation spoofed activity")
	}
}

// --------- конец нового кода chat
