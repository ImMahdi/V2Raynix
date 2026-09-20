package network_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/network"
)

func TestSafeMode_AutoRollbackOnTimeout(t *testing.T) {
	var rollbackCalled atomic.Bool

	// 50ms short timeout for fast unit testing
	sm := network.NewSafeModeController(50*time.Millisecond, func() {
		rollbackCalled.Store(true)
	})

	sm.Start()

	if !sm.IsActive() {
		t.Errorf("expected SafeMode to be active after Start()")
	}

	// Wait for timeout to fire
	time.Sleep(100 * time.Millisecond)

	if !rollbackCalled.Load() {
		t.Errorf("expected rollback function to be called on timeout")
	}

	if sm.IsActive() {
		t.Errorf("expected SafeMode to be inactive after rollback")
	}
}

func TestSafeMode_ConfirmCancelsRollback(t *testing.T) {
	var rollbackCalled atomic.Bool

	sm := network.NewSafeModeController(100*time.Millisecond, func() {
		rollbackCalled.Store(true)
	})

	sm.Start()

	// User confirms after 20ms
	time.Sleep(20 * time.Millisecond)
	confirmed := sm.Confirm()
	if !confirmed {
		t.Errorf("expected confirm to succeed")
	}

	if sm.IsActive() {
		t.Errorf("expected SafeMode to be inactive after confirmation")
	}

	// Wait beyond original timeout duration
	time.Sleep(150 * time.Millisecond)

	if rollbackCalled.Load() {
		t.Errorf("rollback should NOT have been called when confirmed")
	}
}

func TestSafeMode_ManualCancelRollback(t *testing.T) {
	var rollbackCalled atomic.Bool

	sm := network.NewSafeModeController(500*time.Millisecond, func() {
		rollbackCalled.Store(true)
	})

	sm.Start()

	cancelled := sm.CancelAndRollback()
	if !cancelled {
		t.Errorf("expected cancel to succeed")
	}

	if !rollbackCalled.Load() {
		t.Errorf("expected rollback function to be invoked immediately upon CancelAndRollback")
	}

	if sm.IsActive() {
		t.Errorf("expected SafeMode to be inactive")
	}
}
