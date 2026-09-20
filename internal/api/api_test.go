package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/v2raynix/v2raynix/internal/api"
	"github.com/v2raynix/v2raynix/internal/auth"
	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
)

func setupTestRouter(t *testing.T) (http.Handler, string) {
	tempDir, err := os.MkdirTemp("", "v2raynix-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	st, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	hash, _ := auth.HashPassword("admin123")
	_ = st.SetAdminUser(&store.UserAccount{
		Username:     "admin",
		PasswordHash: hash,
	})

	jwtSecret := []byte("test-jwt-secret-key-32-chars-long!")
	sup := core.NewSupervisor(st, 60, true)

	deps := &api.Dependencies{
		Store:      st,
		Supervisor: sup,
		JWTSecret:  jwtSecret,
	}

	router := api.NewRouter(deps)
	return router, tempDir
}

func TestAPI_AuthFlow(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// 1. Login with invalid password -> 401
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	// 2. Login with valid password -> 200 + token
	loginBody, _ = json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &loginResp)
	token, ok := loginResp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected valid token in login response")
	}

	// 3. Access protected /api/auth/me with token
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for protected route, got %d", rec.Code)
	}

	// 4. Access protected without token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAPI_ConfigOperations(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)

	// 1. Post new config
	createBody, _ := json.Marshal(map[string]string{
		"content": "vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@1.2.3.4:443?security=none#API-Test",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/configs", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Get configs
	req = httptest.NewRequest(http.MethodGet, "/api/configs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var configs []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &configs)
	if len(configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(configs))
	}
}

func TestAPI_TunnelEndpoints(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var loginResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &loginResp)
	token := loginResp["token"].(string)

	// 1. Status initially disconnected
	req = httptest.NewRequest(http.MethodGet, "/api/tunnel/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var status map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &status)
	if status["state"] != "disconnected" {
		t.Errorf("expected state disconnected, got %s", status["state"])
	}
}
