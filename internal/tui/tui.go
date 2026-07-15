// Package tui implements the interactive fuzzy-filtered command picker.
// It renders on the controlling terminal (not on stdout) so that the
// final selected command can be captured cleanly via command
// substitution by the calling shell integration.
package tui

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/silvio/cl/internal/store"
)

const maxVisibleRows = 10

// styles holds the lipgloss styles used by the picker. They must be
// built from a renderer bound to the actual terminal we draw on
// (see openTTY), not from lipgloss's default renderer: that default
// detects color support from os.Stdout, which the shell integration
// deliberately redirects via command substitution to capture the
// final selection - checking os.Stdout there would always report "no
// color", even though the picker itself renders on a real, colorful
// terminal.
type styles struct {
	selected lipgloss.Style
	command  lipgloss.Style
	help     lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	return styles{
		selected: r.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		command:  r.NewStyle().Foreground(lipgloss.Color("244")),
		help:     r.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// nameSource adapts a slice of entry names to fuzzy.Source.
type nameSource []string

func (s nameSource) String(i int) string { return s[i] }
func (s nameSource) Len() int            { return len(s) }

type model struct {
	input    textinput.Model
	all      []store.Entry
	filtered []store.Entry
	cursor   int
	scroll   int
	selected string
	quitting bool
	styles   styles
}

func newModel(initialFilter string, entries []store.Entry, st styles) model {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Prompt = "cl> "
	ti.SetValue(initialFilter)
	ti.Focus()
	ti.CursorEnd()

	m := model{input: ti, all: entries, styles: st}
	m.refilter()

	return m
}

func (m *model) refilter() {
	query := m.input.Value()

	if query == "" {
		m.filtered = m.all
	} else {
		names := make(nameSource, len(m.all))
		for i, e := range m.all {
			names[i] = e.Name
		}

		matches := fuzzy.FindFrom(query, names)
		filtered := make([]store.Entry, 0, len(matches))
		for _, match := range matches {
			filtered = append(filtered, m.all[match.Index])
		}
		m.filtered = filtered
	}

	m.cursor = 0
	m.scroll = 0
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor].Command
			}
			m.quitting = true
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scroll {
					m.scroll = m.cursor
				}
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.scroll+maxVisibleRows {
					m.scroll = m.cursor - maxVisibleRows + 1
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.input.View())
	b.WriteString("\n")

	end := m.scroll + maxVisibleRows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.scroll; i < end; i++ {
		entry := m.filtered[i]
		line := fmt.Sprintf("%s  %s", entry.Name, m.styles.command.Render(entry.Command))
		if i == m.cursor {
			line = m.styles.selected.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(m.styles.help.Render("  no matching commands"))
		b.WriteString("\n")
	}

	b.WriteString(m.styles.help.Render("↑/↓ move · enter select · esc cancel"))

	return b.String()
}

// Run displays the interactive picker seeded with initialFilter and
// returns the shell command chosen by the user, or "" if the user
// cancelled the selection.
func Run(initialFilter string, entries []store.Entry) (string, error) {
	tty, cleanup, err := openTTY()
	if err != nil {
		return "", fmt.Errorf("open controlling terminal: %w", err)
	}
	defer cleanup()

	renderer := lipgloss.NewRenderer(tty)
	m := newModel(initialFilter, entries, newStyles(renderer))

	p := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(tty))

	final, err := p.Run()
	if err != nil {
		return "", err
	}

	fm, ok := final.(model)
	if !ok {
		return "", fmt.Errorf("unexpected model type")
	}

	return fm.selected, nil
}

// openTTY opens the controlling terminal for reading and writing,
// independently of how stdin/stdout are currently redirected. This
// lets the UI render on-screen even while stdout is being captured
// via command substitution by the shell integration.
func openTTY() (*os.File, func(), error) {
	name := "/dev/tty"
	if runtime.GOOS == "windows" {
		name = "CONOUT$"
	}

	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}

	return f, func() { f.Close() }, nil
}

var _ io.ReadWriter = (*os.File)(nil)
