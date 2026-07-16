package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_MissingFileDefaultsShowCommandToFalse(t *testing.T) {
	withTempConfigDir(t)

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if c.ShowCommand() {
		t.Fatalf("ShowCommand() = true, want false by default")
	}
}

func TestLoadConfig_MissingFileDefaultsMaxVisibleRowsTo20(t *testing.T) {
	withTempConfigDir(t)

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if got, want := c.MaxVisibleRows(), 20; got != want {
		t.Fatalf("MaxVisibleRows() = %d, want %d by default", got, want)
	}
}

func TestConfig_SaveAndLoadRoundTrip(t *testing.T) {
	dir := withTempConfigDir(t)

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	c.SetShowCommand(true)
	if err := c.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("expected config.json to exist: %v", err)
	}

	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() after Save() error = %v", err)
	}
	if !reloaded.ShowCommand() {
		t.Fatalf("ShowCommand() after reload = false, want true")
	}
}
