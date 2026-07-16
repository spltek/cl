package tui

import (
	"strings"
	"testing"
)

func TestParsePlaceholders_NoPlaceholders(t *testing.T) {
	phs := parsePlaceholders("npm run build")
	if len(phs) != 0 {
		t.Fatalf("parsePlaceholders(%q) len = %d, want 0", "npm run build", len(phs))
	}
}

func TestParsePlaceholders_Simple(t *testing.T) {
	phs := parsePlaceholders("ssh {{user}}@{{host}}")
	if len(phs) != 2 {
		t.Fatalf("len = %d, want 2", len(phs))
	}
	if phs[0].Name != "user" || phs[0].Default != "" {
		t.Fatalf("[0] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[0].Name, phs[0].Default, "user", "")
	}
	if phs[1].Name != "host" || phs[1].Default != "" {
		t.Fatalf("[1] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[1].Name, phs[1].Default, "host", "")
	}
}

func TestParsePlaceholders_WithDefault(t *testing.T) {
	phs := parsePlaceholders("git push {{remote:origin}} {{branch:main}}")
	if len(phs) != 2 {
		t.Fatalf("len = %d, want 2", len(phs))
	}
	if phs[0].Name != "remote" || phs[0].Default != "origin" {
		t.Fatalf("[0] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[0].Name, phs[0].Default, "remote", "origin")
	}
	if phs[1].Name != "branch" || phs[1].Default != "main" {
		t.Fatalf("[1] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[1].Name, phs[1].Default, "branch", "main")
	}
}

func TestParsePlaceholders_DefaultCanContainNumbersAndDots(t *testing.T) {
	phs := parsePlaceholders("ssh {{user}}@prod-{{num:1}}.example.com")
	if len(phs) != 2 {
		t.Fatalf("len = %d, want 2", len(phs))
	}
	if phs[0].Name != "user" {
		t.Fatalf("[0].Name = %q, want %q", phs[0].Name, "user")
	}
	if phs[1].Name != "num" || phs[1].Default != "1" {
		t.Fatalf("[1] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[1].Name, phs[1].Default, "num", "1")
	}
}

func TestParsePlaceholders_DefaultCanContainSpaces(t *testing.T) {
	phs := parsePlaceholders("echo {{msg:hello world}}")
	if len(phs) != 1 {
		t.Fatalf("len = %d, want 1", len(phs))
	}
	if phs[0].Name != "msg" || phs[0].Default != "hello world" {
		t.Fatalf("[0] = {Name:%q Default:%q}, want {Name:%q Default:%q}", phs[0].Name, phs[0].Default, "msg", "hello world")
	}
}

func TestParsePlaceholders_StartEndPositionsAreCorrect(t *testing.T) {
	cmd := "ssh {{user}}@{{host}}"
	phs := parsePlaceholders(cmd)

	if got := cmd[phs[0].Start:phs[0].End]; got != "{{user}}" {
		t.Fatalf("first placeholder text = %q, want %q", got, "{{user}}")
	}
	if got := cmd[phs[1].Start:phs[1].End]; got != "{{host}}" {
		t.Fatalf("second placeholder text = %q, want %q", got, "{{host}}")
	}
}

func TestResolveCommand_AllFilled(t *testing.T) {
	cmd := "ssh {{user}}@{{host}}"
	phs := parsePlaceholders(cmd)
	got := resolveCommand(cmd, phs, []string{"admin", "prod.example.com"})
	want := "ssh admin@prod.example.com"
	if got != want {
		t.Fatalf("resolveCommand = %q, want %q", got, want)
	}
}

func TestResolveCommand_UsesDefaultWhenValueEmpty(t *testing.T) {
	cmd := "git push {{remote:origin}} {{branch:main}}"
	phs := parsePlaceholders(cmd)
	got := resolveCommand(cmd, phs, []string{"", ""})
	want := "git push origin main"
	if got != want {
		t.Fatalf("resolveCommand = %q, want %q", got, want)
	}
}

func TestResolveCommand_MixedFilledAndDefaults(t *testing.T) {
	cmd := "git push {{remote:origin}} {{branch}}"
	phs := parsePlaceholders(cmd)
	got := resolveCommand(cmd, phs, []string{"", "feature-x"})
	want := "git push origin feature-x"
	if got != want {
		t.Fatalf("resolveCommand = %q, want %q", got, want)
	}
}

func TestResolveCommand_EmptyPlaceholderWithoutDefaultUsesEmptyString(t *testing.T) {
	cmd := "echo {{msg}}"
	phs := parsePlaceholders(cmd)
	got := resolveCommand(cmd, phs, []string{""})
	want := "echo "
	if got != want {
		t.Fatalf("resolveCommand = %q, want %q", got, want)
	}
}

func TestBuildPreview_AllUnfilled(t *testing.T) {
	cmd := "ssh {{user}}@{{host}}"
	phs := parsePlaceholders(cmd)
	got := buildPreview(cmd, phs, nil, 0, "")
	if !strings.Contains(got, "{{user}}") || !strings.Contains(got, "{{host}}") {
		t.Fatalf("buildPreview (all unfilled) = %q, want it to still contain placeholders", got)
	}
}

func TestBuildPreview_CurrentTypedTextShown(t *testing.T) {
	cmd := "ssh {{user}}@{{host}}"
	phs := parsePlaceholders(cmd)
	// user index 0: show typed text; host index 1: leave untouched
	got := buildPreview(cmd, phs, nil, 0, "admin")
	if !strings.Contains(got, "admin") {
		t.Fatalf("buildPreview with current text = %q, want it to contain %q", got, "admin")
	}
	if strings.Contains(got, "{{user}}") {
		t.Fatalf("buildPreview = %q, want the current placeholder to be replaced", got)
	}
	if !strings.Contains(got, "{{host}}") {
		t.Fatalf("buildPreview = %q, want remaining placeholder to stay", got)
	}
}

func TestBuildPreview_PreviouslyFilledValuesShown(t *testing.T) {
	cmd := "ssh {{user}}@{{host}}"
	phs := parsePlaceholders(cmd)
	// user (idx 0) filled with "admin", host (idx 1) is current
	got := buildPreview(cmd, phs, []string{"admin"}, 1, "prod")
	if !strings.Contains(got, "admin") {
		t.Fatalf("buildPreview = %q, want previously-filled %q to appear", got, "admin")
	}
	if strings.Contains(got, "{{user}}") {
		t.Fatalf("buildPreview = %q, want the already-filled placeholder gone", got)
	}
	if !strings.Contains(got, "prod") {
		t.Fatalf("buildPreview = %q, want current text %q to appear", got, "prod")
	}
	if strings.Contains(got, "{{host}}") {
		t.Fatalf("buildPreview = %q, want the current placeholder replaced by typed text", got)
	}
}

func TestBuildPreview_LaterPlaceholdersKeepDefaults(t *testing.T) {
	cmd := "git push {{remote:origin}} {{branch:main}}"
	phs := parsePlaceholders(cmd)
	got := buildPreview(cmd, phs, nil, 0, "upstream")
	if !strings.Contains(got, "upstream") {
		t.Fatalf("buildPreview = %q, want current text %q", got, "upstream")
	}
	if !strings.Contains(got, "{{branch:main}}") {
		t.Fatalf("buildPreview = %q, want later placeholder with default left untouched", got)
	}
}

func TestBuildPreview_CurrentTextWinsOverPrefillInValues(t *testing.T) {
	// Mirrors startFillPlaceholders: values[i] is already the default
	// before the user finishes editing. Typed text must still appear
	// live in the preview.
	cmd := "git push {{remote:origin}} {{branch:main}}"
	phs := parsePlaceholders(cmd)
	got := buildPreview(cmd, phs, []string{"origin", "main"}, 0, "upstream")
	if !strings.Contains(got, "upstream") {
		t.Fatalf("buildPreview = %q, want live typed text %q", got, "upstream")
	}
	if strings.Contains(got, "origin") {
		t.Fatalf("buildPreview = %q, want the stale prefilled default not shown for the current field", got)
	}
	if !strings.Contains(got, "{{branch:main}}") {
		t.Fatalf("buildPreview = %q, want later placeholder left as a template", got)
	}
}

func TestBuildPreview_PastEmptyValueFallsBackToDefault(t *testing.T) {
	cmd := "git push {{remote:origin}} {{branch:main}}"
	phs := parsePlaceholders(cmd)
	// User cleared remote and accepted empty (allowed because a default exists).
	got := buildPreview(cmd, phs, []string{""}, 1, "feature")
	if !strings.Contains(got, "origin") {
		t.Fatalf("buildPreview = %q, want past empty value resolved to default %q", got, "origin")
	}
	if strings.Contains(got, "{{remote") {
		t.Fatalf("buildPreview = %q, want past placeholder resolved, not left as a template", got)
	}
	if !strings.Contains(got, "feature") {
		t.Fatalf("buildPreview = %q, want current text %q", got, "feature")
	}
}

func TestParsePlaceholders_EmptyString(t *testing.T) {
	phs := parsePlaceholders("")
	if len(phs) != 0 {
		t.Fatalf("parsePlaceholders(\"\") len = %d, want 0", len(phs))
	}
}

func TestParsePlaceholders_IncompleteBraces(t *testing.T) {
	phs := parsePlaceholders("echo {{unclosed")
	if len(phs) != 0 {
		t.Fatalf("parsePlaceholders with unclosed braces len = %d, want 0", len(phs))
	}
}

func TestParsePlaceholders_EmptyNameNotMatched(t *testing.T) {
	phs := parsePlaceholders("echo {{}}")
	// {{}} has an empty name, \w+ requires at least one character.
	if len(phs) != 0 {
		t.Fatalf("parsePlaceholders with empty name len = %d, want 0", len(phs))
	}
}

func TestParsePlaceholders_ConsecutivePlaceholders(t *testing.T) {
	cmd := "{{a}}{{b}}"
	phs := parsePlaceholders(cmd)
	if len(phs) != 2 {
		t.Fatalf("len = %d, want 2", len(phs))
	}
	if phs[0].Name != "a" || phs[1].Name != "b" {
		t.Fatalf("names = %q, %q, want %q, %q", phs[0].Name, phs[1].Name, "a", "b")
	}
}

func TestResolveCommand_CommandWithNoPlaceholdersIsIdentity(t *testing.T) {
	cmd := "npm run build"
	got := resolveCommand(cmd, nil, nil)
	if got != cmd {
		t.Fatalf("resolveCommand = %q, want %q", got, cmd)
	}
}

func TestResolveCommand_DefaultWithPathSeparators(t *testing.T) {
	cmd := "cp {{src:/path/to/source}} {{dst:/path/to/dest}}"
	phs := parsePlaceholders(cmd)
	got := resolveCommand(cmd, phs, []string{"", ""})
	want := "cp /path/to/source /path/to/dest"
	if got != want {
		t.Fatalf("resolveCommand = %q, want %q", got, want)
	}
}

func TestBuildParamHint_Empty(t *testing.T) {
	if got := buildParamHint(nil); got != "" {
		t.Fatalf("buildParamHint(nil) = %q, want empty", got)
	}
}

func TestBuildParamHint_WithoutDefaults(t *testing.T) {
	phs := parsePlaceholders("ssh {{user}}@{{host}}")
	got := buildParamHint(phs)
	want := "[user, host]"
	if got != want {
		t.Fatalf("buildParamHint = %q, want %q", got, want)
	}
}

func TestBuildParamHint_WithDefaultsUsesDefaultPrefix(t *testing.T) {
	phs := parsePlaceholders("echo {{name:pippo}} {{count:10}}")
	got := buildParamHint(phs)
	want := "[name(default:pippo), count(default:10)]"
	if got != want {
		t.Fatalf("buildParamHint = %q, want %q", got, want)
	}
}
