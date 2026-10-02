package cli

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/store"
)

func TestTUIBridge_OfflineMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize bridge on a port where no daemon is running
	bridge := NewTUIBridge(tmpDir, 29876)

	// 1. Verify daemon detection is false
	if bridge.IsDaemonRunning() {
		t.Fatalf("expected IsDaemonRunning() == false in offline mode")
	}

	// 2. Verify GetStatus() returns offline defaults
	status, err := bridge.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	if status.ServiceState != "inactive" {
		t.Errorf("expected ServiceState == 'inactive', got %q", status.ServiceState)
	}
	if status.TunnelState != "disconnected" {
		t.Errorf("expected TunnelState == 'disconnected', got %q", status.TunnelState)
	}

	// 3. Verify ConnectTunnel() returns clear error indicating daemon is inactive
	err = bridge.ConnectTunnel("test-config-id")
	if err == nil {
		t.Fatalf("expected ConnectTunnel() to fail when daemon is inactive, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "inactive") {
		t.Errorf("expected error containing 'inactive', got %q", err.Error())
	}

	// 4. Verify initial ListConfigs() is empty
	configs, activeID, err := bridge.ListConfigs()
	if err != nil {
		t.Fatalf("unexpected error listing configs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs, got %d", len(configs))
	}
	if activeID != "" {
		t.Errorf("expected empty activeID, got %q", activeID)
	}

	// 5. Verify AddConfigFromLink() saves directly to store
	vlessLink := "vless://a9b8c7d6-e5f4-3210-fedc-ba9876543210@test.example.com:443?security=reality&sni=example.com#TestNode"
	item, err := bridge.AddConfigFromLink(vlessLink)
	if err != nil {
		t.Fatalf("AddConfigFromLink failed: %v", err)
	}
	if item.Name != "TestNode" {
		t.Errorf("expected name 'TestNode', got %q", item.Name)
	}
	if item.Protocol != "vless" {
		t.Errorf("expected protocol 'vless', got %q", item.Protocol)
	}

	// Verify it shows in ListConfigs()
	configs, _, err = bridge.ListConfigs()
	if err != nil {
		t.Fatalf("ListConfigs failed: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].ID != item.ID {
		t.Errorf("expected config ID %q, got %q", item.ID, configs[0].ID)
	}

	// 6. Verify DeleteConfig() removes config directly from store
	err = bridge.DeleteConfig(item.ID)
	if err != nil {
		t.Fatalf("DeleteConfig failed: %v", err)
	}
	configs, _, err = bridge.ListConfigs()
	if err != nil {
		t.Fatalf("ListConfigs failed after delete: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 configs after delete, got %d", len(configs))
	}

	// 7. Verify ListRules(), AddRule(), DeleteRule()
	rule := store.RoutingRule{
		ID:         "rule-offline-1",
		Target:     "adservice.google.com",
		TargetType: "domain",
		Action:     "block",
		Priority:   50,
		IsEnabled:  true,
	}
	err = bridge.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}
	rules, err := bridge.ListRules()
	if err != nil {
		t.Fatalf("ListRules failed: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != "rule-offline-1" {
		t.Fatalf("expected 1 rule with ID 'rule-offline-1', got %+v", rules)
	}
	err = bridge.DeleteRule("rule-offline-1")
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}
	rules, err = bridge.ListRules()
	if err != nil {
		t.Fatalf("ListRules failed after delete: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}

	// 8. Verify ApplyPresets() adds default routing presets
	err = bridge.ApplyPresets()
	if err != nil {
		t.Fatalf("ApplyPresets failed: %v", err)
	}
	rules, err = bridge.ListRules()
	if err != nil {
		t.Fatalf("ListRules failed after presets: %v", err)
	}
	if len(rules) < 2 {
		t.Errorf("expected at least 2 preset rules, got %d", len(rules))
	}

	// 9. Verify GetSettings() and SaveSettings()
	settings, err := bridge.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	settings.WebPort = 3080
	settings.SafeModeSeconds = 90
	err = bridge.SaveSettings(settings)
	if err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}
	updatedSettings, err := bridge.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after save failed: %v", err)
	}
	if updatedSettings.WebPort != 3080 || updatedSettings.SafeModeSeconds != 90 {
		t.Errorf("settings not persisted correctly: %+v", updatedSettings)
	}

	// 10. Verify ApplyCredentials()
	err = bridge.ApplyCredentials("superadmin", "superpass123")
	if err != nil {
		t.Fatalf("ApplyCredentials failed: %v", err)
	}
}

func TestTUIBridge_LiveDaemonMode(t *testing.T) {
	tmpDir := t.TempDir()

	authenticated := false

	// Start httptest.Server mocking the daemon API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/login":
			var body struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Username != "" && body.Password != "" {
				authenticated = true
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"token": "mock-token-xyz-123",
					"user":  map[string]string{"username": body.Username},
				})
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			return

		case "/api/status", "/api/tunnel/status":
			if r.Header.Get("Authorization") != "Bearer mock-token-xyz-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"state":              "connected",
				"activeConfigId":     "cfg-node-1",
				"activeConfigName":   "EU - Premium VLESS",
				"uptimeSeconds":      360,
				"uploadSpeedBps":     25600,
				"downloadSpeedBps":   102400,
				"totalUploadBytes":   1048576,
				"totalDownloadBytes": 4194304,
				"safeMode": map[string]interface{}{
					"isActive":         true,
					"remainingSeconds": 75,
				},
			})
			return

		case "/api/tunnel/connect":
			if r.Header.Get("Authorization") != "Bearer mock-token-xyz-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"state": "connected",
			})
			return

		case "/api/health/check":
			if r.Header.Get("Authorization") != "Bearer mock-token-xyz-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"healthy":   true,
				"latencyMs": 38,
				"error":     "",
			})
			return

		case "/api/logs", "/api/system/logs":
			if r.Header.Get("Authorization") != "Bearer mock-token-xyz-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{
					"timestamp": "2026-10-02 12:00:00",
					"level":     "INFO",
					"message":   "tunnel connected successfully",
				},
				{
					"timestamp": "2026-10-02 12:00:05",
					"level":     "INFO",
					"message":   "health check passed (38ms)",
				},
			})
			return

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port

	bridge := NewTUIBridge(tmpDir, port)
	bridge.SetBaseURL(server.URL)

	// 1. Verify IsDaemonRunning() detects the running server
	if !bridge.IsDaemonRunning() {
		t.Fatalf("expected IsDaemonRunning() == true for running mock daemon")
	}

	// 2. Verify GetStatus() authenticates and retrieves live status
	status, err := bridge.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}
	if !authenticated {
		t.Errorf("expected bridge to have authenticated with mock daemon")
	}
	if status.ServiceState != "active" {
		t.Errorf("expected ServiceState == 'active', got %q", status.ServiceState)
	}
	if status.TunnelState != "connected" {
		t.Errorf("expected TunnelState == 'connected', got %q", status.TunnelState)
	}
	if status.ConfigName != "EU - Premium VLESS" {
		t.Errorf("expected ConfigName == 'EU - Premium VLESS', got %q", status.ConfigName)
	}
	if status.UploadSpeed != 25600 {
		t.Errorf("expected UploadSpeed == 25600, got %d", status.UploadSpeed)
	}
	if status.DownloadSpeed != 102400 {
		t.Errorf("expected DownloadSpeed == 102400, got %d", status.DownloadSpeed)
	}
	if status.SafeModeRemaining != 75 {
		t.Errorf("expected SafeModeRemaining == 75, got %d", status.SafeModeRemaining)
	}

	// 3. Verify ConnectTunnel() calls API endpoint
	err = bridge.ConnectTunnel("cfg-node-1")
	if err != nil {
		t.Fatalf("ConnectTunnel() failed: %v", err)
	}

	// 4. Verify CheckHealth() calls API endpoint
	healthy, lat, err := bridge.CheckHealth()
	if err != nil {
		t.Fatalf("CheckHealth() failed: %v", err)
	}
	if !healthy {
		t.Errorf("expected healthy == true, got false")
	}
	if lat != 38 {
		t.Errorf("expected latency == 38ms, got %d", lat)
	}

	// 5. Verify GetLogs() calls API endpoint
	logs, err := bridge.GetLogs(10)
	if err != nil {
		t.Fatalf("GetLogs() failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logs))
	}
	if logs[0].Message != "tunnel connected successfully" {
		t.Errorf("unexpected log message: %q", logs[0].Message)
	}
}
