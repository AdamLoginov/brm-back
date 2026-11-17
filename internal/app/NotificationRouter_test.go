// --------- новый код chat: HTTP авторизация и личные уведомления
package app

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
	"testing"
	"time"
)

func TestNotificationRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notify.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	oldDB, oldKey := database.DB, config.JWTKey
	database.DB = db
	config.JWTKey = []byte("notification-tests")
	t.Cleanup(func() { database.DB = oldDB; config.JWTKey = oldKey; c, _ := db.DB(); c.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.Task{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	users := []models.User{{Login: "author"}, {Login: "receiver"}}
	db.Create(&users)
	router := NewRouter()
	call := func(method, url, body string, user uint) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		if user > 0 {
			token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Hour).Unix()}).SignedString(config.JWTKey)
			if e != nil {
				t.Fatal(e)
			}
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, req := range []struct{ method, url string }{{"GET", "/api/notifications"}, {"POST", "/api/notifications/read-all"}, {"POST", "/api/notifications/1/read"}, {"POST", "/api/task/create"}} {
		if w := call(req.method, req.url, "{}", 0); w.Code != 401 {
			t.Fatal("anonymous", req, w.Code)
		}
	}
	body := fmt.Sprintf(`{"message":"Задача","to_user_id":%d,"from_user_id":%d}`, users[1].ID, users[1].ID)
	w := call("POST", "/api/task/create", body, users[0].ID)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var task models.Task
	json.Unmarshal(w.Body.Bytes(), &task)
	if task.FromUserID != users[0].ID {
		t.Fatal("spoofed author")
	}
	w = call("GET", fmt.Sprintf("/api/notifications?user_id=%d", users[1].ID), "", users[0].ID)
	var list service.NotificationList
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 {
		t.Fatal("other user's list exposed")
	}
	w = call("GET", "/api/notifications", "", users[1].ID)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Unread != 1 {
		t.Fatal("missing assignment", w.Body.String())
	}
	id := list.Items[0].ID
	if w = call("POST", fmt.Sprintf("/api/notifications/%d/read", id), "", users[0].ID); w.Code != 404 {
		t.Fatal("other user's read", w.Code)
	}
	if w = call("GET", fmt.Sprintf("/api/task/success/%d", task.ID), "", users[1].ID); w.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET mutation", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w = call("POST", fmt.Sprintf("/api/task/success/%d", task.ID), "", users[1].ID); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	w = call("GET", "/api/notifications", "", users[0].ID)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 || list.Items[0].Kind != "completed" {
		t.Fatal("completion duplicate", w.Body.String())
	}
	if w = call("POST", "/api/notifications/read-all", "", users[0].ID); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = call("DELETE", fmt.Sprintf("/api/task/delete/%d", task.ID), "", users[0].ID); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call("GET", "/api/notifications", "", users[1].ID)
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 0 || list.Unread != 0 {
		t.Fatal("delete", w.Body.String())
	}
}

// --------- конец нового кода chat
