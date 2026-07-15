package main

import (
	"runtime"
	"testing"
	"time"
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
// invocation (the only way left to add/edit/rename/delete commands,
// via the picker's ctrl+a/ctrl+e/ctrl+r/ctrl+d) needs a real
// controlling terminal: running it here, with no TTY attached, must
// fail cleanly instead of hanging or panicking. The interactive
// picker's own logic (add, edit, rename, delete, navigation,
// filtering) is covered thoroughly by internal/tui's tests, which
// drive the bubbletea model directly without a TTY.
func TestRun_InteractiveModeRequiresATTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Unlike POSIX's /dev/tty, which fails with ENXIO/ENOTTY when
		// the calling process has no controlling terminal, Windows'
		// CONIN$/CONOUT$ device files succeed as long as any console
		// is attached to the process - which GitHub's Windows runners
		// provide even for a headless `go test` invocation. So
		// tea.OpenTTY() succeeds here and then blocks forever reading
		// console input that never arrives, instead of failing
		// cleanly the way it does on Unix. There is no reliable way
		// to simulate "no controlling terminal" on Windows from
		// within a test, so this scenario is only verified on the
		// Linux/macOS CI runners.
		t.Skip("Windows always provides a console to the test process, so a missing-TTY error can't be reproduced here")
	}

	withTempConfigDir(t)

	// Guarded by an explicit deadline (rather than relying solely on
	// go test's own 10-minute-per-package alarm) so a regression that
	// reintroduces a hang fails this test fast instead of burning an
	// entire CI job.
	done := make(chan error, 1)
	go func() { done <- run([]string{"anything"}) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("run(anything) without a controlling terminal error = nil, want error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run(anything) without a controlling terminal blocked instead of failing")
	}
}
