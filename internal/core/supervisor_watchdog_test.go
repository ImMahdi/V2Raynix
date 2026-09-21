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

func TestSupervisor_LogFDCloseAndReap(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-fd-test-*")
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

	xrayLog, err := os.OpenFile(filepath.Join(tempDir, "xray.log"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open dummy xray log: %v", err)
	}
	tunLog, err := os.OpenFile(filepath.Join(tempDir, "tun.log"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to open dummy tun log: %v", err)
	}

	sup.xrayLogFile = xrayLog
	sup.tunLogFile = tunLog

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "TEST_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start mock process: %v", err)
	}
	sup.xrayCmd = cmd
	sup.watchProcess(cmd, "xray")

	// Call StopTunnel to initiate teardown
	if err := sup.StopTunnel(); err != nil {
		t.Fatalf("failed to stop tunnel: %v", err)
	}

	// 1. Check supervisor handles are nil
	if sup.xrayLogFile != nil {
		t.Errorf("expected sup.xrayLogFile to be nil after StopTunnel")
	}
	if sup.tunLogFile != nil {
		t.Errorf("expected sup.tunLogFile to be nil after StopTunnel")
	}

	// 2. Check the underlying files are actually closed (Write returns error)
	_, writeErr := xrayLog.Write([]byte("test"))
	if writeErr == nil {
		t.Errorf("expected write to closed xrayLog to fail, but succeeded")
	}

	_, writeErr2 := tunLog.Write([]byte("test"))
	if writeErr2 == nil {
		t.Errorf("expected write to closed tunLog to fail, but succeeded")
	}

	// 3. Check process has been reaped (ProcessState is populated or process is dead)
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if cmd.ProcessState == nil {
		t.Errorf("expected cmd.ProcessState to be non-nil (reaped) after teardown")
	}
}
