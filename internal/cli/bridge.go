package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/pinger"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

// EngineUpdateStatus is an alias for updater.UpdateStatus
type EngineUpdateStatus = updater.UpdateStatus

// TUIBridge defines the unified operations interface for the TUI, coordinating
// between the active daemon REST API (when online) and direct file store (when offline).
type TUIBridge interface {
	IsDaemonRunning() bool
	SetBaseURL(url string)
	SetAuth(username, password string)
	GetStatus() (LiveStatusInfo, error)
	ConnectTunnel(configID string) error
	DisconnectTunnel() error
	ConfirmSafeMode() error
	RollbackSafeMode() error
	CheckHealth() (bool, int64, error)
	ListConfigs() ([]store.ConfigItem, string, error)
	AddConfigFromLink(link string) (*store.ConfigItem, error)
	DeleteConfig(id string) error
	SetActiveConfig(id string) error
	TestConfig(id string) (int, error)
	TestAllConfigs() error
	ListRules() ([]store.RoutingRule, error)
	AddRule(rule store.RoutingRule) error
	DeleteRule(id string) error
	ApplyPresets() error
	GetLogs(limit int) ([]core.LogEntry, error)
	CheckCoreUpdates() (*updater.UpdateStatus, error)
	UpdateCore(engine string) error
	GetSettings() (*store.SystemSettings, error)
	SaveSettings(settings *store.SystemSettings) error
	ApplyCredentials(username, password string) error
}

type bridgeImpl struct {
	dataDir    string
	webPort    int
	baseURL    string
	store      store.Store
	username   string
	password   string
	token      string
	httpClient *http.Client
	mu         sync.Mutex
}

// NewTUIBridge initializes a dual-mode bridge.
func NewTUIBridge(dataDir string, webPort int) TUIBridge {
	dbPath := filepath.Join(dataDir, "v2raynix.json")
	st, _ := store.New(dbPath)

	resolvedPort := webPort
	if resolvedPort <= 0 && st != nil {
		if settings, err := st.GetSettings(); err == nil && settings != nil && settings.WebPort > 0 {
			resolvedPort = settings.WebPort
		}
	}
	if resolvedPort <= 0 {
		resolvedPort = 2080
	}

	return &bridgeImpl{
		dataDir:    dataDir,
		webPort:    resolvedPort,
		baseURL:    fmt.Sprintf("http://127.0.0.1:%d", resolvedPort),
		store:      st,
		username:   "admin",
		password:   "admin",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (b *bridgeImpl) SetBaseURL(url string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.baseURL = strings.TrimRight(url, "/")
}

func (b *bridgeImpl) SetAuth(username, password string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.username = username
	b.password = password
	b.token = ""
}

// IsDaemonRunning checks whether the local daemon is responding to HTTP requests.
func (b *bridgeImpl) IsDaemonRunning() bool {
	probeClient := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := probeClient.Get(b.baseURL + "/api/status")
	if err == nil {
		_ = resp.Body.Close()
		return true
	}

	resp2, err2 := probeClient.Get(b.baseURL + "/api/tunnel/status")
	if err2 == nil {
		_ = resp2.Body.Close()
		return true
	}

	return false
}

func (b *bridgeImpl) login() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	u := b.username
	if u == "" {
		u = "admin"
	}
	p := b.password
	if p == "" {
		p = "admin"
	}

	payload := map[string]string{
		"username": u,
		"password": p,
	}
	reqBytes, _ := json.Marshal(payload)
	resp, err := b.httpClient.Post(b.baseURL+"/api/auth/login", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return fmt.Errorf("daemon authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon authentication rejected (status %d)", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode daemon token: %w", err)
	}

	b.token = result.Token
	return nil
}

func (b *bridgeImpl) doRequest(method, path string, reqBody interface{}, respTarget interface{}) error {
	if b.token == "" {
		if err := b.login(); err != nil {
			return err
		}
	}

	execReq := func() (*http.Response, error) {
		var bodyReader io.Reader
		if reqBody != nil {
			data, err := json.Marshal(reqBody)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(data)
		}
		req, err := http.NewRequest(method, b.baseURL+path, bodyReader)
		if err != nil {
			return nil, err
		}
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if b.token != "" {
			req.Header.Set("Authorization", "Bearer "+b.token)
		}
		return b.httpClient.Do(req)
	}

	resp, err := execReq()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If token expired, re-login once and retry
	if resp.StatusCode == http.StatusUnauthorized {
		b.mu.Lock()
		b.token = ""
		b.mu.Unlock()
		if err := b.login(); err != nil {
			return err
		}
		resp, err = execReq()
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return errors.New(errResp.Error)
		}
		return fmt.Errorf("api %s %s returned status %d", method, path, resp.StatusCode)
	}

	if respTarget != nil {
		return json.NewDecoder(resp.Body).Decode(respTarget)
	}
	return nil
}

func (b *bridgeImpl) GetStatus() (LiveStatusInfo, error) {
	if !b.IsDaemonRunning() {
		info := LiveStatusInfo{
			ServiceState:  "inactive",
			TunnelState:   "disconnected",
			HealthState:   "not verified",
			WebPort:       b.webPort,
			WebIP:         "127.0.0.1",
		}
		if b.store != nil {
			if active, err := b.store.GetActiveConfig(); err == nil && active != nil {
				info.ConfigName = active.Name
				info.Protocol = active.Protocol
			}
		}
		return info, nil
	}

	// Connected mode
	var status struct {
		State              string `json:"state"`
		ActiveConfigID     string `json:"activeConfigId"`
		ActiveConfigName   string `json:"activeConfigName"`
		TunInterface       string `json:"tunInterface"`
		UptimeSeconds      int64  `json:"uptimeSeconds"`
		UploadSpeedBps     int64  `json:"uploadSpeedBps"`
		DownloadSpeedBps   int64  `json:"downloadSpeedBps"`
		TotalUploadBytes   int64  `json:"totalUploadBytes"`
		TotalDownloadBytes int64  `json:"totalDownloadBytes"`
		SafeMode           struct {
			IsActive         bool `json:"isActive"`
			RemainingSeconds int  `json:"remainingSeconds"`
		} `json:"safeMode"`
	}

	err := b.doRequest(http.MethodGet, "/api/status", nil, &status)
	if err != nil {
		// Fallback to /api/tunnel/status
		err = b.doRequest(http.MethodGet, "/api/tunnel/status", nil, &status)
	}
	if err != nil {
		return LiveStatusInfo{}, err
	}

	info := LiveStatusInfo{
		ServiceState:      "active",
		TunnelState:       status.State,
		ConfigName:        status.ActiveConfigName,
		UploadSpeed:       status.UploadSpeedBps,
		DownloadSpeed:     status.DownloadSpeedBps,
		UploadTotal:       status.TotalUploadBytes,
		DownloadTotal:     status.TotalDownloadBytes,
		Uptime:            status.UptimeSeconds,
		SafeModeRemaining: status.SafeMode.RemainingSeconds,
		WebPort:           b.webPort,
		WebIP:             "127.0.0.1",
	}

	if b.store != nil && status.ActiveConfigID != "" {
		if cfg, err := b.store.GetConfigByID(status.ActiveConfigID); err == nil && cfg != nil {
			info.Protocol = cfg.Protocol
		}
	}

	return info, nil
}

func (b *bridgeImpl) ConnectTunnel(configID string) error {
	if !b.IsDaemonRunning() {
		return errors.New("service is inactive: start service in menu [6] first")
	}

	if configID != "" {
		_ = b.doRequest(http.MethodPost, fmt.Sprintf("/api/configs/%s/activate", configID), nil, nil)
	}

	return b.doRequest(http.MethodPost, "/api/tunnel/connect", nil, nil)
}

func (b *bridgeImpl) DisconnectTunnel() error {
	if !b.IsDaemonRunning() {
		return errors.New("service is inactive: start service in menu [6] first")
	}
	return b.doRequest(http.MethodPost, "/api/tunnel/disconnect", nil, nil)
}

func (b *bridgeImpl) ConfirmSafeMode() error {
	if !b.IsDaemonRunning() {
		return errors.New("service is inactive: start service in menu [6] first")
	}
	return b.doRequest(http.MethodPost, "/api/tunnel/safe-mode/confirm", nil, nil)
}

func (b *bridgeImpl) RollbackSafeMode() error {
	if !b.IsDaemonRunning() {
		return errors.New("service is inactive: start service in menu [6] first")
	}
	return b.doRequest(http.MethodPost, "/api/tunnel/safe-mode/rollback", nil, nil)
}

func (b *bridgeImpl) CheckHealth() (bool, int64, error) {
	if !b.IsDaemonRunning() {
		return false, 0, errors.New("service is inactive: start service in menu [6] first")
	}

	var resp struct {
		Healthy   bool   `json:"healthy"`
		LatencyMs int64  `json:"latencyMs"`
		Error     string `json:"error"`
	}
	err := b.doRequest(http.MethodPost, "/api/health/check", nil, &resp)
	if err != nil {
		return false, 0, err
	}
	if resp.Error != "" {
		return resp.Healthy, resp.LatencyMs, errors.New(resp.Error)
	}
	return resp.Healthy, resp.LatencyMs, nil
}

func (b *bridgeImpl) ListConfigs() ([]store.ConfigItem, string, error) {
	if b.store == nil {
		return nil, "", errors.New("store not initialized")
	}

	cfgs, err := b.store.GetConfigs()
	if err != nil {
		return nil, "", err
	}

	var activeID string
	if active, err := b.store.GetActiveConfig(); err == nil && active != nil {
		activeID = active.ID
	}

	items := make([]store.ConfigItem, len(cfgs))
	for i, c := range cfgs {
		items[i] = *c
	}
	return items, activeID, nil
}

func (b *bridgeImpl) AddConfigFromLink(link string) (*store.ConfigItem, error) {
	if b.IsDaemonRunning() {
		var created store.ConfigItem
		err := b.doRequest(http.MethodPost, "/api/configs", map[string]string{"content": link}, &created)
		if err == nil && created.ID != "" {
			return &created, nil
		}
	}

	// Offline or fallback to direct store
	if b.store == nil {
		return nil, errors.New("store not initialized")
	}
	item, err := configmgr.ParseShareLink(link)
	if err != nil {
		return nil, err
	}
	if err := b.store.SaveConfig(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (b *bridgeImpl) DeleteConfig(id string) error {
	if b.IsDaemonRunning() {
		_ = b.doRequest(http.MethodDelete, fmt.Sprintf("/api/configs/%s", id), nil, nil)
	}
	if b.store != nil {
		return b.store.DeleteConfig(id)
	}
	return nil
}

func (b *bridgeImpl) SetActiveConfig(id string) error {
	if b.IsDaemonRunning() {
		return b.doRequest(http.MethodPost, fmt.Sprintf("/api/configs/%s/activate", id), nil, nil)
	}
	if b.store == nil {
		return errors.New("store not initialized")
	}
	return b.store.SetActiveConfig(id)
}

func (b *bridgeImpl) TestConfig(id string) (int, error) {
	if b.IsDaemonRunning() {
		var resp struct {
			LatencyMs int `json:"latencyMs"`
		}
		err := b.doRequest(http.MethodPost, fmt.Sprintf("/api/configs/%s/test", id), nil, &resp)
		if err == nil {
			return resp.LatencyMs, nil
		}
	}

	if b.store == nil {
		return -1, errors.New("store not initialized")
	}
	cfg, err := b.store.GetConfigByID(id)
	if err != nil || cfg == nil {
		return -1, fmt.Errorf("config not found: %s", id)
	}

	lat, err := pinger.TestConfigRealDelay(context.Background(), cfg, 3*time.Second)
	_ = b.store.UpdateLatency(id, lat)
	return lat, err
}

func (b *bridgeImpl) TestAllConfigs() error {
	if b.IsDaemonRunning() {
		err := b.doRequest(http.MethodPost, "/api/configs/test-all", nil, nil)
		if err == nil {
			return nil
		}
	}

	if b.store == nil {
		return errors.New("store not initialized")
	}
	cfgs, err := b.store.GetConfigs()
	if err != nil {
		return err
	}
	results := pinger.BatchRealTestContext(context.Background(), cfgs, 5, 3*time.Second)
	return b.store.UpdateLatenciesBatch(results)
}

func (b *bridgeImpl) ListRules() ([]store.RoutingRule, error) {
	if b.store == nil {
		return nil, errors.New("store not initialized")
	}

	rules, err := b.store.GetRoutingRules()
	if err != nil {
		return nil, err
	}
	items := make([]store.RoutingRule, len(rules))
	for i, r := range rules {
		items[i] = *r
	}
	return items, nil
}

func (b *bridgeImpl) AddRule(rule store.RoutingRule) error {
	if b.IsDaemonRunning() {
		_ = b.doRequest(http.MethodPost, "/api/routing/rules", rule, nil)
	}
	if b.store != nil {
		return b.store.SaveRoutingRule(&rule)
	}
	return nil
}

func (b *bridgeImpl) DeleteRule(id string) error {
	if b.IsDaemonRunning() {
		_ = b.doRequest(http.MethodDelete, fmt.Sprintf("/api/routing/rules/%s", id), nil, nil)
	}
	if b.store != nil {
		return b.store.DeleteRoutingRule(id)
	}
	return nil
}

func (b *bridgeImpl) ApplyPresets() error {
	presets := []store.RoutingRule{
		{
			ID:         "preset-ir-domain",
			Target:     "geosite:category-ir",
			TargetType: "domain",
			Action:     "direct",
			Priority:   100,
			IsEnabled:  true,
		},
		{
			ID:         "preset-ir-ip",
			Target:     "geoip:ir",
			TargetType: "ip",
			Action:     "direct",
			Priority:   90,
			IsEnabled:  true,
		},
		{
			ID:         "preset-ads",
			Target:     "geosite:category-ads-all",
			TargetType: "domain",
			Action:     "block",
			Priority:   80,
			IsEnabled:  true,
		},
	}

	for _, p := range presets {
		if err := b.AddRule(p); err != nil {
			return err
		}
	}
	return nil
}

func (b *bridgeImpl) GetLogs(limit int) ([]core.LogEntry, error) {
	if !b.IsDaemonRunning() {
		return []core.LogEntry{}, nil
	}

	var entries []core.LogEntry
	err := b.doRequest(http.MethodGet, "/api/logs", nil, &entries)
	if err != nil {
		err = b.doRequest(http.MethodGet, "/api/system/logs", nil, &entries)
	}
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries, nil
}

func (b *bridgeImpl) CheckCoreUpdates() (*updater.UpdateStatus, error) {
	if b.IsDaemonRunning() {
		var status updater.UpdateStatus
		err := b.doRequest(http.MethodPost, "/api/system/check-updates", nil, &status)
		if err == nil {
			return &status, nil
		}
	}

	upd := updater.NewUpdater(b.dataDir)
	status, err := upd.CheckUpdates(false)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (b *bridgeImpl) UpdateCore(engine string) error {
	if b.IsDaemonRunning() {
		return b.doRequest(http.MethodPost, "/api/system/update-core", map[string]string{"core": engine}, nil)
	}

	upd := updater.NewUpdater(b.dataDir)
	return upd.UpdateCore(engine)
}

func (b *bridgeImpl) GetSettings() (*store.SystemSettings, error) {
	if b.store == nil {
		return nil, errors.New("store not initialized")
	}
	return b.store.GetSettings()
}

func (b *bridgeImpl) SaveSettings(settings *store.SystemSettings) error {
	if b.store == nil {
		return errors.New("store not initialized")
	}
	if err := b.store.SaveSettings(settings); err != nil {
		return err
	}
	if settings != nil && settings.WebPort > 0 {
		b.mu.Lock()
		b.webPort = settings.WebPort
		b.baseURL = fmt.Sprintf("http://127.0.0.1:%d", b.webPort)
		b.mu.Unlock()
	}
	return nil
}

func (b *bridgeImpl) ApplyCredentials(username, password string) error {
	if b.store == nil {
		return errors.New("store not initialized")
	}
	if err := ApplyCredentials(b.store, username, password); err != nil {
		return err
	}
	b.mu.Lock()
	b.username = username
	b.password = password
	b.token = ""
	b.mu.Unlock()
	return nil
}
