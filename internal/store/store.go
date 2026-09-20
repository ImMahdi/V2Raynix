package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrNotFound = errors.New("item not found")
)

type Store interface {
	GetConfigs() ([]*ConfigItem, error)
	GetConfigByID(id string) (*ConfigItem, error)
	SaveConfig(cfg *ConfigItem) error
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

func (fs *FileStore) load() error {
	bytes, err := os.ReadFile(fs.filePath)
	if err != nil {
		return err
	}

	var data fileStoreData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Errorf("corrupted store json: %w", err)
	}

	if data.Configs == nil {
		data.Configs = make(map[string]*ConfigItem)
	}
	if data.RoutingRules == nil {
		data.RoutingRules = make(map[string]*RoutingRule)
	}
	if data.Settings == nil {
		data.Settings = &SystemSettings{
			WebPort:         2080,
			SafeModeSeconds: 120,
			AutoStartTunnel: false,
		}
	}

	fs.data = data
	return nil
}

func (fs *FileStore) persist() error {
	bytes, err := json.MarshalIndent(fs.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store data: %w", err)
	}

	tmpFile := fs.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0600); err != nil {
		return fmt.Errorf("failed to write tmp store file: %w", err)
	}

	if err := os.Rename(tmpFile, fs.filePath); err != nil {
		return fmt.Errorf("failed to atomic rename store file: %w", err)
	}

	return nil
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
	fs.mu.Lock()
	defer fs.mu.Unlock()

	itemCopy := *cfg
	fs.data.Configs[cfg.ID] = &itemCopy
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
	fs.mu.Lock()
	defer fs.mu.Unlock()

	settingsCopy := *settings
	fs.data.Settings = &settingsCopy
	return fs.persist()
}
