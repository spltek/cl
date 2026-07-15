package store

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	return dir
}

func TestLoad_MissingFileReturnsEmptyStore(t *testing.T) {
	withTempConfigDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(s.List()) != 0 {
		t.Fatalf("expected empty store, got %d entries", len(s.List()))
	}
}

func TestSetGetRemove(t *testing.T) {
	withTempConfigDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s.Set("greet", "echo hello")

	cmd, ok := s.Get("greet")
	if !ok || cmd != "echo hello" {
		t.Fatalf("Get(greet) = (%q, %v), want (%q, true)", cmd, ok, "echo hello")
	}

	if !s.Remove("greet") {
		t.Fatalf("Remove(greet) = false, want true")
	}

	if _, ok := s.Get("greet"); ok {
		t.Fatalf("Get(greet) after Remove: found, want not found")
	}

	if s.Remove("greet") {
		t.Fatalf("Remove(greet) again = true, want false")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := withTempConfigDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s.Set("build", "npm run build")
	s.Set("clean", "rm -rf dist")

	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// The commands file should now exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "commands.json")); err != nil {
		t.Fatalf("expected commands.json to exist: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	cmd, ok := reloaded.Get("build")
	if !ok || cmd != "npm run build" {
		t.Fatalf("Get(build) after reload = (%q, %v), want (%q, true)", cmd, ok, "npm run build")
	}
}

func TestList_SortedByName(t *testing.T) {
	withTempConfigDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	s.Set("zeta", "echo z")
	s.Set("alpha", "echo a")
	s.Set("mid", "echo m")

	entries := s.List()
	if len(entries) != 3 {
		t.Fatalf("List() len = %d, want 3", len(entries))
	}

	want := []string{"alpha", "mid", "zeta"}
	for i, e := range entries {
		if e.Name != want[i] {
			t.Fatalf("List()[%d].Name = %q, want %q", i, e.Name, want[i])
		}
	}
}
