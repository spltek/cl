package config

import "testing"

func withTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("CL_CONFIG_DIR", t.TempDir())
}

func TestLoad_MissingFileReturnsEmptyConfig(t *testing.T) {
	withTempConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Editor != "" {
		t.Fatalf("Editor = %q, want empty", cfg.Editor)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	withTempConfigDir(t)

	cfg := &Config{Editor: "nano"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if reloaded.Editor != "nano" {
		t.Fatalf("Editor after reload = %q, want %q", reloaded.Editor, "nano")
	}
}
