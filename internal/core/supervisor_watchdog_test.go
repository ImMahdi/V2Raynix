package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/store"
)

// TestHelperProcess is used as an OS-agnostic mock child process
func TestHelperProcess(t *testing.T) {
	if os.Getenv("TEST_HELPER_PROCESS") != "1" {
		return
	}
	time.Sleep(50 * time.Millisecond)
	os.Exit(1)
}

func TestSupervisor_ChildCrashWatchdog(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-watchdog-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.New(filepath.Join(tempDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	sup := NewSupervisor(st, 120, true, tempDir)
	sup.state = "connected"

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "TEST_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}

	sup.xrayCmd = cmd
	sup.watchProcess(cmd, "xray")

	// Wait for process to exit and watchdog to trigger emergency teardown
	success := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sup.GetStatus().State == "disconnected" {
			success = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if !success {
		t.Fatalf("expected state to transition to 'disconnected' after child crash, got '%s'", sup.GetStatus().State)
	}

	// Verify log recorded the unexpected crash
	logs := sup.GetLogs(10)
	foundCrashLog := false
	for _, l := range logs {
		if l.Level == "error" {
			foundCrashLog = true
			break
		}
	}
	if !foundCrashLog {
		t.Errorf("expected error log recording child crash, but none found")
	}
}
