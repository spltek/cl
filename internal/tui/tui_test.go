package tui

import (
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/silvio/cl/internal/store"
)

func testEntries() []store.Entry {
	return []store.Entry{
		{Name: "backup", Command: "rsync -av ./src ./backup"},
		{Name: "build", Command: "npm run build"},
		{Name: "cleanup", Command: "rm -rf dist"},
	}
}

// testStyles returns a styles value suitable for tests, which don't
// care about actual color output.
func testStyles() styles {
	return newStyles(lipgloss.NewRenderer(io.Discard))
}

func TestNewModel_NoFilterShowsAllEntries(t *testing.T) {
	m := newModel("", testEntries(), testStyles())

	if len(m.filtered) != 3 {
		t.Fatalf("filtered len = %d, want 3", len(m.filtered))
	}
}

func TestNewModel_FilterNarrowsByName(t *testing.T) {
	m := newModel("bui", testEntries(), testStyles())

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered = %+v, want just %q", m.filtered, "build")
	}
}

func TestUpdate_ArrowKeysMoveCursorWithinBounds(t *testing.T) {
	m := newModel("", testEntries(), testStyles())

	// Moving up from the first row should have no effect.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(model)
	if m.cursor != 0 {
		t.Fatalf("cursor after Up at top = %d, want 0", m.cursor)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	if m.cursor != 1 {
		t.Fatalf("cursor after Down = %d, want 1", m.cursor)
	}

	// Moving down past the last row should clamp at len-1.
	for i := 0; i < 10; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(model)
	}
	if m.cursor != len(m.filtered)-1 {
		t.Fatalf("cursor after many Down = %d, want %d", m.cursor, len(m.filtered)-1)
	}
}

func TestUpdate_EnterSelectsHighlightedCommand(t *testing.T) {
	m := newModel("clean", testEntries(), testStyles())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if m.selected != "rm -rf dist" {
		t.Fatalf("selected = %q, want %q", m.selected, "rm -rf dist")
	}
	if !m.quitting {
		t.Fatalf("quitting = false, want true after Enter")
	}
	if cmd == nil {
		t.Fatalf("expected a tea.Quit command, got nil")
	}
}

func TestUpdate_EscCancelsWithoutSelection(t *testing.T) {
	m := newModel("build", testEntries(), testStyles())

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)

	if m.selected != "" {
		t.Fatalf("selected = %q, want empty after Esc", m.selected)
	}
	if !m.quitting {
		t.Fatalf("quitting = false, want true after Esc")
	}
	if cmd == nil {
		t.Fatalf("expected a tea.Quit command, got nil")
	}
}

func TestUpdate_TypingNarrowsFilterAndResetsCursor(t *testing.T) {
	m := newModel("", testEntries(), testStyles())

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	if m.cursor != 1 {
		t.Fatalf("cursor before typing = %d, want 1", m.cursor)
	}

	for _, r := range "build" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered after typing 'build' = %+v", m.filtered)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor after typing = %d, want reset to 0", m.cursor)
	}
}
