// Package tui implements the interactive command picker and all
// in-picker management (add/edit/rename/delete). It renders on the
// controlling terminal (not on stdout) so that the final selected
// command can be captured cleanly via command substitution by the
// calling shell integration.
package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/silvio/cl/internal/store"
)

const maxVisibleRows = 10

// styles holds the lipgloss styles used by the picker's own chrome
// (list selection, command previews, help text, errors). Lip Gloss
// v2 styles carry no I/O state of their own - the terminal's color
// profile is detected and applied automatically by Bubble Tea when
// it writes to the program's output - so, unlike in v1, these don't
// need to be bound to any particular renderer.
type styles struct {
	selected lipgloss.Style
	command  lipgloss.Style
	help     lipgloss.Style
	helpKey  lipgloss.Style
	errMsg   lipgloss.Style
}

func newStyles() styles {
	return styles{
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		command:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		help:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		helpKey:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("246")),
		errMsg:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("204")),
	}
}

// newTextInput returns a focused textinput.Model configured to draw
// a real terminal cursor - a thin, blinking "|" bar, the shape most
// shells use by default - instead of bubbles' virtual, block-shaped
// one. Disabling the virtual cursor here means textinput.View()
// renders the character that would be underneath it as plain text;
// the real cursor itself is reported separately via Model.Cursor()
// and plugged into the top-level tea.View in viewList/viewForm.
func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetVirtualCursor(false)

	s := ti.Styles()
	s.Cursor.Shape = tea.CursorBar
	s.Cursor.Color = nil // inherit the terminal's own cursor color
	ti.SetStyles(s)

	ti.Focus()
	return ti
}

// nameSource adapts a slice of entry names to fuzzy.Source.
type nameSource []string

func (s nameSource) String(i int) string { return s[i] }
func (s nameSource) Len() int            { return len(s) }

// mode selects which screen/interaction the model is currently
// showing. modeList is the default fuzzy-filter picker; the others
// are the add/edit/rename/delete sub-flows entered via
// ctrl+a/ctrl+e/ctrl+r/ctrl+d.
type mode int

const (
	modeList mode = iota
	modeAddName
	modeAddValue
	modeEditValue
	modeRenameName
	modeConfirmSaveEdit
	modeConfirmRename
	modeConfirmDelete
)

type model struct {
	st *store.Store

	input    textinput.Model
	all      []store.Entry
	filtered []store.Entry
	cursor   int
	scroll   int
	selected string
	quitting bool
	styles   styles

	mode mode

	// form is the single-line input reused by every add/edit
	// sub-flow (name entry, then command entry, then editing an
	// existing command).
	form textinput.Model

	pendingName  string // name captured by modeAddName/modeRenameName, staged until modeAddValue/modeConfirmRename commits it
	pendingValue string // command captured by modeEditValue, staged until modeConfirmSaveEdit commits it
	pendingErr   string // inline validation/save error shown next to the active form
	target       store.Entry
}

func newModel(initialFilter string, st *store.Store, sty styles) model {
	ti := newTextInput("type to filter...")
	ti.Prompt = "cl> "
	ti.SetValue(initialFilter)
	ti.CursorEnd()

	m := model{st: st, input: ti, styles: sty}
	m.all = st.List()
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

// finishMutation reloads the in-memory entry list from the store
// after an add/edit/rename/delete, resets all sub-flow state, and
// returns to the list screen.
func (m *model) finishMutation() {
	m.mode = modeList
	m.pendingName = ""
	m.pendingValue = ""
	m.pendingErr = ""
	m.all = m.st.List()
	m.refilter()
}

// cancelForm discards whatever the user was typing in an add/edit
// sub-flow and returns to the list without changing the store.
func (m *model) cancelForm() {
	m.mode = modeList
	m.pendingName = ""
	m.pendingValue = ""
	m.pendingErr = ""
}

func (m *model) startAdd() {
	m.mode = modeAddName
	m.pendingErr = ""
	m.form = newTextInput("command name (spaces allowed)")
}

func (m *model) startEdit(e store.Entry) {
	m.mode = modeEditValue
	m.target = e
	m.pendingErr = ""
	m.form = newTextInput("command")
	m.form.SetValue(e.Command)
	m.form.CursorEnd()
}

func (m *model) startRename(e store.Entry) {
	m.mode = modeRenameName
	m.target = e
	m.pendingErr = ""
	m.form = newTextInput("command name (spaces allowed)")
	m.form.SetValue(e.Name)
	m.form.CursorEnd()
}

func (m *model) startDelete(e store.Entry) {
	m.mode = modeConfirmDelete
	m.target = e
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAddName, modeAddValue, modeEditValue, modeRenameName:
		return m.updateForm(msg)
	case modeConfirmSaveEdit, modeConfirmRename, modeConfirmDelete:
		return m.updateConfirm(msg)
	default:
		return m.updateList(msg)
	}
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
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

		case "ctrl+a":
			m.startAdd()
			return m, nil

		case "ctrl+e":
			if len(m.filtered) > 0 {
				m.startEdit(m.filtered[m.cursor])
			}
			return m, nil

		case "ctrl+r":
			if len(m.filtered) > 0 {
				m.startRename(m.filtered[m.cursor])
			}
			return m, nil

		case "ctrl+d":
			if len(m.filtered) > 0 {
				m.startDelete(m.filtered[m.cursor])
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()

	return m, cmd
}

// updateForm drives modeAddName, modeAddValue, modeEditValue and
// modeRenameName: a single-line input plus
// enter-to-continue/esc-to-cancel.
func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.cancelForm()
			return m, nil

		case "enter":
			return m.submitForm()
		}
	}

	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

// submitForm validates and advances the current add/edit sub-flow.
// Empty (after trimming) names/commands, and duplicate names, are
// rejected with an inline message instead of leaving the form.
func (m model) submitForm() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.form.Value())

	switch m.mode {
	case modeAddName:
		if value == "" {
			m.pendingErr = "name cannot be empty"
			return m, nil
		}
		if _, exists := m.st.Get(value); exists {
			m.pendingErr = fmt.Sprintf("%q is already used - choose another name", value)
			return m, nil
		}

		m.pendingName = value
		m.pendingErr = ""
		m.mode = modeAddValue
		m.form = newTextInput("shell command")
		return m, nil

	case modeAddValue:
		if value == "" {
			m.pendingErr = "command cannot be empty"
			return m, nil
		}

		m.st.Set(m.pendingName, value)
		if err := m.st.Save(); err != nil {
			m.pendingErr = fmt.Sprintf("save failed: %v", err)
			return m, nil
		}
		m.finishMutation()
		return m, nil

	case modeEditValue:
		if value == "" {
			m.pendingErr = "command cannot be empty"
			return m, nil
		}

		m.pendingValue = value
		m.mode = modeConfirmSaveEdit
		return m, nil

	case modeRenameName:
		if value == "" {
			m.pendingErr = "name cannot be empty"
			return m, nil
		}
		if value != m.target.Name {
			if _, exists := m.st.Get(value); exists {
				m.pendingErr = fmt.Sprintf("%q is already used - choose another name", value)
				return m, nil
			}
		}

		m.pendingName = value
		m.mode = modeConfirmRename
		return m, nil
	}

	return m, nil
}

// updateConfirm drives the yes/no confirmation screens for saving an
// edit, for renaming, and for deleting an entry.
func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "y", "Y":
		switch m.mode {
		case modeConfirmSaveEdit:
			m.st.Set(m.target.Name, m.pendingValue)
			if err := m.st.Save(); err != nil {
				m.pendingErr = fmt.Sprintf("save failed: %v", err)
			}
		case modeConfirmRename:
			m.st.Rename(m.target.Name, m.pendingName)
			if err := m.st.Save(); err != nil {
				m.pendingErr = fmt.Sprintf("save failed: %v", err)
			}
		case modeConfirmDelete:
			m.st.Remove(m.target.Name)
			if err := m.st.Save(); err != nil {
				m.pendingErr = fmt.Sprintf("save failed: %v", err)
			}
		}
		m.finishMutation()
		return m, nil

	case "n", "N", "esc":
		m.cancelForm()
		return m, nil
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	switch m.mode {
	case modeAddName:
		return m.viewForm("Add command - name:", "enter continue · esc cancel")
	case modeAddValue:
		return m.viewForm(fmt.Sprintf("Add command %q - shell command:", m.pendingName), "enter save · esc cancel")
	case modeEditValue:
		return m.viewForm(fmt.Sprintf("Edit %q:", m.target.Name), "enter continue · esc cancel")
	case modeRenameName:
		return m.viewForm(fmt.Sprintf("Rename %q:", m.target.Name), "enter continue · esc cancel")
	case modeConfirmSaveEdit:
		return tea.NewView(m.viewConfirm(fmt.Sprintf("Save %q -> %s ?", m.target.Name, m.pendingValue)))
	case modeConfirmRename:
		return tea.NewView(m.viewConfirm(fmt.Sprintf("Rename %q -> %q ?", m.target.Name, m.pendingName)))
	case modeConfirmDelete:
		return tea.NewView(m.viewConfirm(fmt.Sprintf("Delete %q (%s) ?", m.target.Name, m.target.Command)))
	default:
		return m.viewList()
	}
}

func (m model) viewList() tea.View {
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

	b.WriteString("\n")
	b.WriteString(m.listHelp())

	v := tea.NewView(b.String())
	v.Cursor = m.input.Cursor() // the filter input is always on row 0
	return v
}

// listHelp renders one key binding per line, with the key itself
// bolded so it stands out from its (dimmer) description.
func (m model) listHelp() string {
	items := [][2]string{
		{"↑/↓", "move"},
		{"enter", "select"},
		{"esc", "cancel"},
		{"ctrl+a", "add"},
	}
	if len(m.filtered) != 0 {
		items = append(items,
			[2]string{"ctrl+e", "edit"},
			[2]string{"ctrl+r", "rename"},
			[2]string{"ctrl+d", "delete"},
		)
	}

	lines := make([]string, len(items))
	for i, kv := range items {
		lines[i] = m.styles.helpKey.Render(kv[0]) + " " + m.styles.help.Render(kv[1])
	}
	return strings.Join(lines, "\n")
}

func (m model) viewForm(title, help string) tea.View {
	var b strings.Builder

	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(m.form.View())
	b.WriteString("\n")

	if m.pendingErr != "" {
		b.WriteString(m.styles.errMsg.Render(m.pendingErr))
		b.WriteString("\n")
	}

	b.WriteString(m.styles.help.Render(help))

	v := tea.NewView(b.String())
	if cur := m.form.Cursor(); cur != nil {
		cur.Position.Y = 1 // the title occupies row 0
		v.Cursor = cur
	}
	return v
}

func (m model) viewConfirm(prompt string) string {
	var b strings.Builder

	b.WriteString(prompt)
	b.WriteString(" [y/N]\n")

	if m.pendingErr != "" {
		b.WriteString(m.styles.errMsg.Render(m.pendingErr))
		b.WriteString("\n")
	}

	b.WriteString(m.styles.help.Render("y confirm · n/esc cancel"))

	return b.String()
}

// Run displays the interactive picker seeded with initialFilter and
// returns the shell command chosen by the user, or "" if the user
// cancelled the selection. Add/edit/rename/delete sub-flows persist
// to st immediately as they're confirmed, independently of the
// final selection.
func Run(initialFilter string, st *store.Store) (string, error) {
	ttyIn, ttyOut, err := tea.OpenTTY()
	if err != nil {
		return "", fmt.Errorf("open controlling terminal: %w", err)
	}
	defer ttyIn.Close()
	defer ttyOut.Close()

	m := newModel(initialFilter, st, newStyles())

	p := tea.NewProgram(m, tea.WithInput(ttyIn), tea.WithOutput(ttyOut))

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
