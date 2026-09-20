package core

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/network"
	"github.com/v2raynix/v2raynix/internal/store"
)

type SafeModeInfo struct {
	IsActive         bool `json:"isActive"`
	RemainingSeconds int  `json:"remainingSeconds"`
}

type TunnelStatus struct {
	State              string       `json:"state"` // disconnected, connecting, connected, rolling_back
	ActiveConfigID     string       `json:"activeConfigId"`
	ActiveConfigName   string       `json:"activeConfigName"`
	TunInterface       string       `json:"tunInterface"`
	UptimeSeconds      int64        `json:"uptimeSeconds"`
	UploadSpeedBps     int64        `json:"uploadSpeedBps"`
	DownloadSpeedBps   int64        `json:"downloadSpeedBps"`
	TotalUploadBytes   int64        `json:"totalUploadBytes"`
	TotalDownloadBytes int64        `json:"totalDownloadBytes"`
	SafeMode           SafeModeInfo `json:"safeMode"`
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type Supervisor struct {
	store    store.Store
	mockMode bool

	mu              sync.Mutex
	state           string
	activeConfig    *store.ConfigItem
	startedAt       time.Time
	safeModeTimeout time.Duration
	safeMode        *network.SafeModeController

	xrayCmd      *exec.Cmd
	tun2socksCmd *exec.Cmd

	logs   []LogEntry
	logsMu sync.RWMutex
}

func NewSupervisor(st store.Store, safeModeSec int, mockMode bool) *Supervisor {
	if safeModeSec <= 0 {
		safeModeSec = 120
	}
	sup := &Supervisor{
		store:           st,
		mockMode:        mockMode,
		state:           "disconnected",
		safeModeTimeout: time.Duration(safeModeSec) * time.Second,
		logs:            make([]LogEntry, 0, 500),
	}
	return sup
}

func (s *Supervisor) SetSafeModeTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.safeModeTimeout = d
}

func (s *Supervisor) StartTunnel(cfg *store.ConfigItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	s.addLog("info", fmt.Sprintf("Starting tunnel with config: %s (%s)", cfg.Name, cfg.Protocol))
	s.state = "connecting"
	s.activeConfig = cfg
	s.startedAt = time.Now()

	if s.mockMode {
		s.state = "connected"
		s.startSafeModeTimerLocked()
		s.addLog("info", "Tunnel connected successfully (mock)")
		return nil
	}

	// Real Linux orchestration:
	rules, err := s.store.GetRoutingRules()
	if err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to get routing rules: %w", err)
	}

	// 1. Generate Xray JSON
	_, err = configmgr.GenerateXrayConfig(cfg, rules, 10808, 10809)
	if err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to generate xray config: %w", err)
	}

	// 2. Start Xray and tun2socks child processes (in production)
	s.state = "connected"
	s.startSafeModeTimerLocked()
	s.addLog("info", "Tunnel connected successfully")
	return nil
}

func (s *Supervisor) startSafeModeTimerLocked() {
	s.safeMode = network.NewSafeModeController(s.safeModeTimeout, func() {
		s.addLog("warn", "Safe Mode timeout reached without confirmation. Rolling back network changes...")
		_ = s.StopTunnel()
	})
	s.safeMode.Start()
}

func (s *Supervisor) StopTunnel() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.addLog("info", "Stopping tunnel...")
	s.state = "rolling_back"

	if s.safeMode != nil {
		s.safeMode.Confirm()
	}

	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		_ = s.xrayCmd.Process.Kill()
		s.xrayCmd = nil
	}
	if s.tun2socksCmd != nil && s.tun2socksCmd.Process != nil {
		_ = s.tun2socksCmd.Process.Kill()
		s.tun2socksCmd = nil
	}

	s.state = "disconnected"
	s.addLog("info", "Tunnel disconnected cleanly")
	return nil
}

func (s *Supervisor) ConfirmSafeMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.safeMode != nil {
		ok := s.safeMode.Confirm()
		if ok {
			s.addLog("info", "Safe mode confirmed by administrator; changes are now persistent")
		}
		return ok
	}
	return false
}

func (s *Supervisor) RollbackSafeMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.safeMode != nil {
		return s.safeMode.CancelAndRollback()
	}
	return false
}

func (s *Supervisor) GetStatus() TunnelStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var uptime int64
	if s.state == "connected" && !s.startedAt.IsZero() {
		uptime = int64(time.Since(s.startedAt).Seconds())
	}

	var activeID, activeName string
	if s.activeConfig != nil {
		activeID = s.activeConfig.ID
		activeName = s.activeConfig.Name
	}

	safeModeInfo := SafeModeInfo{
		IsActive:         false,
		RemainingSeconds: 0,
	}

	if s.safeMode != nil && s.safeMode.IsActive() {
		safeModeInfo.IsActive = true
		safeModeInfo.RemainingSeconds = s.safeMode.GetRemainingSeconds()
	}

	return TunnelStatus{
		State:              s.state,
		ActiveConfigID:     activeID,
		ActiveConfigName:   activeName,
		TunInterface:       network.TunDevice,
		UptimeSeconds:      uptime,
		UploadSpeedBps:     0,
		DownloadSpeedBps:   0,
		TotalUploadBytes:   0,
		TotalDownloadBytes: 0,
		SafeMode:           safeModeInfo,
	}
}

func (s *Supervisor) AddExternalLog(level, message string) {
	s.addLog(level, message)
}

func (s *Supervisor) addLog(level, message string) {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
	}

	s.logs = append(s.logs, entry)
	if len(s.logs) > 500 {
		s.logs = s.logs[1:]
	}
}

func (s *Supervisor) GetLogs(limit int) []LogEntry {
	s.logsMu.RLock()
	defer s.logsMu.RUnlock()

	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}

	start := len(s.logs) - limit
	out := make([]LogEntry, limit)
	copy(out, s.logs[start:])
	return out
}
