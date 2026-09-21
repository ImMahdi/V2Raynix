package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
)

func TestSupervisor_Lifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	sup := core.NewSupervisor(s, 10, true) // mockMode = true for unit testing without root/xray binaries

	// 1. Initial status: disconnected
	status := sup.GetStatus()
	if status.State != "disconnected" {
		t.Errorf("expected disconnected state, got %s", status.State)
	}

	// 2. Start tunnel with a config
	cfg := &store.ConfigItem{
		ID:       "cfg-test-1",
		Name:     "Mock Server",
		Protocol: "vless",
		Server:   "1.1.1.1",
		Port:     443,
		RawURL:   "vless://uuid@1.1.1.1:443?security=none",
	}

	err = sup.StartTunnel(cfg)
	if err != nil {
		t.Fatalf("failed to start tunnel: %v", err)
	}

	status = sup.GetStatus()
	if status.State != "connected" {
		t.Errorf("expected connected state, got %s", status.State)
	}
	if status.ActiveConfigID != "cfg-test-1" {
		t.Errorf("expected active config cfg-test-1, got %s", status.ActiveConfigID)
	}
	if !status.SafeMode.IsActive {
		t.Errorf("expected SafeMode to be active after tunnel start")
	}

	// 3. Confirm Safe Mode
	confirmed := sup.ConfirmSafeMode()
	if !confirmed {
		t.Errorf("expected confirm safe mode to succeed")
	}

	status = sup.GetStatus()
	if status.SafeMode.IsActive {
		t.Errorf("expected SafeMode to be inactive after confirmation")
	}

	// 4. Stop Tunnel
	err = sup.StopTunnel()
	if err != nil {
		t.Fatalf("failed to stop tunnel: %v", err)
	}

	status = sup.GetStatus()
	if status.State != "disconnected" {
		t.Errorf("expected disconnected state, got %s", status.State)
	}
}

func TestSupervisor_SafeModeRollback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	// Set short safe mode timeout (50ms)
	sup := core.NewSupervisor(s, 1, true) // mockMode = true
	sup.SetSafeModeTimeout(50 * time.Millisecond)

	cfg := &store.ConfigItem{
		ID:       "cfg-test-2",
		Name:     "Mock Server 2",
		Protocol: "vless",
		Server:   "2.2.2.2",
		Port:     443,
		RawURL:   "vless://uuid@2.2.2.2:443?security=none",
	}

	err = sup.StartTunnel(cfg)
	if err != nil {
		t.Fatalf("failed to start tunnel: %v", err)
	}

	// Wait for safe mode timer to expire and trigger auto-rollback
	time.Sleep(100 * time.Millisecond)

	status := sup.GetStatus()
	if status.State != "disconnected" {
		t.Errorf("expected tunnel to roll back to disconnected, got %s", status.State)
	}
}

func TestSupervisor_DisconnectedActiveConfigPreserved(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	cfg := &store.ConfigItem{
		ID:       "cfg-preselected",
		Name:     "Preselected Server",
		Protocol: "vless",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vless://uuid@1.2.3.4:443?security=none",
	}
	_ = s.SaveConfig(cfg)
	_ = s.SetActiveConfig("cfg-preselected")

	sup := core.NewSupervisor(s, 10, true)

	// Even when disconnected (before StartTunnel is called),
	// GetStatus() MUST return the active config from the store!
	status := sup.GetStatus()
	if status.ActiveConfigID != "cfg-preselected" {
		t.Fatalf("expected activeConfigId 'cfg-preselected' while disconnected, got '%s'", status.ActiveConfigID)
	}
	if status.ActiveConfigName != "Preselected Server" {
		t.Fatalf("expected activeConfigName 'Preselected Server' while disconnected, got '%s'", status.ActiveConfigName)
	}
}

func TestSupervisor_RollbackSafeMode_ManualDeadlockCheck(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-supervisor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	sup := core.NewSupervisor(s, 120, true) // 120s timeout, mockMode

	cfg := &store.ConfigItem{
		ID:       "cfg-test-rollback",
		Name:     "Mock Server Rollback",
		Protocol: "vless",
		Server:   "3.3.3.3",
		Port:     443,
		RawURL:   "vless://uuid@3.3.3.3:443?security=none",
	}

	err = sup.StartTunnel(cfg)
	if err != nil {
		t.Fatalf("failed to start tunnel: %v", err)
	}

	done := make(chan bool)
	go func() {
		ok := sup.RollbackSafeMode()
		done <- ok
	}()

	select {
	case ok := <-done:
		if !ok {
			t.Errorf("expected RollbackSafeMode to return true")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("DEADLOCK DETECTED: RollbackSafeMode failed to return within 1s")
	}

	status := sup.GetStatus()
	if status.State != "disconnected" {
		t.Errorf("expected disconnected state after rollback, got %s", status.State)
	}
}

