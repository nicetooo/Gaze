package cache

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"Gaze/pkg/fileutil"
)

// AppPackage represents cached app package information
type AppPackage struct {
	Name                 string   `json:"name"`
	Label                string   `json:"label"`
	Icon                 string   `json:"icon"`
	Type                 string   `json:"type"`
	State                string   `json:"state"`
	VersionName          string   `json:"versionName"`
	VersionCode          string   `json:"versionCode"`
	MinSdkVersion        string   `json:"minSdkVersion"`
	TargetSdkVersion     string   `json:"targetSdkVersion"`
	Permissions          []string `json:"permissions"`
	Activities           []string `json:"activities"`
	LaunchableActivities []string `json:"launchableActivities"`
}

// Settings represents persistent application settings
type Settings struct {
	LastActive   map[string]int64 `json:"lastActive"`
	PinnedSerial string           `json:"pinnedSerial"`
}

// Service manages application cache and settings persistence
type Service struct {
	// Paths
	configDir    string
	cachePath    string
	historyPath  string
	settingsPath string

	// AAPT cache
	aaptCache   map[string]AppPackage
	aaptCacheMu sync.RWMutex

	// Settings state (kept in sync with file)
	lastActive   map[string]int64
	lastActiveMu sync.RWMutex

	pinnedSerial string
	pinnedMu     sync.RWMutex

	// History
	historyMu sync.Mutex

	// Logger function (optional)
	logFunc func(format string, args ...interface{})
}

// Config for creating a new CacheService
type Config struct {
	ConfigDir string
	LogFunc   func(format string, args ...interface{})
}

// New creates a new CacheService instance
func New(cfg Config) (*Service, error) {
	configDir := cfg.ConfigDir
	if configDir == "" {
		var err error
		configDir, err = os.UserConfigDir()
		if err != nil {
			configDir = os.TempDir()
		}
		configDir = filepath.Join(configDir, "Gaze")
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	s := &Service{
		configDir:    configDir,
		cachePath:    filepath.Join(configDir, "aapt_cache.json"),
		historyPath:  filepath.Join(configDir, "history.json"),
		settingsPath: filepath.Join(configDir, "settings.json"),
		aaptCache:    make(map[string]AppPackage),
		lastActive:   make(map[string]int64),
		logFunc:      cfg.LogFunc,
	}

	// Load persisted data
	s.loadCache()
	s.loadSettings()

	return s, nil
}

// log writes a log message if logFunc is set
func (s *Service) log(format string, args ...interface{}) {
	if s.logFunc != nil {
		s.logFunc(format, args...)
	}
}

// ========================================
// AAPT Cache Methods
// ========================================

// GetCachedPackage returns a cached package if it exists
func (s *Service) GetCachedPackage(packageName string) (AppPackage, bool) {
	s.aaptCacheMu.RLock()
	defer s.aaptCacheMu.RUnlock()
	pkg, exists := s.aaptCache[packageName]
	return pkg, exists
}

// SetCachedPackage caches package information
func (s *Service) SetCachedPackage(packageName string, pkg AppPackage) {
	s.aaptCacheMu.Lock()
	s.aaptCache[packageName] = pkg
	s.aaptCacheMu.Unlock()
}

// SaveCache persists the AAPT cache to disk
func (s *Service) SaveCache() error {
	s.aaptCacheMu.RLock()
	data, err := json.Marshal(s.aaptCache)
	s.aaptCacheMu.RUnlock()

	if err != nil {
		s.log("Error marshaling cache: %v", err)
		return err
	}

	if err := fileutil.WriteFileAtomic(s.cachePath, data, 0644); err != nil {
		s.log("Error saving cache to %s: %v", s.cachePath, err)
		return err
	}
	return nil
}

func (s *Service) loadCache() {
	s.aaptCacheMu.Lock()
	defer s.aaptCacheMu.Unlock()

	data, err := os.ReadFile(s.cachePath)
	if err != nil {
		return
	}

	if err := json.Unmarshal(data, &s.aaptCache); err != nil {
		log.Printf("[cache] Failed to unmarshal aapt cache: %v", err)
	}
}

// ========================================
// Settings Methods
// ========================================

// GetLastActive returns the last active timestamp for a device
func (s *Service) GetLastActive(deviceID string) int64 {
	s.lastActiveMu.RLock()
	defer s.lastActiveMu.RUnlock()
	return s.lastActive[deviceID]
}

// SetLastActive updates the last active timestamp for a device
func (s *Service) SetLastActive(deviceID string, timestamp int64) {
	s.lastActiveMu.Lock()
	s.lastActive[deviceID] = timestamp
	s.lastActiveMu.Unlock()
}

// GetAllLastActive returns a copy of all last active timestamps
func (s *Service) GetAllLastActive() map[string]int64 {
	s.lastActiveMu.RLock()
	defer s.lastActiveMu.RUnlock()
	result := make(map[string]int64, len(s.lastActive))
	for k, v := range s.lastActive {
		result[k] = v
	}
	return result
}

// GetPinnedSerial returns the pinned device serial
func (s *Service) GetPinnedSerial() string {
	s.pinnedMu.RLock()
	defer s.pinnedMu.RUnlock()
	return s.pinnedSerial
}

// SetPinnedSerial sets the pinned device serial
func (s *Service) SetPinnedSerial(serial string) {
	s.pinnedMu.Lock()
	s.pinnedSerial = serial
	s.pinnedMu.Unlock()
}

// SaveSettings persists settings to disk
func (s *Service) SaveSettings() error {
	s.lastActiveMu.RLock()
	lastActive := make(map[string]int64)
	for k, v := range s.lastActive {
		lastActive[k] = v
	}
	s.lastActiveMu.RUnlock()

	s.pinnedMu.RLock()
	pinnedSerial := s.pinnedSerial
	s.pinnedMu.RUnlock()

	settings := Settings{
		LastActive:   lastActive,
		PinnedSerial: pinnedSerial,
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.settingsPath, data, 0644)
}

func (s *Service) loadSettings() {
	if s.settingsPath == "" {
		return
	}
	data, err := os.ReadFile(s.settingsPath)
	if err != nil {
		return
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}

	s.lastActiveMu.Lock()
	if settings.LastActive != nil {
		s.lastActive = settings.LastActive
	}
	s.lastActiveMu.Unlock()

	s.pinnedMu.Lock()
	s.pinnedSerial = settings.PinnedSerial
	s.pinnedMu.Unlock()
}

// ========================================
// Path Accessors
// ========================================

// ConfigDir returns the configuration directory path
func (s *Service) ConfigDir() string {
	return s.configDir
}

// HistoryPath returns the history file path
func (s *Service) HistoryPath() string {
	return s.historyPath
}

// ========================================
// Shutdown
// ========================================

// Close saves all caches and settings before shutdown
func (s *Service) Close() error {
	if err := s.SaveCache(); err != nil {
		s.log("Error saving cache on close: %v", err)
	}
	if err := s.SaveSettings(); err != nil {
		s.log("Error saving settings on close: %v", err)
	}
	return nil
}
