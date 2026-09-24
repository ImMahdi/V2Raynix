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
	switch os.Getenv("TEST_HELPER_PROCESS") {
	case "1":
		time.Sleep(50 * time.Millisecond)
		os.Exit(1)
	case "long":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		return
	}
}

func TestSupervisor_ReconnectWatchdogRace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "v2raynix-watchdog-race-*")
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

	// Start initial xray and tun2socks processes (cmd1, tunCmd1)
	cmd1 := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd1.Env = append(os.Environ(), "TEST_HELPER_PROCESS=long")
	if err := cmd1.Start(); err != nil {
		t.Fatalf("failed to start cmd1: %v", err)
	}
	sup.xrayCmd = cmd1
	sup.watchProcess(cmd1, "xray")

	tunCmd1 := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	tunCmd1.Env = append(os.Environ(), "TEST_HELPER_PROCESS=long")
	if err := tunCmd1.Start(); err != nil {
		_ = cmd1.Process.Kill()
		t.Fatalf("failed to start tunCmd1: %v", err)
	}
	sup.tun2socksCmd = tunCmd1
	sup.watchProcess(tunCmd1, "tun2socks")

	// Start replacement processes (cmd2, tunCmd2) representing reconnect / new config
	cmd2 := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd2.Env = append(os.Environ(), "TEST_HELPER_PROCESS=long")
	if err := cmd2.Start(); err != nil {
		_ = cmd1.Process.Kill()
		_ = tunCmd1.Process.Kill()
		t.Fatalf("failed to start cmd2: %v", err)
	}
	defer func() {
		if cmd2.Process != nil {
			_ = cmd2.Process.Kill()
		}
	}()

	tunCmd2 := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	tunCmd2.Env = append(os.Environ(), "TEST_HELPER_PROCESS=long")
	if err := tunCmd2.Start(); err != nil {
		_ = cmd1.Process.Kill()
		_ = tunCmd1.Process.Kill()
		_ = cmd2.Process.Kill()
		t.Fatalf("failed to start tunCmd2: %v", err)
	}
	defer func() {
		if tunCmd2.Process != nil {
			_ = tunCmd2.Process.Kill()
		}
	}()

	sup.xrayCmd = cmd2
	sup.watchProcess(cmd2, "xray")
	sup.tun2socksCmd = tunCmd2
	sup.watchProcess(tunCmd2, "tun2socks")

	// Now kill the old processes (cmd1, tunCmd1).
	// Their watchdog goroutines will wake up on cmd.Wait().
	_ = cmd1.Process.Kill()
	_ = tunCmd1.Process.Kill()

	// Wait briefly for the watchdog goroutines to run
	time.Sleep(200 * time.Millisecond)

	// Without the fix, the stale watchdog for cmd1 or tunCmd1 will see state == "connected",
	// assume a crash occurred, and trigger stopTunnelLocked(), terminating cmd2/tunCmd2 and
	// setting state to "disconnected".
	if state := sup.GetStatus().State; state != "connected" {
		t.Fatalf("expected state to remain 'connected', got '%s'", state)
	}
	if sup.xrayCmd != cmd2 {
		t.Fatalf("expected active xrayCmd to still be cmd2, got %v", sup.xrayCmd)
	}
	if sup.tun2socksCmd != tunCmd2 {
		t.Fatalf("expected active tun2socksCmd to still be tunCmd2, got %v", sup.tun2socksCmd)
	}
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
