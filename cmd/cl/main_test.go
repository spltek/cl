package main

import (
	"testing"
)

func withTempConfigDir(t *testing.T) {
	t.Helper()
	t.Setenv("CL_CONFIG_DIR", t.TempDir())
}

func TestRun_UsageErrorsOnMissingArgs(t *testing.T) {
	if err := run([]string{"init"}); err == nil {
		t.Errorf("run(init) with no shell arg error = nil, want usage error")
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

func TestRun_VersionFlagsSucceed(t *testing.T) {
	for _, args := range [][]string{{"-v"}, {"--version"}} {
		if err := run(args); err != nil {
			t.Errorf("run(%v) error = %v, want nil", args, err)
		}
	}
}

func TestRun_HelpFlagsSucceed(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		if err := run(args); err != nil {
			t.Errorf("run(%v) error = %v, want nil", args, err)
		}
	}
}

// TestRun_InteractiveModeRequiresATTY documents that the bare/filter
// invocation (the only way left to add/edit/remove commands, via the
// picker's ctrl+a/ctrl+e/ctrl+r) needs a real controlling terminal:
// running it here, with no TTY attached, must fail cleanly instead
// of hanging or panicking. The interactive picker's own logic (add,
// edit, remove, navigation, filtering) is covered thoroughly by
// internal/tui's tests, which drive the bubbletea model directly
// without a TTY.
func TestRun_InteractiveModeRequiresATTY(t *testing.T) {
	withTempConfigDir(t)

	if err := run([]string{"anything"}); err == nil {
		t.Fatalf("run(anything) without a controlling terminal error = nil, want error")
	}
}
