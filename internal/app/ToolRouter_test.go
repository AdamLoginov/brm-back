// --------- новый код chat: общий список МТР защищён авторизацией, договорный список изолирован
package app

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/config"
	"project/internal/database"
	"project/internal/models"
	"testing"
	"time"
)

func TestToolListRoutes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tools.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousKey := database.DB, config.JWTKey
	database.DB, config.JWTKey = db, []byte("tools-test")
	t.Cleanup(func() { database.DB, config.JWTKey = previousDB, previousKey; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.Tool{}); err != nil {
		t.Fatal(err)
	}
	tools := []models.Tool{{AgreementID: 1, Name: "Дрель"}, {AgreementID: 2, Name: "Перфоратор"}, {AgreementID: 1, Name: "Удалённый"}}
	if err := db.Create(&tools).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&tools[2]).Error; err != nil {
		t.Fatal(err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}).SignedString(config.JWTKey)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter()
	call := func(path string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := call("/api/tools", false); w.Code != 401 {
		t.Fatalf("anonymous status: %d", w.Code)
	}
	for _, tc := range []struct {
		path      string
		count     int
		agreement uint
	}{
		{"/api/tools", 2, 0},
		{fmt.Sprintf("/api/agreements/%d/tools/create", 1), 1, 1},
		{fmt.Sprintf("/api/agreements/%d/tools/create", 2), 1, 2},
	} {
		w := call(tc.path, true)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		var rows []models.Tool
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != tc.count {
			t.Fatalf("%s: count %d", tc.path, len(rows))
		}
		for _, row := range rows {
			if tc.agreement != 0 && row.AgreementID != tc.agreement {
				t.Fatal("foreign agreement")
			}
		}
	}
}

// --------- конец нового кода chat
