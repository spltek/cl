package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds cl's own persisted settings - as opposed to Store,
// which holds the command dictionary itself.
type Config struct {
	path string
	data configData
}

// configData is the on-disk shape of config.json. Its zero value
// (ShowCommand: false, MaxVisibleRows: 0) is also Config's default,
// so a missing file behaves exactly like one with every setting left
// at its default.
type configData struct {
	// ShowCommand controls whether the picker's list shows each
	// entry's command next to its name. When true, the command is
	// shown in the list. When false (the default), the list only
	// shows names. Enter always runs the command directly
	// regardless of this setting.
	ShowCommand bool `json:"showCommand"`

	// MaxVisibleRows is the maximum number of entries the picker
	// shows inside the list box before scrolling. It must be
	// between 1 and 1000. A zero value means "use the default"
	// (10).
	MaxVisibleRows int `json:"maxVisibleRows"`
}

// configPath returns the full path to the settings JSON file.
func configPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig reads cl's settings from disk. If the file does not
// exist yet, it returns a Config with every setting at its default.
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	c := &Config{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	if len(data) == 0 {
		return c, nil
	}

	if err := json.Unmarshal(data, &c.data); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}

	return c, nil
}

// Save writes the current settings to disk atomically.
func (c *Config) Save() error {
	return writeJSONAtomic(c.path, c.data)
}

// ShowCommand reports whether the picker should show each entry's
// command next to its name. Enter always runs the command directly
// regardless of this setting. See configData.ShowCommand.
func (c *Config) ShowCommand() bool {
	return c.data.ShowCommand
}

// SetShowCommand updates the ShowCommand setting in memory; call
// Save to persist it.
func (c *Config) SetShowCommand(v bool) {
	c.data.ShowCommand = v
}

// MaxVisibleRows returns the maximum number of entries the picker
// shows inside the list box. The default (when the setting has never
// been changed) is 10.
func (c *Config) MaxVisibleRows() int {
	if c.data.MaxVisibleRows <= 0 {
		return 10
	}
	return c.data.MaxVisibleRows
}

// SetMaxVisibleRows updates the MaxVisibleRows setting in memory;
// call Save to persist. v is clamped to [1, 1000].
func (c *Config) SetMaxVisibleRows(v int) {
	if v < 1 {
		v = 1
	}
	if v > 1000 {
		v = 1000
	}
	c.data.MaxVisibleRows = v
}
