package cli

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

type mockTunnelLogsBridge struct {
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
	logs                   []core.LogEntry
	getLogsLimit           int
	getLogsErr             error
}

func (m *mockTunnelLogsBridge) IsDaemonRunning() bool               { return true }
func (m *mockTunnelLogsBridge) SetBaseURL(url string)              {}
func (m *mockTunnelLogsBridge) SetAuth(username, password string)  {}
func (m *mockTunnelLogsBridge) GetStatus() (LiveStatusInfo, error) {
	return m.status, m.statusErr
}
func (m *mockTunnelLogsBridge) ConnectTunnel(configID string) error {
	m.connectedConfigID = configID
	return m.connectTunnelErr
}
func (m *mockTunnelLogsBridge) DisconnectTunnel() error {
	m.disconnectTunnelCalled = true
	return m.disconnectTunnelErr
}
func (m *mockTunnelLogsBridge) ConfirmSafeMode() error {
	m.confirmSafeModeCalled = true
	return m.confirmSafeModeErr
}
func (m *mockTunnelLogsBridge) RollbackSafeMode() error {
	m.rollbackSafeModeCalled = true
	return m.rollbackSafeModeErr
}
func (m *mockTunnelLogsBridge) CheckHealth() (bool, int64, error) {
	m.checkHealthCalled = true
	return m.healthHealthy, m.healthLatency, m.checkHealthErr
}
func (m *mockTunnelLogsBridge) ListConfigs() ([]store.ConfigItem, string, error) {
	return m.configs, m.activeConfigID, m.listConfigsErr
}
func (m *mockTunnelLogsBridge) AddConfigFromLink(link string) (*store.ConfigItem, error) {
	return nil, nil
}
func (m *mockTunnelLogsBridge) DeleteConfig(id string) error    { return nil }
func (m *mockTunnelLogsBridge) SetActiveConfig(id string) error {
	m.activeConfigID = id
	return nil
}
func (m *mockTunnelLogsBridge) TestConfig(id string) (int, error) { return 0, nil }
func (m *mockTunnelLogsBridge) TestAllConfigs() error             { return nil }
func (m *mockTunnelLogsBridge) ListRules() ([]store.RoutingRule, error) {
	return nil, nil
}
func (m *mockTunnelLogsBridge) AddRule(rule store.RoutingRule) error    { return nil }
func (m *mockTunnelLogsBridge) DeleteRule(id string) error              { return nil }
func (m *mockTunnelLogsBridge) ApplyPresets() error                     { return nil }
func (m *mockTunnelLogsBridge) GetLogs(limit int) ([]core.LogEntry, error) {
	m.getLogsLimit = limit
	return m.logs, m.getLogsErr
}
func (m *mockTunnelLogsBridge) CheckCoreUpdates() (*updater.UpdateStatus, error) {
	return nil, nil
}
func (m *mockTunnelLogsBridge) UpdateCore(engine string) error { return nil }
func (m *mockTunnelLogsBridge) GetSettings() (*store.SystemSettings, error) {
	return nil, nil
}
func (m *mockTunnelLogsBridge) SaveSettings(settings *store.SystemSettings) error {
	return nil
}
func (m *mockTunnelLogsBridge) ApplyCredentials(username, password string) error {
	return nil
}

func TestHandleTunnelMenu_BackOption(t *testing.T) {
	mock := &mockTunnelLogsBridge{
		status: LiveStatusInfo{
			ServiceState: "active",
			TunnelState:  "disconnected",
		},
	}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleTunnelMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleTunnelMenu_ConnectAndDisconnect(t *testing.T) {
	mock := &mockTunnelLogsBridge{
		status: LiveStatusInfo{
			ServiceState: "active",
			TunnelState:  "disconnected",
		},
		configs: []store.ConfigItem{
			{
				ID:       "cfg-1",
				Name:     "Frankfurt High Speed",
				Protocol: "vless",
				Server:   "de.example.com",
				Port:     443,
			},
			{
				ID:       "cfg-2",
				Name:     "Amsterdam Backup",
				Protocol: "vmess",
				Server:   "nl.example.com",
				Port:     8443,
			},
		},
	}

	// 1. Connect -> select config 1 -> then back
	inputConnect := "1\n1\n0\n"
	readerConnect := bufio.NewReader(strings.NewReader(inputConnect))

	err := HandleTunnelMenu(mock, readerConnect)
	if err != nil {
		t.Fatalf("unexpected error during connect: %v", err)
	}
	if mock.connectedConfigID != "cfg-1" {
		t.Errorf("expected connectedConfigID == 'cfg-1', got %q", mock.connectedConfigID)
	}

	// 2. Disconnect -> then back
	inputDisconnect := "2\n0\n"
	readerDisconnect := bufio.NewReader(strings.NewReader(inputDisconnect))

	err = HandleTunnelMenu(mock, readerDisconnect)
	if err != nil {
		t.Fatalf("unexpected error during disconnect: %v", err)
	}
	if !mock.disconnectTunnelCalled {
		t.Errorf("expected DisconnectTunnel to be called")
	}
}

func TestHandleTunnelMenu_HealthCheckAndSafeMode(t *testing.T) {
	mock := &mockTunnelLogsBridge{
		status: LiveStatusInfo{
			ServiceState:      "active",
			TunnelState:       "connected",
			SafeModeRemaining: 45,
		},
		healthHealthy: true,
		healthLatency: 82,
	}

	// Option 3: Health check -> press enter to continue ->
	// Option 4: Confirm Safe Mode ->
	// Option 5: Rollback Safe Mode ->
	// Option 0: Back
	input := "3\n\n4\n5\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleTunnelMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.checkHealthCalled {
		t.Errorf("expected CheckHealth to be called")
	}
	if !mock.confirmSafeModeCalled {
		t.Errorf("expected ConfirmSafeMode to be called")
	}
	if !mock.rollbackSafeModeCalled {
		t.Errorf("expected RollbackSafeMode to be called")
	}
}

func TestHandleLogsMenu_BackOption(t *testing.T) {
	mock := &mockTunnelLogsBridge{}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleLogsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleLogsMenu_ViewRecent(t *testing.T) {
	mock := &mockTunnelLogsBridge{
		logs: []core.LogEntry{
			{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "INFO",
				Message:   "Core engine started successfully",
			},
			{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "WARN",
				Message:   "Latency spiked above 200ms",
			},
			{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "ERROR",
				Message:   "Handshake failed with upstream gateway",
			},
		},
	}

	// 1 -> view recent 50 logs -> press enter -> 0 -> back
	input := "1\n\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleLogsMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.getLogsLimit != 50 {
		t.Errorf("expected GetLogs(50), got limit %d", mock.getLogsLimit)
	}
}

func TestHandleLogsMenu_FollowLive(t *testing.T) {
	mock := &mockTunnelLogsBridge{
		logs: []core.LogEntry{
			{
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
				Level:     "INFO",
				Message:   "Core engine started successfully",
			},
		},
	}

	// 2 -> follow live logs -> enter to stop -> 0 -> back
	input := "2\n\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleLogsMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error during live follow logs: %v", err)
	}
}

