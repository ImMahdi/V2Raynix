package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	dataDir  string

	mu              sync.Mutex
	state           string
	activeConfig    *store.ConfigItem
	startedAt       time.Time
	safeModeTimeout time.Duration
	safeMode        *network.SafeModeController

	activeRemoteIP string
	activeIface    string
	activeGw       string
	activeSSHPort  int
	activeWebPort  int

	xrayCmd      *exec.Cmd
	tun2socksCmd *exec.Cmd

	logs   []LogEntry
	logsMu sync.RWMutex
}

func NewSupervisor(st store.Store, safeModeSec int, mockMode bool, dataDirs ...string) *Supervisor {
	if safeModeSec <= 0 {
		safeModeSec = 120
	}
	dir := "/etc/v2raynix"
	if len(dataDirs) > 0 && dataDirs[0] != "" {
		dir = dataDirs[0]
	}
	sup := &Supervisor{
		store:           st,
		mockMode:        mockMode,
		dataDir:         dir,
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

func (s *Supervisor) SetActiveConfig(cfg *store.ConfigItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeConfig = cfg
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
	// Kill previous child processes if running to prevent port or tun0 contention
	if s.tun2socksCmd != nil && s.tun2socksCmd.Process != nil {
		_ = s.tun2socksCmd.Process.Kill()
		_ = s.tun2socksCmd.Wait()
		s.tun2socksCmd = nil
	}
	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		_ = s.xrayCmd.Process.Kill()
		_ = s.xrayCmd.Wait()
		s.xrayCmd = nil
	}

	rules, err := s.store.GetRoutingRules()
	if err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to get routing rules: %w", err)
	}

	// 1. Generate Xray JSON
	rawJSON, err := configmgr.GenerateXrayConfig(cfg, rules, 10808, 10809)
	if err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to generate xray config: %w", err)
	}

	_ = os.MkdirAll(s.dataDir, 0755)
	xrayConfigPath := filepath.Join(s.dataDir, "xray-active.json")
	if err := os.WriteFile(xrayConfigPath, []byte(rawJSON), 0644); err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to write xray config: %w", err)
	}

	// 2. Discover default route & SSH port
	iface, gw, err := network.GetDefaultRoute()
	if err != nil {
		s.addLog("warn", fmt.Sprintf("Could not auto-detect default route (%v), using fallback dev", err))
	}
	sshPort := network.DetectSSHPort()
	remoteIP, _ := network.ResolveHost(cfg.Server)
	if remoteIP == "" {
		remoteIP = cfg.Server
	}

	s.activeRemoteIP = remoteIP
	s.activeIface = iface
	s.activeGw = gw
	s.activeSSHPort = sshPort

	// 3. Start Xray child process
	s.addLog("info", fmt.Sprintf("Launching Xray core on 127.0.0.1:10808 (inbound) -> %s:%d...", cfg.Server, cfg.Port))
	xrayCmd := exec.Command("xray", "run", "-c", xrayConfigPath)
	xrayLogPath := filepath.Join(s.dataDir, "xray.log")
	if xLog, err := os.OpenFile(xrayLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		xrayCmd.Stdout = xLog
		xrayCmd.Stderr = xLog
	}
	if err := xrayCmd.Start(); err != nil {
		s.state = "disconnected"
		return fmt.Errorf("failed to start xray: %w", err)
	}
	s.xrayCmd = xrayCmd
	s.watchProcess(xrayCmd, "xray")

	// Give Xray 200ms to initialize
	time.Sleep(200 * time.Millisecond)

	// 4. Setup routing commands
	if iface != "" && gw != "" && remoteIP != "" {
		webPort := 2080
		if s.store != nil {
			if settings, err := s.store.GetSettings(); err == nil && settings != nil && settings.WebPort > 0 {
				webPort = settings.WebPort
			}
		}
		s.activeWebPort = webPort

		s.addLog("info", fmt.Sprintf("Configuring Linux routing rules (iface: %s, gw: %s, remoteIP: %s, sshPort: %d, webPort: %d)...", iface, gw, remoteIP, sshPort, webPort))
		// Clean up any stale rules or leftover tun device from previous runs to ensure clean slate
		cleanupCmds := network.BuildCleanupCommands(remoteIP, iface, gw, sshPort, webPort)
		_ = network.ExecuteCommands(cleanupCmds)

		cmds := network.BuildRoutingCommands(remoteIP, iface, gw, sshPort, webPort)
		errs := network.ExecuteCommands(cmds)
		for _, e := range errs {
			s.addLog("warn", e.Error())
		}
	}

	// 5. Start tun2socks
	s.addLog("info", "Starting tun2socks interface tun0...")
	tunCmd := exec.Command("tun2socks", "-d", "tun0", "-p", "socks5://127.0.0.1:10808")
	tunLogPath := filepath.Join(s.dataDir, "tun2socks.log")
	if tLog, err := os.OpenFile(tunLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		tunCmd.Stdout = tLog
		tunCmd.Stderr = tLog
	}
	if err := tunCmd.Start(); err != nil {
		s.addLog("error", fmt.Sprintf("failed to start tun2socks: %v", err))
		_ = s.stopTunnelLocked()
		return fmt.Errorf("failed to start tun2socks: %w", err)
	}
	s.tun2socksCmd = tunCmd
	s.watchProcess(tunCmd, "tun2socks")

	s.state = "connected"
	s.startSafeModeTimerLocked()
	s.addLog("info", "Tunnel connected successfully and routing applied")
	return nil
}

func (s *Supervisor) watchProcess(cmd *exec.Cmd, name string) {
	go func() {
		err := cmd.Wait()

		s.mu.Lock()
		defer s.mu.Unlock()

		// If we are still in "connected" state, this exit was unexpected (crash or kill).
		// Emergency teardown is required to avoid an unreachable blackhole routing state.
		if s.state == "connected" {
			s.addLog("error", fmt.Sprintf("Child process %s exited unexpectedly (%v). Initiating emergency teardown to prevent network blackhole...", name, err))
			_ = s.stopTunnelLocked()
		}
	}()
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
	return s.stopTunnelLocked()
}

func (s *Supervisor) stopTunnelLocked() error {
	s.addLog("info", "Stopping tunnel...")
	s.state = "rolling_back"

	if s.safeMode != nil {
		s.safeMode.Confirm()
	}

	if s.tun2socksCmd != nil && s.tun2socksCmd.Process != nil {
		_ = s.tun2socksCmd.Process.Kill()
		s.tun2socksCmd = nil
	}
	if s.xrayCmd != nil && s.xrayCmd.Process != nil {
		_ = s.xrayCmd.Process.Kill()
		s.xrayCmd = nil
	}

	// Clean up Linux network routing
	if !s.mockMode && s.activeIface != "" && s.activeGw != "" && s.activeRemoteIP != "" {
		webPort := s.activeWebPort
		if webPort <= 0 {
			webPort = 2080
		}
		s.addLog("info", "Tearing down Linux routing table and tun0 interface...")
		cleanupCmds := network.BuildCleanupCommands(s.activeRemoteIP, s.activeIface, s.activeGw, s.activeSSHPort, webPort)
		_ = network.ExecuteCommands(cleanupCmds)
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

	if s.safeMode != nil && s.safeMode.IsActive() {
		s.safeMode.Confirm() // Disarm pending timer to avoid concurrent/duplicate rollback
		_ = s.stopTunnelLocked()
		s.addLog("warn", "Safe mode rollback initiated by administrator")
		return true
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
	} else if s.store != nil {
		if cfg, err := s.store.GetActiveConfig(); err == nil && cfg != nil {
			activeID = cfg.ID
			activeName = cfg.Name
		}
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
