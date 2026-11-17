// --------- новый код chat: файловая cookie, изоляция API, диапазоны и выход
package app

import (
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
	"testing"
	"time"
)

func TestFileAccessRoutes(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "files.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldKey := database.DB, config.JWTKey
	database.DB = db
	config.JWTKey = []byte("file-access-test-key")
	t.Cleanup(func() { database.DB = oldDB; config.JWTKey = oldKey; conn, _ := db.DB(); conn.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.EmployeeDocuments{}, &models.Attachment{}); err != nil {
		t.Fatal(err)
	}
	password, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	user := models.User{Login: "reader", Password: string(password)}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "document.pdf")
	if err = os.WriteFile(path, []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	document := models.EmployeeDocuments{FileName: "Документ.pdf", FilePath: path, MimeType: "application/pdf"}
	if err = db.Create(&document).Error; err != nil {
		t.Fatal(err)
	}
	attachment := models.Attachment{FileName: "Вложение.pdf", FilePath: path, MimeType: "application/pdf"}
	if err = db.Create(&attachment).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter()
	call := func(method, url, body string, cookie *http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://example.test"+url, strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	docURL := fmt.Sprintf("/api/files/documents/%d", document.ID)
	if r := call("GET", docURL, "", nil, nil); r.Code != 401 {
		t.Fatal("anonymous", r.Code)
	}
	login := `{"login":"reader","password":"test-password"}`
	if r := call("POST", "/api/login", login, nil, map[string]string{"Origin": "https://evil.test"}); r.Code != 403 {
		t.Fatal("cross-site login", r.Code)
	}
	r := call("POST", "/api/login", login, nil, map[string]string{"Origin": "https://example.test"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var response struct {
		Token string `json:"token"`
	}
	json.Unmarshal(r.Body.Bytes(), &response)
	if response.Token == "" {
		t.Fatal("Bearer removed")
	}
	cookies := r.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	cookie := cookies[0]
	if cookie.Path != "/api/files/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("cookie flags", cookie)
	}
	if r = call("GET", docURL, "", cookie, map[string]string{"Range": "bytes=2-5"}); r.Code != 206 || r.Body.String() != "2345" {
		t.Fatal("range", r.Code, r.Body.String())
	}
	if r.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("cache")
	}
	if r = call("HEAD", docURL, "", cookie, nil); r.Code != 200 || r.Body.Len() != 0 {
		t.Fatal("HEAD", r.Code)
	}
	if r = call("GET", fmt.Sprintf("/api/files/attachments/%d", attachment.ID), "", cookie, nil); r.Code != 200 || !strings.HasPrefix(r.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("download", r.Code)
	}
	// Cookie не открывает API даже если клиент вручную передал её вне Path.
	if r = call("GET", "/api/users", "", cookie, nil); r.Code != 401 {
		t.Fatal("cookie grants API")
	}
	if r = call("GET", "/api/users", "", nil, map[string]string{"Authorization": "Bearer " + cookie.Value}); r.Code != 401 {
		t.Fatal("file JWT grants API")
	}
	bearer := map[string]string{"Authorization": "Bearer " + response.Token, "Origin": "https://example.test"}
	r = call("POST", "/api/files/session", "", nil, bearer)
	if r.Code != 204 || len(r.Result().Cookies()) != 1 {
		t.Fatal("existing login", r.Code)
	}
	if r = call("POST", "/api/files/session", "", cookie, nil); r.Code != 401 {
		t.Fatal("cookie can renew itself")
	}
	if r = call("POST", "/api/files/logout", "", cookie, map[string]string{"Origin": "https://evil.test"}); r.Code != 403 {
		t.Fatal("cross-site logout")
	}
	if r = call("POST", "/api/files/logout", "", cookie, map[string]string{"Origin": "https://example.test"}); r.Code != 204 || r.Result().Cookies()[0].MaxAge != -1 || r.Result().Cookies()[0].Path != "/api/files/" {
		t.Fatal("logout")
	}
	// Поддельные и истёкшие cookie не принимаются.
	invalid := *cookie
	invalid.Value += "bad"
	if r = call("GET", docURL, "", &invalid, nil); r.Code != 401 {
		t.Fatal("tampered")
	}
	expired := httptest.NewRecorder()
	service.SetFileCookie(expired, httptest.NewRequest("GET", "https://example.test", nil), user.ID, time.Now().Add(-time.Hour))
	if r = call("GET", docURL, "", expired.Result().Cookies()[0], nil); r.Code != 401 {
		t.Fatal("expired")
	}
	// Активное содержимое открывается только как скачивание.
	db.Model(&document).Updates(map[string]any{"mime_type": "text/html", "file_name": "test.html"})
	if r = call("GET", docURL, "", cookie, nil); !strings.HasPrefix(r.Header().Get("Content-Disposition"), "attachment;") || r.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("active content")
	}
	db.Delete(&user)
	if r = call("GET", docURL, "", cookie, nil); r.Code != 401 {
		t.Fatal("deleted user")
	}
}

// --------- конец нового кода chat
