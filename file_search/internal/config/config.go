package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all user-defined settings that persist across server restarts.
type Config struct {
	IndexedDirs  []string `json:"indexed_dirs"`
	ModeOverride string   `json:"mode_override,omitempty"`
}

// Manager handles loading and saving the config file atomically.
type Manager struct {
	mu   sync.Mutex
	path string
	cfg  Config
}

// Load reads the config file from disk. If path is empty or file doesn't exist, returns empty config.
func Load(path string) (*Manager, error) {
	m := &Manager{path: path}
	if path == "" {
		return m, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m.cfg); err != nil {
		return nil, err
	}
	return m, nil
}

// Get returns a snapshot of the current config.
func (m *Manager) Get() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return copy to avoid data races on slices.
	dirs := make([]string, len(m.cfg.IndexedDirs))
	copy(dirs, m.cfg.IndexedDirs)
	return Config{IndexedDirs: dirs, ModeOverride: m.cfg.ModeOverride}
}

// AddDir adds a directory if not already present and saves to disk.
func (m *Manager) AddDir(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.cfg.IndexedDirs {
		if d == dir {
			return nil
		}
	}
	m.cfg.IndexedDirs = append(m.cfg.IndexedDirs, dir)
	return m.save()
}

// SetModeOverride sets the mode override and saves to disk.
func (m *Manager) SetModeOverride(mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.ModeOverride = mode
	return m.save()
}

// SetDirs replaces the full list of directories and saves.
func (m *Manager) SetDirs(dirs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg.IndexedDirs = dirs
	return m.save()
}

// save writes config atomically (write to .tmp, then rename).
func (m *Manager) save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
