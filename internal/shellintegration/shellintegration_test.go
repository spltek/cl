package shellintegration

import (
	"os/exec"
	"strings"
	"testing"
)

func TestScript_UnsupportedShell(t *testing.T) {
	if _, err := Script("fish"); err == nil {
		t.Fatalf("Script(fish) error = nil, want error")
	}
}

func TestScript_Zsh(t *testing.T) {
	out, err := Script("zsh")
	if err != nil {
		t.Fatalf("Script(zsh) error = %v", err)
	}

	if !strings.Contains(out, "print -z") {
		t.Fatalf("Script(zsh) does not mention print -z:\n%s", out)
	}

	checkSyntax(t, "zsh", "-n", out)
}

func TestScript_Bash(t *testing.T) {
	out, err := Script("bash")
	if err != nil {
		t.Fatalf("Script(bash) error = %v", err)
	}

	if !strings.Contains(out, "read -e -i") {
		t.Fatalf("Script(bash) does not mention read -e -i:\n%s", out)
	}

	checkSyntax(t, "bash", "-n", out)
}

func TestScript_PowerShell(t *testing.T) {
	for _, name := range []string{"powershell", "pwsh"} {
		out, err := Script(name)
		if err != nil {
			t.Fatalf("Script(%s) error = %v", name, err)
		}

		if !strings.Contains(out, "PSConsoleReadLine") {
			t.Fatalf("Script(%s) does not mention PSConsoleReadLine:\n%s", name, out)
		}
	}
}

// infoFlags are every informational flag/subcommand cl's own arg
// parser (cmd/cl/main.go) handles directly, as opposed to treating
// its argument as a fuzzy-filter for the interactive picker (adding,
// editing, renaming and deleting commands all happen inside the
// picker itself, via ctrl+a/ctrl+e/ctrl+r/ctrl+d - there is no
// longer a separate CLI flag for them). Each shell wrapper must pass
// all of these straight through to the real binary instead of
// capturing its output via command substitution - otherwise -v/-h
// output would get pushed into the prompt buffer instead of printed
// normally.
var infoFlags = []string{"init", "-v", "--version", "-h", "--help", "help"}

func TestScript_ZshPassesThroughInfoFlags(t *testing.T) {
	out, err := Script("zsh")
	if err != nil {
		t.Fatalf("Script(zsh) error = %v", err)
	}

	want := strings.Join(infoFlags, "|")
	if !strings.Contains(out, want) {
		t.Fatalf("Script(zsh) case pattern does not list all pass-through flags (want %q):\n%s", want, out)
	}
}

func TestScript_BashPassesThroughInfoFlags(t *testing.T) {
	out, err := Script("bash")
	if err != nil {
		t.Fatalf("Script(bash) error = %v", err)
	}

	want := strings.Join(infoFlags, "|")
	if !strings.Contains(out, want) {
		t.Fatalf("Script(bash) case pattern does not list all pass-through flags (want %q):\n%s", want, out)
	}
}

func TestScript_PowerShellPassesThroughInfoFlags(t *testing.T) {
	out, err := Script("powershell")
	if err != nil {
		t.Fatalf("Script(powershell) error = %v", err)
	}

	for _, flag := range infoFlags {
		quoted := "'" + flag + "'"
		if !strings.Contains(out, quoted) {
			t.Fatalf("Script(powershell) $passthrough array does not list flag %s:\n%s", quoted, out)
		}
	}
}

// checkSyntax runs the interpreter's own syntax checker over the
// generated script when that interpreter is available on PATH,
// skipping otherwise (e.g. bash may be missing on some CI images).
func checkSyntax(t *testing.T, interpreter string, checkFlag string, script string) {
	t.Helper()

	if _, err := exec.LookPath(interpreter); err != nil {
		t.Skipf("%s not available on PATH, skipping syntax check", interpreter)
	}

	cmd := exec.Command(interpreter, checkFlag)
	cmd.Stdin = strings.NewReader(script)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", interpreter, checkFlag, err, out)
	}
}
