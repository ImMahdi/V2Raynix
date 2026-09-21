package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
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

type errorStore struct {
	store.Store
	getAdminErr error
	setAdminCalled bool
}

func (e *errorStore) GetAdminUser() (*store.UserAccount, error) {
	if e.getAdminErr != nil {
		return nil, e.getAdminErr
	}
	return e.Store.GetAdminUser()
}

func (e *errorStore) SetAdminUser(user *store.UserAccount) error {
	e.setAdminCalled = true
	return e.Store.SetAdminUser(user)
}

func TestLoginStoreErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-api-err-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	mockErrStore := &errorStore{
		Store:       st,
		getAdminErr: errors.New("simulated I/O disk corruption"),
	}

	sup := core.NewSupervisor(mockErrStore, 60, true)
	deps := &api.Dependencies{
		Store:      mockErrStore,
		Supervisor: sup,
		JWTSecret:  []byte("test-jwt-secret-key-32-chars-long!"),
	}
	router := api.NewRouter(deps)

	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "anypassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// It must NOT reset admin or return 401 based on fake "admin:admin"
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 when store fails, got HTTP %d", rec.Code)
	}

	if mockErrStore.setAdminCalled {
		t.Fatalf("SECURITY VIOLATION: SetAdminUser was called on store error (API-01 regression)")
	}
}
