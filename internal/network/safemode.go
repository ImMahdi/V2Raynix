package network

import (
	"sync"
	"time"
)

// SafeModeController manages a countdown timer that reverts network routing unless confirmed
type SafeModeController struct {
	timeout    time.Duration
	onRollback func()

	mu        sync.Mutex
	active    bool
	startedAt time.Time
	timer     *time.Timer
}

// NewSafeModeController creates a new safe mode controller
func NewSafeModeController(timeout time.Duration, onRollback func()) *SafeModeController {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &SafeModeController{
		timeout:    timeout,
		onRollback: onRollback,
	}
}

// Start begins the countdown timer
func (sm *SafeModeController) Start() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.timer != nil {
		sm.timer.Stop()
	}

	sm.active = true
	sm.startedAt = time.Now()

	sm.timer = time.AfterFunc(sm.timeout, func() {
		sm.mu.Lock()
		if !sm.active {
			sm.mu.Unlock()
			return
		}
		sm.active = false
		sm.mu.Unlock()

		if sm.onRollback != nil {
			sm.onRollback()
		}
	})
}

// Confirm commits changes and stops the auto-rollback countdown
func (sm *SafeModeController) Confirm() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.active {
		return false
	}

	if sm.timer != nil {
		sm.timer.Stop()
	}
	sm.active = false
	return true
}

// CancelAndRollback immediately invokes rollback and stops countdown
func (sm *SafeModeController) CancelAndRollback() bool {
	sm.mu.Lock()
	if !sm.active {
		sm.mu.Unlock()
		return false
	}

	if sm.timer != nil {
		sm.timer.Stop()
	}
	sm.active = false
	sm.mu.Unlock()

	if sm.onRollback != nil {
		sm.onRollback()
	}
	return true
}

// IsActive returns whether safe mode is currently running
func (sm *SafeModeController) IsActive() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.active
}

// GetRemainingSeconds returns seconds left in the countdown
func (sm *SafeModeController) GetRemainingSeconds() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.active {
		return 0
	}

	elapsed := time.Since(sm.startedAt)
	remaining := sm.timeout - elapsed
	if remaining <= 0 {
		return 0
	}

	return int(remaining.Seconds())
}
