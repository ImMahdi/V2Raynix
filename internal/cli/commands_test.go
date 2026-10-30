package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

type mockCLIBridge struct {
	isDaemonRunning        bool
	status                 LiveStatusInfo
	statusErr              error
	configs                []store.ConfigItem
	activeConfigID         string
	listConfigsErr         error
	connectedConfigID      string
	connectTunnelErr       error
	disconnectTunnelCalled bool
	disconnectTunnelErr    error
	confirmSafeModeCalled  bool
	confirmSafeModeErr     error
	rollbackSafeModeCalled bool
	rollbackSafeModeErr    error
	checkHealthCalled      bool
	healthHealthy          bool
	healthLatency          int64
	checkHealthErr         error
}

func (m *mockCLIBridge) IsDaemonRunning() bool               { return m.isDaemonRunning }
func (m *mockCLIBridge) SetBaseURL(url string)              {}
func (m *mockCLIBridge) SetAuth(username, password string)  {}
func (m *mockCLIBridge) GetStatus() (LiveStatusInfo, error) {
	return m.status, m.statusErr
}
func (m *mockCLIBridge) ConnectTunnel(configID string) error {
	m.connectedConfigID = configID
	return m.connectTunnelErr
}
func (m *mockCLIBridge) DisconnectTunnel() error {
	m.disconnectTunnelCalled = true
	return m.disconnectTunnelErr
}
func (m *mockCLIBridge) ConfirmSafeMode() error {
	m.confirmSafeModeCalled = true
	return m.confirmSafeModeErr
}
func (m *mockCLIBridge) RollbackSafeMode() error {
	m.rollbackSafeModeCalled = true
	return m.rollbackSafeModeErr
}
func (m *mockCLIBridge) CheckHealth() (bool, int64, error) {
	m.checkHealthCalled = true
	return m.healthHealthy, m.healthLatency, m.checkHealthErr
}
func (m *mockCLIBridge) ListConfigs() ([]store.ConfigItem, string, error) {
	return m.configs, m.activeConfigID, m.listConfigsErr
}
func (m *mockCLIBridge) AddConfigFromLink(link string) (*store.ConfigItem, error) {
	return nil, nil
}
func (m *mockCLIBridge) DeleteConfig(id string) error    { return nil }
func (m *mockCLIBridge) SetActiveConfig(id string) error { return nil }
func (m *mockCLIBridge) TestConfig(id string) (int, error) {
	return 0, nil
}
func (m *mockCLIBridge) TestAllConfigs() error             { return nil }
func (m *mockCLIBridge) ListRules() ([]store.RoutingRule, error) {
	return nil, nil
}
func (m *mockCLIBridge) AddRule(rule store.RoutingRule) error { return nil }
func (m *mockCLIBridge) DeleteRule(id string) error           { return nil }
func (m *mockCLIBridge) ApplyPresets() error                  { return nil }
func (m *mockCLIBridge) GetLogs(limit int) ([]core.LogEntry, error) {
	return nil, nil
}
func (m *mockCLIBridge) CheckCoreUpdates() (*updater.UpdateStatus, error) {
	return nil, nil
}
func (m *mockCLIBridge) UpdateCore(engine string) error { return nil }
func (m *mockCLIBridge) GetSettings() (*store.SystemSettings, error) {
	return nil, nil
}
func (m *mockCLIBridge) SaveSettings(settings *store.SystemSettings) error {
	return nil
}
func (m *mockCLIBridge) ApplyCredentials(username, password string) error {
	return nil
}

// captureCLIOutput intercepts os.Stdout during function execution.
func captureCLIOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		outChan <- buf.String()
	}()

	fnErr := fn()

	_ = w.Close()
	os.Stdout = origStdout
	out := <-outChan

	return out, fnErr
}

func TestHandleCLICommand_Help(t *testing.T) {
	bridge := &mockCLIBridge{}

	subcmds := [][]string{
		{"help"},
		{"--help"},
		{"-h"},
	}

	for _, args := range subcmds {
		out, err := captureCLIOutput(t, func() error {
			return HandleCLICommand(args, bridge)
		})
		if err != nil {
			t.Fatalf("expected nil error for help command %v, got %v", args, err)
		}
		if !strings.Contains(out, "Usage:") && !strings.Contains(out, "Commands:") {
			t.Errorf("expected usage or commands in help output for %v, got:\n%s", args, out)
		}
		for _, requiredKeyword := range []string{"connect", "disconnect", "status", "health", "list"} {
			if !strings.Contains(out, requiredKeyword) {
				t.Errorf("expected keyword %q in help output for %v, got:\n%s", requiredKeyword, args, out)
			}
		}
	}
}

func TestHandleCLICommand_List(t *testing.T) {
	bridge := &mockCLIBridge{
		configs: []store.ConfigItem{
			{ID: "cfg-1", Name: "US-East-1", Protocol: "vless", Server: "10.0.0.1", Port: 443},
			{ID: "cfg-2", Name: "DE-Frankfurt", Protocol: "vmess", Server: "10.0.0.2", Port: 8443},
		},
		activeConfigID: "cfg-1",
	}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"list"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	for _, expected := range []string{"US-East-1", "DE-Frankfurt", "vless", "vmess", "cfg-1", "cfg-2"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in list output, got:\n%s", expected, out)
		}
	}

	// Empty list case
	emptyBridge := &mockCLIBridge{
		configs: []store.ConfigItem{},
	}
	emptyOut, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"list"}, emptyBridge)
	})
	if err != nil {
		t.Fatalf("expected nil error for empty configs, got %v", err)
	}
	if !strings.Contains(strings.ToLower(emptyOut), "no configuration") {
		t.Errorf("expected empty message in list output, got:\n%s", emptyOut)
	}
}

func TestHandleCLICommand_Status(t *testing.T) {
	bridge := &mockCLIBridge{
		isDaemonRunning: true,
		status: LiveStatusInfo{
			ServiceState:  "active",
			TunnelState:   "connected",
			ConfigName:    "US-Fast-Node",
			Protocol:      "vless",
			UploadSpeed:   1024 * 50,
			DownloadSpeed: 1024 * 200,
			Uptime:        3600,
			HealthState:   "healthy",
			HealthLatency: 42,
		},
	}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"status"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	for _, expected := range []string{"active", "connected", "US-Fast-Node", "vless"} {
		if !strings.Contains(out, expected) {
			t.Errorf("expected %q in status output, got:\n%s", expected, out)
		}
	}
}

func TestHandleCLICommand_StatusJSON(t *testing.T) {
	bridge := &mockCLIBridge{
		isDaemonRunning: true,
		status: LiveStatusInfo{
			ServiceState:  "active",
			TunnelState:   "connected",
			ConfigName:    "US-Fast-Node",
			Protocol:      "vless",
			UploadSpeed:   51200,
			DownloadSpeed: 204800,
			Uptime:        3600,
			HealthState:   "healthy",
			HealthLatency: 42,
		},
	}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"status", "--json"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(out), &jsonResult); err != nil {
		t.Fatalf("expected valid json output, got unmarshal error %v on:\n%s", err, out)
	}

	if jsonResult["serviceState"] != "active" && jsonResult["ServiceState"] != "active" {
		t.Errorf("expected serviceState == active, got %v", jsonResult)
	}
	if jsonResult["tunnelState"] != "connected" && jsonResult["TunnelState"] != "connected" {
		t.Errorf("expected tunnelState == connected, got %v", jsonResult)
	}
}

func TestHandleCLICommand_Connect(t *testing.T) {
	bridge := &mockCLIBridge{
		configs: []store.ConfigItem{
			{ID: "cfg-us-12345", Name: "US Premium", Protocol: "vless"},
			{ID: "cfg-de-67890", Name: "Frankfurt Edge", Protocol: "vmess"},
		},
	}

	// 1. Connect by exact ID
	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect", "cfg-us-12345"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected connect by exact ID to succeed, got %v", err)
	}
	if bridge.connectedConfigID != "cfg-us-12345" {
		t.Errorf("expected connectedConfigID cfg-us-12345, got %s", bridge.connectedConfigID)
	}
	if !strings.Contains(out, "Connected") && !strings.Contains(out, "US Premium") {
		t.Errorf("expected confirmation message in output, got:\n%s", out)
	}

	// 2. Connect by name substring (case-insensitive)
	bridge.connectedConfigID = ""
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect", "frankfurt"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected connect by substring to succeed, got %v", err)
	}
	if bridge.connectedConfigID != "cfg-de-67890" {
		t.Errorf("expected connectedConfigID cfg-de-67890, got %s", bridge.connectedConfigID)
	}

	// 3. Connect missing target node argument
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect"}, bridge)
	})
	if err == nil {
		t.Fatalf("expected error when target node is missing, got nil")
	}

	// 4. Connect non-existent target node
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect", "non-existent-node"}, bridge)
	})
	if err == nil {
		t.Fatalf("expected error when node not found, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got %v", err)
	}
}

func TestHandleCLICommand_ConnectForce(t *testing.T) {
	bridge := &mockCLIBridge{
		configs: []store.ConfigItem{
			{ID: "cfg-test-force", Name: "Force Node", Protocol: "vless"},
		},
	}

	// Test with --force
	_, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect", "Force Node", "--force"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on connect --force, got %v", err)
	}
	if bridge.connectedConfigID != "cfg-test-force" {
		t.Errorf("expected connectedConfigID cfg-test-force, got %s", bridge.connectedConfigID)
	}
	if !bridge.confirmSafeModeCalled {
		t.Errorf("expected confirmSafeModeCalled to be true when --force is passed")
	}

	// Test with -f
	bridge.confirmSafeModeCalled = false
	bridge.connectedConfigID = ""
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"connect", "-f", "cfg-test-force"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on connect -f, got %v", err)
	}
	if bridge.connectedConfigID != "cfg-test-force" {
		t.Errorf("expected connectedConfigID cfg-test-force, got %s", bridge.connectedConfigID)
	}
	if !bridge.confirmSafeModeCalled {
		t.Errorf("expected confirmSafeModeCalled to be true when -f is passed")
	}
}

func TestHandleCLICommand_Disconnect(t *testing.T) {
	bridge := &mockCLIBridge{}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"disconnect"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on disconnect, got %v", err)
	}
	if !bridge.disconnectTunnelCalled {
		t.Errorf("expected disconnectTunnelCalled to be true")
	}
	if !strings.Contains(strings.ToLower(out), "disconnected") {
		t.Errorf("expected confirmation message in output, got:\n%s", out)
	}
}

func TestHandleCLICommand_Health(t *testing.T) {
	bridge := &mockCLIBridge{
		healthHealthy: true,
		healthLatency: 38,
	}

	// 1. "health" subcommand with healthy probe
	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"health"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on healthy check, got %v", err)
	}
	if !bridge.checkHealthCalled {
		t.Errorf("expected checkHealthCalled to be true")
	}
	if !strings.Contains(out, "38") || !strings.Contains(strings.ToLower(out), "healthy") {
		t.Errorf("expected latency and health status in output, got:\n%s", out)
	}

	// 2. "check" subcommand alias
	bridge.checkHealthCalled = false
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"check"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on 'check' alias, got %v", err)
	}
	if !bridge.checkHealthCalled {
		t.Errorf("expected checkHealthCalled to be true for 'check'")
	}

	// 3. Unhealthy probe should return an error (causing non-zero CLI exit)
	bridge.healthHealthy = false
	bridge.healthLatency = 0
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"health"}, bridge)
	})
	if err == nil {
		t.Fatalf("expected error on unhealthy probe, got nil")
	}
}

func TestHandleCLICommand_Confirm(t *testing.T) {
	bridge := &mockCLIBridge{}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"confirm"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on confirm, got %v", err)
	}
	if !bridge.confirmSafeModeCalled {
		t.Errorf("expected confirmSafeModeCalled to be true")
	}
	if !strings.Contains(strings.ToLower(out), "safe mode") {
		t.Errorf("expected safe mode in output, got:\n%s", out)
	}
}

func TestHandleCLICommand_Rollback(t *testing.T) {
	bridge := &mockCLIBridge{}

	out, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"rollback"}, bridge)
	})
	if err != nil {
		t.Fatalf("expected nil error on rollback, got %v", err)
	}
	if !bridge.rollbackSafeModeCalled {
		t.Errorf("expected rollbackSafeModeCalled to be true")
	}
	if !strings.Contains(strings.ToLower(out), "safe mode") && !strings.Contains(strings.ToLower(out), "rollback") {
		t.Errorf("expected rollback confirmation in output, got:\n%s", out)
	}
}

func TestHandleCLICommand_Unknown(t *testing.T) {
	bridge := &mockCLIBridge{}

	_, err := captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{"invalid-subcommand"}, bridge)
	})
	if err == nil {
		t.Fatalf("expected error on unknown subcommand, got nil")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected 'unknown' in error message, got %v", err)
	}

	// Empty args
	_, err = captureCLIOutput(t, func() error {
		return HandleCLICommand([]string{}, bridge)
	})
	if err == nil {
		t.Fatalf("expected error on empty args, got nil")
	}
}
