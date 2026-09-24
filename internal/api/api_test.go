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

func TestAPI_BatchConfigImport(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// Login
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

	// Post multi-line batch configs
	batchContent := "vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@1.1.1.1:443?security=none#Node1\n" +
		"vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@2.2.2.2:443?security=none#Node2\n" +
		"vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@3.3.3.3:443?security=none#Node3"

	createBody, _ := json.Marshal(map[string]string{
		"content": batchContent,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/configs", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for batch import, got %d: %s", rec.Code, rec.Body.String())
	}

	var created []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if len(created) != 3 {
		t.Fatalf("expected 3 created configs, got %d", len(created))
	}

	// Verify all 3 in store
	req = httptest.NewRequest(http.MethodGet, "/api/configs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var all []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &all)
	if len(all) != 3 {
		t.Fatalf("expected 3 total configs in store, got %d", len(all))
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

func TestAPI_RequestBodySizeLimit(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// Construct payload larger than 2MB limit (e.g. 2.5 MB)
	largeData := make([]byte, 2500000)
	for i := range largeData {
		largeData[i] = 'a'
	}
	largeJSON, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": string(largeData),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(largeJSON))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 413 or 400 for oversized payload, got HTTP %d", rec.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] == "" {
		t.Errorf("expected error message in response body")
	}
}

func TestAPI_LoginRateLimiting(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	clientAddr := "192.168.1.100:54321"

	// 5 failed login attempts
	for i := 1; i <= 5; i++ {
		loginBody, _ := json.Marshal(map[string]string{
			"username": "admin",
			"password": "wrong-password",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
		req.RemoteAddr = clientAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 Unauthorized, got %d", i, rec.Code)
		}
	}

	// 6th attempt should be blocked by rate limiter (HTTP 429)
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123", // Even with correct password, blocked
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.RemoteAddr = clientAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected HTTP 429 Too Many Requests on 6th attempt, got HTTP %d: %s", rec.Code, rec.Body.String())
	}

	// Another client IP should still be allowed
	otherAddr := "192.168.1.101:54321"
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.RemoteAddr = otherAddr
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected other client IP to succeed with 200 OK, got HTTP %d", rec.Code)
	}

	// Verify counter reset on successful login:
	// A client fails 3 times (< 5), logs in successfully, resetting counter to 0.
	resetClientAddr := "192.168.1.102:54321"
	for i := 0; i < 3; i++ {
		badReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(`{"username":"admin","password":"bad"}`)))
		badReq.RemoteAddr = resetClientAddr
		badRec := httptest.NewRecorder()
		router.ServeHTTP(badRec, badReq)
		if badRec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", badRec.Code)
		}
	}

	// Successful login resets counter
	goodReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	goodReq.RemoteAddr = resetClientAddr
	goodRec := httptest.NewRecorder()
	router.ServeHTTP(goodRec, goodReq)
	if goodRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", goodRec.Code)
	}

	// Should now be able to fail 4 more times without being blocked (since counter was reset)
	for i := 0; i < 4; i++ {
		badReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(`{"username":"admin","password":"bad"}`)))
		badReq.RemoteAddr = resetClientAddr
		badRec := httptest.NewRecorder()
		router.ServeHTTP(badRec, badReq)
		if badRec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on attempt %d after reset, got %d", i+1, badRec.Code)
		}
	}
}

func TestAPI_ConfigTestEndpoints(t *testing.T) {
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
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", rec.Code)
	}

	var authResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &authResp)
	token := authResp["token"]

	// Create a test config
	createBody, _ := json.Marshal(map[string]string{
		"content": "vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@127.0.0.1:59990?type=tcp#TestNode",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/configs", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create config: %d %s", rec.Code, rec.Body.String())
	}

	var created store.ConfigItem
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	configID := created.ID

	// 1. Single config test -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/configs/"+configID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for single test, got %d: %s", rec.Code, rec.Body.String())
	}
	var singleRes map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &singleRes)
	if singleRes["id"] != configID {
		t.Errorf("expected id %s, got %v", configID, singleRes["id"])
	}

	// 2. Single config test with non-existent ID -> 404 Not Found
	req = httptest.NewRequest(http.MethodPost, "/api/configs/non-existent-id/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent config, got %d", rec.Code)
	}

	// 3. Batch test-all -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/configs/test-all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for test-all, got %d", rec.Code)
	}
	var batchRes map[string]int
	_ = json.Unmarshal(rec.Body.Bytes(), &batchRes)
	if _, ok := batchRes[configID]; !ok {
		t.Errorf("expected configID %s in batch results", configID)
	}

	// 4. Batch ping-all -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/configs/ping-all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ping-all, got %d", rec.Code)
	}
	var pingRes map[string]int
	_ = json.Unmarshal(rec.Body.Bytes(), &pingRes)
	if _, ok := pingRes[configID]; !ok {
		t.Errorf("expected configID %s in ping-all results", configID)
	}
}

func TestAPI_SystemUpdateEndpoints(t *testing.T) {
	router, tempDir := setupTestRouter(t)
	defer os.RemoveAll(tempDir)

	// Unauthorized test
	req := httptest.NewRequest(http.MethodGet, "/api/system/updates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without token, got %d", rec.Code)
	}

	// Login to get token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "admin123",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", rec.Code)
	}

	var authResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &authResp)
	token := authResp["token"]

	// GET /api/system/updates -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/system/updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/system/updates, got %d: %s", rec.Code, rec.Body.String())
	}

	// POST /api/system/check-updates -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/system/check-updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST /api/system/check-updates, got %d: %s", rec.Code, rec.Body.String())
	}

	// POST /api/system/update-core with invalid core -> 400 Bad Request
	badCoreBody, _ := json.Marshal(map[string]string{"core": "invalid-engine"})
	req = httptest.NewRequest(http.MethodPost, "/api/system/update-core", bytes.NewReader(badCoreBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid core, got %d: %s", rec.Code, rec.Body.String())
	}
}



