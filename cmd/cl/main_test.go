package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func withTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("CL_CONFIG_DIR", t.TempDir())
}

func fakeStdin(t *testing.T, content string) {
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

	t.Cleanup(func() {
		os.Stdin = original
		r.Close()
	})
}

func writeFakeEditorScript(t *testing.T, content string) string {
	t.Helper()

	path := t.TempDir() + "/fake-editor.sh"
	script := "#!/bin/sh\necho '" + strings.ReplaceAll(content, "'", "'\\''") + "' > \"$1\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake editor script: %v", err)
	}

	return path
}

func TestRun_UsageErrorsOnMissingArgs(t *testing.T) {
	cases := [][]string{
		{"-add"},
		{"-remove"},
		{"init"},
	}

	for _, args := range cases {
		if err := run(args); err == nil {
			t.Errorf("run(%v) error = nil, want usage error", args)
		}
	}
}

func TestRun_InitPrintsScriptForKnownShell(t *testing.T) {
	if err := run([]string{"init", "zsh"}); err != nil {
		t.Fatalf("run(init zsh) error = %v", err)
	}
}

func TestRun_InitFailsForUnknownShell(t *testing.T) {
	if err := run([]string{"init", "fish"}); err == nil {
		t.Fatalf("run(init fish) error = nil, want error")
	}
}

func TestRun_AddThenRemoveRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake editor script relies on a POSIX shell")
	}

	withTempConfigDir(t)
	t.Setenv("EDITOR", writeFakeEditorScript(t, "echo hello-from-test"))

	if err := run([]string{"-add", "greet"}); err != nil {
		t.Fatalf("run(-add greet) error = %v", err)
	}

	if err := run([]string{"-remove", "greet"}); err != nil {
		t.Fatalf("run(-remove greet) error = %v", err)
	}

	if err := run([]string{"-remove", "greet"}); err == nil {
		t.Fatalf("run(-remove greet) again error = nil, want not-found error")
	}
}

func TestRun_AddExistingAsksForConfirmation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake editor script relies on a POSIX shell")
	}

	withTempConfigDir(t)
	t.Setenv("EDITOR", writeFakeEditorScript(t, "echo first-value"))

	if err := run([]string{"-add", "greet"}); err != nil {
		t.Fatalf("first run(-add greet) error = %v", err)
	}

	// Answering "n" should abort without changing the stored value.
	fakeStdin(t, "n\n")
	t.Setenv("EDITOR", writeFakeEditorScript(t, "echo second-value"))
	if err := run([]string{"-add", "greet"}); err != nil {
		t.Fatalf("second run(-add greet) error = %v", err)
	}
}

func TestConfirm_ParsesYesVariants(t *testing.T) {
	cases := map[string]bool{
		"y\n":   true,
		"Y\n":   true,
		"yes\n": true,
		"n\n":   false,
		"\n":    false,
		"":      false,
	}

	for input, want := range cases {
		fakeStdin(t, input)
		got, err := confirm("Overwrite?")
		if err != nil && input != "" {
			t.Errorf("confirm() with input %q error = %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("confirm() with input %q = %v, want %v", input, got, want)
		}
	}
}
