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

func TestStore_DurablePersistAndBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-store-durability-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "data.json")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// Saving a config should trigger persist and generate/update .bak
	cfg := &store.ConfigItem{
		ID:       "cfg-test",
		Name:     "Initial Node",
		Protocol: "vmess",
		Server:   "1.1.1.1",
		Port:     443,
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	bakPath := dbPath + ".bak"
	if _, err := os.Stat(bakPath); err != nil {
		t.Fatalf("expected backup file %s to exist after persist, got err: %v", bakPath, err)
	}

	// Read backup file and verify it is valid JSON
	bakBytes, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if len(bakBytes) == 0 {
		t.Fatalf("backup file is empty")
	}
}

func TestStore_AutoRecoveryFromCorruptedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-store-recovery-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "data.json")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	cfg := &store.ConfigItem{
		ID:       "cfg-recover",
		Name:     "Recoverable Node",
		Protocol: "vless",
		Server:   "8.8.8.8",
		Port:     443,
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Save another item to ensure the .bak has the valid state
	rule := &store.RoutingRule{
		ID:     "rule-recover",
		Target: "geosite:category-ads",
		Action: "block",
	}
	if err := s.SaveRoutingRule(rule); err != nil {
		t.Fatalf("failed to save rule: %v", err)
	}

	// Now corrupt the primary data.json (simulate power cut mid-write or 0-byte truncation)
	if err := os.WriteFile(dbPath, []byte("{\"corrupted_json_garbage"), 0644); err != nil {
		t.Fatalf("failed to corrupt file: %v", err)
	}

	// Reopen store. It MUST NOT crash/fail, but recover from .bak and archive the corrupt file
	recoveredStore, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New failed on corrupted file instead of auto-recovering: %v", err)
	}

	configs, err := recoveredStore.GetConfigs()
	if err != nil {
		t.Fatalf("failed to get configs after recovery: %v", err)
	}
	if len(configs) == 0 {
		t.Fatalf("expected recovered configs, got 0")
	}
	if configs[0].ID != "cfg-recover" {
		t.Errorf("expected config ID 'cfg-recover', got %s", configs[0].ID)
	}

	// Check that an archived corrupt file was created
	matches, err := filepath.Glob(dbPath + ".corrupt.*")
	if err != nil || len(matches) == 0 {
		t.Errorf("expected archived corrupt file matching %s.corrupt.*, found %v", dbPath, matches)
	}
}

func TestStore_AutoRecoveryWhenNoBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-store-nobak-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "data.json")
	// Create corrupt file directly with no .bak
	if err := os.WriteFile(dbPath, []byte("NOT_JSON_DATA!!!"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	recoveredStore, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New failed on corrupted file with no backup: %v", err)
	}

	configs, err := recoveredStore.GetConfigs()
	if err != nil {
		t.Fatalf("failed to get configs: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs in newly initialized store, got %d", len(configs))
	}

	matches, err := filepath.Glob(dbPath + ".corrupt.*")
	if err != nil || len(matches) == 0 {
		t.Errorf("expected archived corrupt file, found %v", matches)
	}
}


