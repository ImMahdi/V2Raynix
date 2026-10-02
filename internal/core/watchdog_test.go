package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/store"
)

func TestWatchdog_ProbeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	st, err := store.New(filepath.Join(tempDir, "store.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sup := core.NewSupervisor(st, 60, true, tempDir)

	healthy, latency, err := sup.CheckHealth(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("expected nil err, got: %v", err)
	}
	if !healthy {
		t.Fatalf("expected healthy true, got false")
	}
	if latency <= 0 {
		t.Fatalf("expected latency > 0, got %v", latency)
	}
}

func TestWatchdog_ProbeFailureAndRetry(t *testing.T) {
	// 1. HTTP 500 failure
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts500.Close()

	tempDir := t.TempDir()
	st, err := store.New(filepath.Join(tempDir, "store.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	sup := core.NewSupervisor(st, 60, true, tempDir)

	healthy, _, err := sup.CheckHealth(context.Background(), ts500.URL)
	if healthy {
		t.Fatalf("expected healthy false for HTTP 500, got true")
	}
	if err == nil {
		t.Fatalf("expected error for HTTP 500, got nil")
	}

	// 2. Timeout failure
	tsTimeout := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer tsTimeout.Close()

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	healthyTimeout, _, errTimeout := sup.CheckHealth(timeoutCtx, tsTimeout.URL)
	if healthyTimeout {
		t.Fatalf("expected healthy false for timeout, got true")
	}
	if errTimeout == nil {
		t.Fatalf("expected error for timeout, got nil")
	}
}

func TestWatchdog_AutoRecoveryTrigger(t *testing.T) {
	tsFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsFail.Close()

	tempDir := t.TempDir()
	st, err := store.New(filepath.Join(tempDir, "store.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := &store.ConfigItem{
		ID:       "cfg-test-recovery",
		Name:     "Test Server",
		Protocol: "vless",
		Server:   "127.0.0.1",
		Port:     443,
	}
	if err := st.SaveConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	if err := st.SetActiveConfig(cfg.ID); err != nil {
		t.Fatalf("failed to set active config: %v", err)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	settings.HealthCheckIntervalMinutes = 1
	settings.HealthCheckURL = tsFail.URL
	if err := st.SaveSettings(settings); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	sup := core.NewSupervisor(st, 60, true, tempDir)
	sup.SetWatchdogInterval(25 * time.Millisecond)
	sup.SetWatchdogRetryDelay(10 * time.Millisecond)

	if err := sup.StartTunnel(cfg); err != nil {
		t.Fatalf("failed to start tunnel: %v", err)
	}
	defer sup.StopTunnel()

	// Wait for watchdog to fail twice and trigger RestartTunnel
	deadline := time.Now().Add(2 * time.Second)
	recovered := false
	for time.Now().Before(deadline) {
		if sup.GetRestartCount() >= 1 {
			recovered = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !recovered {
		t.Fatalf("expected watchdog to trigger RestartTunnel after 2 consecutive failures, restartCount=%d", sup.GetRestartCount())
	}
}
