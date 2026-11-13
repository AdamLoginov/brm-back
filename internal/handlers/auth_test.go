package handlers

import (
	"net/http"
	"net/http/httptest"
	"project/internal/config"
	"project/internal/middleware"
	"testing"
)

func TestTokenUsesLoadedKey(t *testing.T) {
	old := config.JWTKey
	t.Cleanup(func() { config.JWTKey = old })
	config.JWTKey = []byte("test-key-loaded-after-package-initialization")
	token, err := GenerateToken(1, "test")
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid token: %d", response.Code)
	}
	config.JWTKey = []byte("different-key")
	response = httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: %d", response.Code)
	}
}
