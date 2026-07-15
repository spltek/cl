package editor

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/silvio/cl/internal/config"
)

func withTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("CL_CONFIG_DIR", t.TempDir())
}

func TestResolve_PrefersEnvVar(t *testing.T) {
	withTempConfigDir(t)
	t.Setenv("EDITOR", "code --wait")

	cfg := &config.Config{Editor: "nano"} // should be ignored
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save() error = %v", err)
	}

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got != "code --wait" {
		t.Fatalf("Resolve() = %q, want %q", got, "code --wait")
	}
}

func TestResolve_FallsBackToSavedPreference(t *testing.T) {
	withTempConfigDir(t)
	t.Setenv("EDITOR", "")

	cfg := &config.Config{Editor: "nano"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save() error = %v", err)
	}

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got != "nano" {
		t.Fatalf("Resolve() = %q, want %q", got, "nano")
	}
}

func TestResolve_PromptsAndPersistsWhenUnset(t *testing.T) {
	withTempConfigDir(t)
	t.Setenv("EDITOR", "")

	restoreStdin := fakeStdin(t, "my-editor\n")
	defer restoreStdin()

	got, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got != "my-editor" {
		t.Fatalf("Resolve() = %q, want %q", got, "my-editor")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	if cfg.Editor != "my-editor" {
		t.Fatalf("persisted editor = %q, want %q", cfg.Editor, "my-editor")
	}
}

func TestEditValue_WritesAndReadsBackContent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake editor script relies on a POSIX shell")
	}

	withTempConfigDir(t)

	script := writeFakeEditorScript(t, "echo appended-command")
	t.Setenv("EDITOR", script)

	got, err := EditValue("")
	if err != nil {
		t.Fatalf("EditValue() error = %v", err)
	}

	if got != "echo appended-command" {
		t.Fatalf("EditValue() = %q, want %q", got, "echo appended-command")
	}
}

// fakeStdin temporarily replaces os.Stdin with a pipe pre-loaded with
// content, and returns a function that restores the original stdin.
func fakeStdin(t *testing.T, content string) func() {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	if _, err := w.WriteString(content); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}
	w.Close()

	original := os.Stdin
	os.Stdin = r

	return func() {
		os.Stdin = original
		r.Close()
	}
}

// writeFakeEditorScript creates a small shell script that overwrites
// whatever file it's given ($1) with the given content, simulating a
// user typing a command into their editor and saving.
func writeFakeEditorScript(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := dir + "/fake-editor.sh"

	script := "#!/bin/sh\necho '" + strings.ReplaceAll(content, "'", "'\\''") + "' > \"$1\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake editor script: %v", err)
	}

	return path
}
