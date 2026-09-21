package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("item not found")
)

type Store interface {
	GetConfigs() ([]*ConfigItem, error)
	GetConfigByID(id string) (*ConfigItem, error)
	SaveConfig(cfg *ConfigItem) error
	SaveConfigsBatch(configs []*ConfigItem) error
	DeleteConfig(id string) error
	SetActiveConfig(id string) error
	GetActiveConfig() (*ConfigItem, error)
	UpdateLatency(id string, latencyMs int) error

	GetRoutingRules() ([]*RoutingRule, error)
	SaveRoutingRule(rule *RoutingRule) error
	DeleteRoutingRule(id string) error

	GetAdminUser() (*UserAccount, error)
	SetAdminUser(user *UserAccount) error

	GetSettings() (*SystemSettings, error)
	SaveSettings(settings *SystemSettings) error
}

type fileStoreData struct {
	Configs      map[string]*ConfigItem  `json:"configs"`
	ActiveID     string                  `json:"activeId"`
	RoutingRules map[string]*RoutingRule `json:"routingRules"`
	AdminUser    *UserAccount            `json:"adminUser"`
	Settings     *SystemSettings         `json:"settings"`
}

type FileStore struct {
	filePath string
	mu       sync.RWMutex
	data     fileStoreData
}

func New(filePath string) (*FileStore, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory for store: %w", err)
	}

	fs := &FileStore{
		filePath: filePath,
		data: fileStoreData{
			Configs:      make(map[string]*ConfigItem),
			RoutingRules: make(map[string]*RoutingRule),
			Settings: &SystemSettings{
				WebPort:         2080,
				SafeModeSeconds: 120,
				AutoStartTunnel: false,
			},
		},
	}

	if err := fs.load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Save initial state
			if err := fs.persist(); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("failed to load store data: %w", err)
		}
	}

	return fs, nil
}

func (fs *FileStore) normalizeData() {
	if fs.data.Configs == nil {
		fs.data.Configs = make(map[string]*ConfigItem)
	}
	if fs.data.RoutingRules == nil {
		fs.data.RoutingRules = make(map[string]*RoutingRule)
	}
	if fs.data.Settings == nil {
		fs.data.Settings = &SystemSettings{
			WebPort:         2080,
			SafeModeSeconds: 120,
			AutoStartTunnel: false,
		}
	}
}

func (fs *FileStore) load() error {
	bytes, err := os.ReadFile(fs.filePath)
	if err != nil {
		return err
	}

	var data fileStoreData
	if len(bytes) == 0 || json.Unmarshal(bytes, &data) != nil {
		// Corrupted or 0-byte primary file detected
		timestamp := time.Now().UnixNano()
		corruptArchive := fmt.Sprintf("%s.corrupt.%d", fs.filePath, timestamp)
		_ = os.Rename(fs.filePath, corruptArchive)

		// Try loading from .bak
		bakPath := fs.filePath + ".bak"
		bakBytes, bakErr := os.ReadFile(bakPath)
		if bakErr == nil && len(bakBytes) > 0 {
			var bakData fileStoreData
			if json.Unmarshal(bakBytes, &bakData) == nil {
				log.Printf("[store] WARNING: Corrupted store at %s recovered from backup (archived corrupt to %s)", fs.filePath, corruptArchive)
				fs.data = bakData
				fs.normalizeData()
				_ = fs.persist()
				return nil
			}
		}

		// Fallback: No valid backup, initialize clean state
		log.Printf("[store] CRITICAL: Corrupted store at %s and no valid backup found. Initializing clean store (archived corrupt to %s)", fs.filePath, corruptArchive)
		fs.data = fileStoreData{}
		fs.normalizeData()
		_ = fs.persist()
		return nil
	}

	fs.data = data
	fs.normalizeData()
	return nil
}

func (fs *FileStore) persist() error {
	bytes, err := json.MarshalIndent(fs.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store data: %w", err)
	}

	// 1. Write tmp file with explicit sync (STORE-01)
	tmpFile := fs.filePath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open tmp store file: %w", err)
	}

	if _, err := f.Write(bytes); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write tmp store file: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to sync tmp store file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close tmp store file: %w", err)
	}

	// 2. Snapshot current valid file to .bak before replacing
	bakFile := fs.filePath + ".bak"
	if existingBytes, err := os.ReadFile(fs.filePath); err == nil && len(existingBytes) > 0 {
		var dummy interface{}
		if json.Unmarshal(existingBytes, &dummy) == nil {
			writeSyncFile(bakFile, existingBytes)
		}
	}

	// 3. Atomic rename tmp to primary
	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		return fmt.Errorf("failed to atomic rename store file: %w", err)
	}

	// 4. Ensure .bak exists even if this was first write
	if _, err := os.Stat(bakFile); os.IsNotExist(err) {
		writeSyncFile(bakFile, bytes)
	}

	// 5. Best-effort parent directory sync
	if dirF, err := os.Open(filepath.Dir(fs.filePath)); err == nil {
		_ = dirF.Sync()
		_ = dirF.Close()
	}

	return nil
}

func writeSyncFile(path string, data []byte) {
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return
	}
	_ = f.Sync()
	_ = f.Close()
	_ = os.Rename(tmpPath, path)
}

func (fs *FileStore) GetConfigs() ([]*ConfigItem, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	items := make([]*ConfigItem, 0, len(fs.data.Configs))
	for _, cfg := range fs.data.Configs {
		itemCopy := *cfg
		items = append(items, &itemCopy)
	}
	return items, nil
}

func (fs *FileStore) GetConfigByID(id string) (*ConfigItem, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	cfg, ok := fs.data.Configs[id]
	if !ok {
		return nil, ErrNotFound
	}
	itemCopy := *cfg
	return &itemCopy, nil
}

func (fs *FileStore) SaveConfig(cfg *ConfigItem) error {
	if cfg == nil {
		return errors.New("nil config cannot be saved")
	}
	if cfg.ID == "" {
		return errors.New("empty config id")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	itemCopy := *cfg
	fs.data.Configs[cfg.ID] = &itemCopy
	return fs.persist()
}

func (fs *FileStore) SaveConfigsBatch(configs []*ConfigItem) error {
	if len(configs) == 0 {
		return nil
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	for _, cfg := range configs {
		if cfg == nil || cfg.ID == "" {
			continue
		}
		itemCopy := *cfg
		fs.data.Configs[cfg.ID] = &itemCopy
	}
	return fs.persist()
}

func (fs *FileStore) DeleteConfig(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.data.Configs, id)
	if fs.data.ActiveID == id {
		fs.data.ActiveID = ""
	}
	return fs.persist()
}

func (fs *FileStore) SetActiveConfig(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if id != "" {
		if _, ok := fs.data.Configs[id]; !ok {
			return ErrNotFound
		}
	}

	fs.data.ActiveID = id
	for k, v := range fs.data.Configs {
		v.IsActive = (k == id)
	}
	return fs.persist()
}

func (fs *FileStore) GetActiveConfig() (*ConfigItem, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if fs.data.ActiveID == "" {
		return nil, nil
	}

	cfg, ok := fs.data.Configs[fs.data.ActiveID]
	if !ok {
		return nil, nil
	}
	itemCopy := *cfg
	return &itemCopy, nil
}

func (fs *FileStore) UpdateLatency(id string, latencyMs int) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	cfg, ok := fs.data.Configs[id]
	if !ok {
		return ErrNotFound
	}
	cfg.LatencyMs = latencyMs
	return fs.persist()
}

func (fs *FileStore) GetRoutingRules() ([]*RoutingRule, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	rules := make([]*RoutingRule, 0, len(fs.data.RoutingRules))
	for _, r := range fs.data.RoutingRules {
		ruleCopy := *r
		rules = append(rules, &ruleCopy)
	}
	return rules, nil
}

func (fs *FileStore) SaveRoutingRule(rule *RoutingRule) error {
	if rule == nil {
		return errors.New("nil routing rule cannot be saved")
	}
	if rule.ID == "" {
		return errors.New("empty routing rule id")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	ruleCopy := *rule
	fs.data.RoutingRules[rule.ID] = &ruleCopy
	return fs.persist()
}

func (fs *FileStore) DeleteRoutingRule(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.data.RoutingRules, id)
	return fs.persist()
}

func (fs *FileStore) GetAdminUser() (*UserAccount, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if fs.data.AdminUser == nil {
		return nil, nil
	}
	userCopy := *fs.data.AdminUser
	return &userCopy, nil
}

func (fs *FileStore) SetAdminUser(user *UserAccount) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if user == nil {
		fs.data.AdminUser = nil
	} else {
		userCopy := *user
		fs.data.AdminUser = &userCopy
	}
	return fs.persist()
}

func (fs *FileStore) GetSettings() (*SystemSettings, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	settingsCopy := *fs.data.Settings
	return &settingsCopy, nil
}

func (fs *FileStore) SaveSettings(settings *SystemSettings) error {
	if settings == nil {
		return errors.New("nil settings cannot be saved")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	settingsCopy := *settings
	fs.data.Settings = &settingsCopy
	return fs.persist()
}
