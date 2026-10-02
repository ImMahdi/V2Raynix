package cli

import (
	"bufio"
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

type mockSettingsServiceBridge struct {
	status                 LiveStatusInfo
	statusErr              error
	settings               *store.SystemSettings
	getSettingsErr         error
	savedSettings          *store.SystemSettings
	saveSettingsErr        error
	appliedUsername        string
	appliedPassword        string
	applyCredentialsErr    error
	checkCoreUpdatesCalled bool
	updateStatusRet        *updater.UpdateStatus
	checkCoreUpdatesErr    error
	updatedCores           []string
	updateCoreErr          error
}

func (m *mockSettingsServiceBridge) IsDaemonRunning() bool               { return true }
func (m *mockSettingsServiceBridge) SetBaseURL(url string)              {}
func (m *mockSettingsServiceBridge) SetAuth(username, password string)  {}
func (m *mockSettingsServiceBridge) GetStatus() (LiveStatusInfo, error) {
	return m.status, m.statusErr
}
func (m *mockSettingsServiceBridge) ConnectTunnel(configID string) error { return nil }
func (m *mockSettingsServiceBridge) DisconnectTunnel() error              { return nil }
func (m *mockSettingsServiceBridge) ConfirmSafeMode() error               { return nil }
func (m *mockSettingsServiceBridge) RollbackSafeMode() error              { return nil }
func (m *mockSettingsServiceBridge) CheckHealth() (bool, int64, error)   { return true, 0, nil }
func (m *mockSettingsServiceBridge) ListConfigs() ([]store.ConfigItem, string, error) {
	return nil, "", nil
}
func (m *mockSettingsServiceBridge) AddConfigFromLink(link string) (*store.ConfigItem, error) {
	return nil, nil
}
func (m *mockSettingsServiceBridge) DeleteConfig(id string) error    { return nil }
func (m *mockSettingsServiceBridge) SetActiveConfig(id string) error { return nil }
func (m *mockSettingsServiceBridge) TestConfig(id string) (int, error) {
	return 0, nil
}
func (m *mockSettingsServiceBridge) TestAllConfigs() error { return nil }
func (m *mockSettingsServiceBridge) ListRules() ([]store.RoutingRule, error) {
	return nil, nil
}
func (m *mockSettingsServiceBridge) AddRule(rule store.RoutingRule) error { return nil }
func (m *mockSettingsServiceBridge) DeleteRule(id string) error           { return nil }
func (m *mockSettingsServiceBridge) ApplyPresets() error                  { return nil }
func (m *mockSettingsServiceBridge) GetLogs(limit int) ([]core.LogEntry, error) {
	return nil, nil
}
func (m *mockSettingsServiceBridge) CheckCoreUpdates() (*updater.UpdateStatus, error) {
	m.checkCoreUpdatesCalled = true
	return m.updateStatusRet, m.checkCoreUpdatesErr
}
func (m *mockSettingsServiceBridge) UpdateCore(engine string) error {
	m.updatedCores = append(m.updatedCores, engine)
	return m.updateCoreErr
}
func (m *mockSettingsServiceBridge) GetSettings() (*store.SystemSettings, error) {
	if m.settings != nil {
		return m.settings, m.getSettingsErr
	}
	return &store.SystemSettings{
		WebPort:                    2080,
		SafeModeSeconds:            120,
		HealthCheckIntervalMinutes: 5,
		HealthCheckURL:             "https://www.google.com/generate_204",
	}, m.getSettingsErr
}
func (m *mockSettingsServiceBridge) SaveSettings(settings *store.SystemSettings) error {
	m.savedSettings = settings
	return m.saveSettingsErr
}
func (m *mockSettingsServiceBridge) ApplyCredentials(username, password string) error {
	m.appliedUsername = username
	m.appliedPassword = password
	return m.applyCredentialsErr
}

func TestHandleSettingsMenu_BackOption(t *testing.T) {
	mock := &mockSettingsServiceBridge{}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleSettingsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleSettingsMenu_ChangeWebPort(t *testing.T) {
	mock := &mockSettingsServiceBridge{
		settings: &store.SystemSettings{
			WebPort:                    2080,
			SafeModeSeconds:            120,
			HealthCheckIntervalMinutes: 5,
			HealthCheckURL:             "https://www.google.com/generate_204",
		},
	}
	input := "1\n9090\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleSettingsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if mock.savedSettings == nil {
		t.Fatal("expected SaveSettings to be called on bridge")
	}
	if mock.savedSettings.WebPort != 9090 {
		t.Fatalf("expected WebPort 9090, got %d", mock.savedSettings.WebPort)
	}
}

func TestHandleServiceMenu_BackOption(t *testing.T) {
	mock := &mockSettingsServiceBridge{}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleServiceMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleServiceMenu_CheckCoreUpdates(t *testing.T) {
	mock := &mockSettingsServiceBridge{
		updateStatusRet: &updater.UpdateStatus{
			Cores: map[string]updater.CoreInfo{
				"xray": {
					Name:            "xray",
					CurrentVersion:  "1.8.4",
					LatestVersion:   "1.8.4",
					UpdateAvailable: false,
				},
			},
		},
	}
	input := "1\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleServiceMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !mock.checkCoreUpdatesCalled {
		t.Fatal("expected CheckCoreUpdates to be called on bridge")
	}
}

func TestHandleSettingsMenu_ChangeCredentials(t *testing.T) {
	mock := &mockSettingsServiceBridge{}
	input := "2\nnewadmin\nsecretpassword\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleSettingsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if mock.appliedUsername != "newadmin" {
		t.Fatalf("expected username 'newadmin', got '%s'", mock.appliedUsername)
	}
	if mock.appliedPassword != "secretpassword" {
		t.Fatalf("expected password 'secretpassword', got '%s'", mock.appliedPassword)
	}
}

func TestHandleSettingsMenu_ConfigureWatchdog(t *testing.T) {
	mock := &mockSettingsServiceBridge{
		settings: &store.SystemSettings{
			WebPort:                    2080,
			HealthCheckIntervalMinutes: 5,
			HealthCheckURL:             "https://www.google.com/generate_204",
		},
	}
	input := "3\n15\nhttps://cloudflare.com/cdn-cgi/trace\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleSettingsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if mock.savedSettings == nil {
		t.Fatal("expected SaveSettings to be called on bridge")
	}
	if mock.savedSettings.HealthCheckIntervalMinutes != 15 {
		t.Fatalf("expected interval 15, got %d", mock.savedSettings.HealthCheckIntervalMinutes)
	}
	if mock.savedSettings.HealthCheckURL != "https://cloudflare.com/cdn-cgi/trace" {
		t.Fatalf("expected URL 'https://cloudflare.com/cdn-cgi/trace', got '%s'", mock.savedSettings.HealthCheckURL)
	}
}

func TestHandleServiceMenu_CheckCoreUpdatesWithUpgrade(t *testing.T) {
	mock := &mockSettingsServiceBridge{
		updateStatusRet: &updater.UpdateStatus{
			Cores: map[string]updater.CoreInfo{
				"sing-box": {
					Name:            "sing-box",
					CurrentVersion:  "1.8.0",
					LatestVersion:   "1.9.0",
					UpdateAvailable: true,
				},
			},
		},
	}
	input := "1\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleServiceMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !mock.checkCoreUpdatesCalled {
		t.Fatal("expected CheckCoreUpdates to be called on bridge")
	}
	if len(mock.updatedCores) != 1 || mock.updatedCores[0] != "sing-box" {
		t.Fatalf("expected sing-box to be updated, got %v", mock.updatedCores)
	}
}

