// Package config handles persistence of cl's own preferences
// (currently just the preferred editor command) as JSON.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silvio/cl/internal/store"
)

// Config holds cl's user preferences.
type Config struct {
	Editor string `json:"editor,omitempty"`
}

func configPath() (string, error) {
	dir, err := store.ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file from disk. If it does not exist yet, it
// returns an empty Config without error.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}

	return cfg, nil
}

// Save writes the config to disk atomically.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file to %q: %w", path, err)
	}

	return nil
}
