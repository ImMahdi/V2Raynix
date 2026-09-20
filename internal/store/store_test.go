package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/store"
)

func TestStore_ConfigOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.json")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// 1. Initially empty
	configs, err := s.GetConfigs()
	if err != nil {
		t.Fatalf("unexpected error getting configs: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(configs))
	}

	// 2. Save a config
	cfg1 := &store.ConfigItem{
		ID:        "cfg-1",
		Name:      "Server Frankfurt",
		Protocol:  "vless",
		Server:    "1.2.3.4",
		Port:      443,
		RawURL:    "vless://user@1.2.3.4:443?type=tcp&security=reality",
		LatencyMs: 120,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	err = s.SaveConfig(cfg1)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// 3. Retrieve and verify
	configs, err = s.GetConfigs()
	if err != nil {
		t.Fatalf("failed to get configs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Name != "Server Frankfurt" {
		t.Errorf("expected name 'Server Frankfurt', got '%s'", configs[0].Name)
	}

	// 4. Set Active
	err = s.SetActiveConfig("cfg-1")
	if err != nil {
		t.Fatalf("failed to set active config: %v", err)
	}

	active, err := s.GetActiveConfig()
	if err != nil {
		t.Fatalf("failed to get active config: %v", err)
	}
	if active == nil || active.ID != "cfg-1" {
		t.Fatalf("expected active config 'cfg-1', got %+v", active)
	}

	// 5. Delete Config
	err = s.DeleteConfig("cfg-1")
	if err != nil {
		t.Fatalf("failed to delete config: %v", err)
	}

	configs, err = s.GetConfigs()
	if err != nil {
		t.Fatalf("failed to get configs after deletion: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs after deletion, got %d", len(configs))
	}
}

func TestStore_RoutingRules(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.json")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	rule := &store.RoutingRule{
		ID:         "rule-1",
		Target:     "geosite:ir",
		TargetType: "domain",
		Action:     "direct",
		IsEnabled:  true,
		Priority:   10,
	}

	err = s.SaveRoutingRule(rule)
	if err != nil {
		t.Fatalf("failed to save rule: %v", err)
	}

	rules, err := s.GetRoutingRules()
	if err != nil {
		t.Fatalf("failed to get routing rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Action != "direct" {
		t.Errorf("expected action direct, got %s", rules[0].Action)
	}

	err = s.DeleteRoutingRule("rule-1")
	if err != nil {
		t.Fatalf("failed to delete rule: %v", err)
	}

	rules, err = s.GetRoutingRules()
	if err != nil {
		t.Fatalf("failed to get rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules after deletion, got %d", len(rules))
	}
}
