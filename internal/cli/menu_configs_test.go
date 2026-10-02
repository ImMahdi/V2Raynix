package cli

import (
	"bufio"
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

type mockConfigsRoutingBridge struct {
	status              LiveStatusInfo
	statusErr           error
	configs             []store.ConfigItem
	activeConfigID      string
	listConfigsErr      error
	addedLink           string
	addedConfigRet      *store.ConfigItem
	addConfigErr        error
	deletedConfigID     string
	deleteConfigErr     error
	setActiveConfigID   string
	setActiveConfigErr  error
	testedConfigID      string
	testConfigErr       error
	testAllConfigsErr   error
	rules               []store.RoutingRule
	listRulesErr        error
	addedRule           store.RoutingRule
	addRuleErr          error
	deletedRuleID       string
	deleteRuleErr       error
	applyPresetsCalled  bool
	applyPresetsErr     error
}

func (m *mockConfigsRoutingBridge) IsDaemonRunning() bool               { return true }
func (m *mockConfigsRoutingBridge) SetBaseURL(url string)              {}
func (m *mockConfigsRoutingBridge) SetAuth(username, password string)  {}
func (m *mockConfigsRoutingBridge) GetStatus() (LiveStatusInfo, error) {
	return m.status, m.statusErr
}
func (m *mockConfigsRoutingBridge) ConnectTunnel(configID string) error { return nil }
func (m *mockConfigsRoutingBridge) DisconnectTunnel() error              { return nil }
func (m *mockConfigsRoutingBridge) ConfirmSafeMode() error               { return nil }
func (m *mockConfigsRoutingBridge) RollbackSafeMode() error              { return nil }
func (m *mockConfigsRoutingBridge) CheckHealth() (bool, int64, error)   { return true, 0, nil }
func (m *mockConfigsRoutingBridge) ListConfigs() ([]store.ConfigItem, string, error) {
	return m.configs, m.activeConfigID, m.listConfigsErr
}
func (m *mockConfigsRoutingBridge) AddConfigFromLink(link string) (*store.ConfigItem, error) {
	m.addedLink = link
	if m.addedConfigRet != nil {
		return m.addedConfigRet, m.addConfigErr
	}
	return &store.ConfigItem{ID: "new-cfg-1", Name: "Imported Node", Protocol: "vless"}, m.addConfigErr
}
func (m *mockConfigsRoutingBridge) DeleteConfig(id string) error {
	m.deletedConfigID = id
	return m.deleteConfigErr
}
func (m *mockConfigsRoutingBridge) SetActiveConfig(id string) error {
	m.setActiveConfigID = id
	m.activeConfigID = id
	return m.setActiveConfigErr
}
func (m *mockConfigsRoutingBridge) TestConfig(id string) (int, error) {
	m.testedConfigID = id
	return 42, m.testConfigErr
}
func (m *mockConfigsRoutingBridge) TestAllConfigs() error {
	return m.testAllConfigsErr
}
func (m *mockConfigsRoutingBridge) ListRules() ([]store.RoutingRule, error) {
	return m.rules, m.listRulesErr
}
func (m *mockConfigsRoutingBridge) AddRule(rule store.RoutingRule) error {
	m.addedRule = rule
	return m.addRuleErr
}
func (m *mockConfigsRoutingBridge) DeleteRule(id string) error {
	m.deletedRuleID = id
	return m.deleteRuleErr
}
func (m *mockConfigsRoutingBridge) ApplyPresets() error {
	m.applyPresetsCalled = true
	return m.applyPresetsErr
}
func (m *mockConfigsRoutingBridge) GetLogs(limit int) ([]core.LogEntry, error) {
	return nil, nil
}
func (m *mockConfigsRoutingBridge) CheckCoreUpdates() (*updater.UpdateStatus, error) {
	return nil, nil
}
func (m *mockConfigsRoutingBridge) UpdateCore(engine string) error { return nil }
func (m *mockConfigsRoutingBridge) GetSettings() (*store.SystemSettings, error) {
	return nil, nil
}
func (m *mockConfigsRoutingBridge) SaveSettings(settings *store.SystemSettings) error {
	return nil
}
func (m *mockConfigsRoutingBridge) ApplyCredentials(username, password string) error {
	return nil
}

func TestHandleConfigsMenu_BackOption(t *testing.T) {
	mock := &mockConfigsRoutingBridge{}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleConfigsMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleConfigsMenu_ListAndAdd(t *testing.T) {
	mock := &mockConfigsRoutingBridge{
		configs: []store.ConfigItem{
			{
				ID:       "cfg-existing",
				Name:     "Existing Node",
				Protocol: "vmess",
				Server:   "1.2.3.4",
				Port:     443,
			},
		},
	}
	link := "vless://uuid@domain:443?security=reality..."
	input := "2\n" + link + "\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleConfigsMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.addedLink != link {
		t.Errorf("expected AddConfigFromLink called with %q, got %q", link, mock.addedLink)
	}
}

func TestHandleConfigsMenu_DeleteAndSetActive(t *testing.T) {
	mock := &mockConfigsRoutingBridge{
		configs: []store.ConfigItem{
			{
				ID:       "cfg-1",
				Name:     "Node One",
				Protocol: "vless",
				Server:   "node1.example.com",
				Port:     443,
			},
			{
				ID:       "cfg-2",
				Name:     "Node Two",
				Protocol: "vmess",
				Server:   "node2.example.com",
				Port:     8443,
			},
		},
	}

	// 4 -> select 2 -> 3 -> select 1 -> 0 -> back
	input := "4\n2\n3\n1\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleConfigsMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.setActiveConfigID != "cfg-2" {
		t.Errorf("expected SetActiveConfig called with 'cfg-2', got %q", mock.setActiveConfigID)
	}
	if mock.deletedConfigID != "cfg-1" {
		t.Errorf("expected DeleteConfig called with 'cfg-1', got %q", mock.deletedConfigID)
	}
}

func TestHandleRoutingMenu_BackOption(t *testing.T) {
	mock := &mockConfigsRoutingBridge{}
	input := "0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleRoutingMenu(mock, reader)
	if err != nil {
		t.Fatalf("expected nil error on back option, got %v", err)
	}
}

func TestHandleRoutingMenu_ApplyPresets(t *testing.T) {
	mock := &mockConfigsRoutingBridge{}
	input := "3\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleRoutingMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.applyPresetsCalled {
		t.Errorf("expected ApplyPresets to be called")
	}
}

func TestHandleRoutingMenu_AddAndDeleteRule(t *testing.T) {
	mock := &mockConfigsRoutingBridge{
		rules: []store.RoutingRule{
			{
				ID:         "rule-existing-1",
				Target:     "example.com",
				TargetType: "domain",
				Action:     "direct",
				Priority:   100,
				IsEnabled:  true,
			},
		},
	}

	// 2 -> Add rule: Type 1 (domain) -> Target "iran.ir" -> Action 1 (direct) ->
	// 4 -> Delete rule: Select 1 ->
	// 0 -> Back
	input := "2\n1\niran.ir\n1\n4\n1\n0\n"
	reader := bufio.NewReader(strings.NewReader(input))

	err := HandleRoutingMenu(mock, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.addedRule.Target != "iran.ir" {
		t.Errorf("expected addedRule.Target == 'iran.ir', got %q", mock.addedRule.Target)
	}
	if mock.addedRule.Action != "direct" {
		t.Errorf("expected addedRule.Action == 'direct', got %q", mock.addedRule.Action)
	}
	if mock.deletedRuleID != "rule-existing-1" {
		t.Errorf("expected deletedRuleID == 'rule-existing-1', got %q", mock.deletedRuleID)
	}
}
