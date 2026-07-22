// Package tui implements the interactive command picker and all
// in-picker management (add/edit/rename/delete). It renders on the
// controlling terminal (not on stdout) so that the picker can read
// input and display output without interfering with the caller's
// own stdout.
package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/silvio/cl/internal/store"
)

// styles holds the lipgloss styles used by the picker's own chrome
// (list selection, command previews, help text, errors). Lip Gloss
// v2 styles carry no I/O state of their own - the terminal's color
// profile is detected and applied automatically by Bubble Tea when
// it writes to the program's output - so, unlike in v1, these don't
// need to be bound to any particular renderer.
type styles struct {
	selected  lipgloss.Style
	command   lipgloss.Style
	help      lipgloss.Style
	helpKey   lipgloss.Style
	errMsg    lipgloss.Style
	paramHint lipgloss.Style
}

func newStyles() styles {
	return styles{
		selected:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
		command:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		help:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		helpKey:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("246")),
		errMsg:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("204")),
		paramHint: lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("246")),
	}
}

// newTextArea returns a focused textarea.Model configured as a
// single logical line of text that word-wraps across as many
// terminal rows as it needs - the same way any other long line
// wraps in the terminal - instead of scrolling horizontally or
// overflowing past the terminal's edge. It draws a real terminal
// cursor - a thin, blinking "|" bar, the shape most shells use by
// default - instead of bubbles' virtual, block-shaped one.
//
// It's kept to a single logical line by construction: Enter never
// reaches it (updateForm/updateList intercept it to submit/select
// instead), and pasted text has its newlines collapsed to spaces by
// sanitizePaste before being inserted.
func newTextArea(prompt, placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Prompt = prompt // kept for introspection (Width()); rendering uses the func below instead
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false

	// The prompt marks where a logical line begins, so it must only
	// show on its first (possibly only) row - continuation rows from
	// wrapping get a same-width blank gutter instead, the same way a
	// shell's own multi-line prompt continuation works.
	ta.SetPromptFunc(lipgloss.Width(prompt), func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return prompt
		}
		return ""
	})

	// Grow/shrink to fit exactly the (soft-wrapped) content - never
	// more, never less - so there's no dead space and no internal
	// scrolling to reconcile with our own row math below.
	ta.DynamicHeight = true
	ta.MinHeight = 1
	ta.SetHeight(1)

	ta.SetVirtualCursor(false)

	s := ta.Styles()
	s.Cursor.Shape = tea.CursorBar
	s.Cursor.Color = nil // inherit the terminal's own cursor color
	// This field is always exactly one (possibly wrapped) logical
	// line, so "the current line" - which textarea highlights with
	// a background fill by default, as in a code editor - is the
	// entire field. Disable that so it renders as a plain input.
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Blurred.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	ta.Focus()
	return ta
}

// sanitizePaste collapses any newlines in pasted text into spaces.
// input/form must always hold exactly one logical line - typing
// can't violate that since Enter is handled before it ever reaches
// them, but a clipboard paste can contain arbitrary text, including
// embedded newlines that would otherwise turn a name/command/filter
// into multiple logical lines.
func sanitizePaste(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// mode selects which screen/interaction the model is currently
// showing. modeList is the default filter picker; the others
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
	modeFillPlaceholders
)

type model struct {
	st  *store.Store
	cfg *store.Config

	input    textarea.Model
	all      []store.Entry
	filtered []store.Entry
	cursor   int
	scroll   int
	selected store.Entry
	quitting bool
	styles   styles
	version  string // set by Run(), empty means no header

	// width is the known terminal width (0 until the first
	// tea.WindowSizeMsg arrives). It's applied to input/form via
	// applyWidth, and to the plain text surrounding them (titles,
	// help, errors) via wrap/wrapStyled, so that nothing ever
	// overflows onto the terminal's own line wrap - which would
	// throw off the fixed row math (cursor Y) below it.
	width  int
	height int

	mode mode

	// form is the single-line input reused by every add/edit
	// sub-flow (name entry, then command entry, then editing an
	// existing command) and by the placeholder-filling flow.
	form textarea.Model

	pendingName  string // name captured by modeAddName/modeRenameName, staged until modeAddValue/modeConfirmRename commits it
	pendingValue string // command captured by modeEditValue, staged until modeConfirmSaveEdit commits it
	pendingErr   string // inline validation/save error shown next to the active form
	target       store.Entry

	// placeholder-filling state (modeFillPlaceholders).
	placeholders []placeholder // parsed from the selected command
	phIdx        int           // which placeholder is currently being filled
	phVals       []string      // values entered so far (index-aligned with placeholders)
}

func newModel(initialFilter string, st *store.Store, cfg *store.Config, sty styles) model {
	ti := newTextArea("cl> ", "type to filter...")
	ti.SetValue(initialFilter)
	ti.CursorEnd()

	// form isn't shown until an add/edit/rename sub-flow starts (see
	// setForm), but it must still be a real textarea.Model - not the
	// zero value - since applyWidth touches it unconditionally on
	// every tea.WindowSizeMsg regardless of the current mode.
	m := model{st: st, cfg: cfg, input: ti, form: newTextArea("> ", ""), styles: sty}
	m.all = st.List()
	m.refilter()

	return m
}

// setForm replaces m.form with a fresh input and re-applies the
// known terminal width to it (newTextArea can't do this itself,
// since it has no access to the model).
func (m *model) setForm(ta textarea.Model) {
	m.form = ta
	m.applyWidth()
}

// applyWidth constrains input/form to the known terminal width.
// The filter input is narrower to fit inside the bordered box.
func (m *model) applyWidth() {
	if m.width <= 0 {
		return
	}
	// Filter sits inside the box (2 border chars).
	m.input.SetWidth(m.width - 2)
	m.form.SetWidth(m.width)
}

// wrap word-wraps s to the known terminal width, exactly like
// input/form do via applyWidth, so plain text (titles, help,
// prompts) never overflows onto the terminal's own line wrap
// either. It's a no-op until the first tea.WindowSizeMsg is known.
func (m *model) wrap(s string) string {
	if m.width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(m.width).Render(s)
}

// wrapStyled is wrap, with sty applied to the text at the same time
// (word-wrapping and styling in a single Render, so the style - not
// just the raw text - is what ends up constrained to m.width).
func (m *model) wrapStyled(sty lipgloss.Style, s string) string {
	if m.width <= 0 {
		return sty.Render(s)
	}
	return sty.Width(m.width).Render(s)
}

// refilter narrows m.all down to entries whose name contains the
// current filter query as a case-insensitive substring - the whole
// typed sequence has to appear together, not just its letters
// scattered anywhere in the name (as a fuzzy subsequence match
// would allow).
func (m *model) refilter() {
	query := strings.ToLower(m.input.Value())

	if query == "" {
		m.filtered = m.all
	} else {
		filtered := make([]store.Entry, 0, len(m.all))
		for _, e := range m.all {
			if strings.Contains(strings.ToLower(e.Name), query) {
				filtered = append(filtered, e)
			}
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
	m.target = store.Entry{}
	m.selected = store.Entry{}
}

func (m *model) startAdd() {
	m.mode = modeAddName
	m.pendingErr = ""
	m.setForm(newTextArea("> ", "command name (spaces allowed)"))
}

func (m *model) startEdit(e store.Entry) {
	m.mode = modeEditValue
	m.target = e
	m.pendingErr = ""
	m.setForm(newTextArea("> ", "command"))
	m.form.SetValue(e.Command)
	m.form.CursorEnd()
}

func (m *model) startRename(e store.Entry) {
	m.mode = modeRenameName
	m.target = e
	m.pendingErr = ""
	m.setForm(newTextArea("> ", "command name (spaces allowed)"))
	m.form.SetValue(e.Name)
	m.form.CursorEnd()
}

func (m *model) startDelete(e store.Entry) {
	m.mode = modeConfirmDelete
	m.target = e
}

// startFillPlaceholders enters the placeholder-filling flow for the
// selected entry. It parses placeholders from the command and
// pre-fills each value with its default (if any).
func (m *model) startFillPlaceholders(e store.Entry, phs []placeholder) {
	m.mode = modeFillPlaceholders
	m.selected = e
	m.placeholders = phs
	m.phIdx = 0
	m.phVals = make([]string, len(phs))
	for i, ph := range phs {
		m.phVals[i] = ph.Default
	}
	m.pendingErr = ""
	m.setForm(newTextArea("> ", "value"))
	m.form.SetValue(m.phVals[0])
	m.form.CursorEnd()
}

// headerView renders a colorful ASCII logo with the "cl" letters
// and the command launcher version string. It returns an empty
// string when no version is set (tests, for example).
func (m model) headerView() string {
	if m.version == "" {
		return ""
	}

	type logoPart struct{ c, l string }
	parts := []logoPart{
		{c: " ██████╗", l: " ██╗"},
		{c: "██╔════╝", l: " ██║"},
		{c: "██║     ", l: " ██║"},
		{c: "██║     ", l: " ██║"},
		{c: "╚██████╗", l: " ███████╗"},
		{c: " ╚═════╝", l: " ╚══════╝"},
	}

	cStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	lStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("246"))
	verStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	versionStr := fmt.Sprintf("v%s - SPLTek", m.version)

	var b strings.Builder
	for i, p := range parts {
		b.WriteString(cStyle.Render(p.c))
		b.WriteString(lStyle.Render(p.l))
		switch i {
		case 0:
			b.WriteString("  ")
			b.WriteString(titleStyle.Render("Command Launcher"))
		case 1:
			b.WriteString("  ")
			b.WriteString(verStyle.Render(versionStr))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// headerHeight returns the number of terminal rows consumed by the
// header view, or 0 when no header is shown (version is empty).
func (m model) headerHeight() int {
	if m.version == "" {
		return 0
	}
	return strings.Count(m.headerView(), "\n")
}

// toggleShowCommand flips and persists the showCommand setting,
// which controls whether the list shows each entry's command
// next to its name. Enter always runs commands directly
// regardless of this setting — see store.Config.
func (m *model) toggleShowCommand() {
	m.cfg.SetShowCommand(!m.cfg.ShowCommand())
	if err := m.cfg.Save(); err != nil {
		m.pendingErr = fmt.Sprintf("save failed: %v", err)
		return
	}
	m.pendingErr = ""
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sizeMsg.Width
		m.height = sizeMsg.Height
		m.applyWidth()
		// Force a full repaint on resize. Without this, expanding
		// the terminal (especially after inline/alt-screen
		// transitions) can leave blank "holes" that were never
		// drawn into.
		return m, tea.ClearScreen
	}

	switch m.mode {
	case modeAddName, modeAddValue, modeEditValue, modeRenameName:
		return m.updateForm(msg)
	case modeConfirmSaveEdit, modeConfirmRename, modeConfirmDelete:
		return m.updateConfirm(msg)
	case modeFillPlaceholders:
		return m.updateFillPlaceholders(msg)
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
			// Enter always runs the command directly. The caller
			// receives m.selected and executes it.
			if len(m.filtered) > 0 {
				entry := m.filtered[m.cursor]
				phs := parsePlaceholders(entry.Command)
				if len(phs) > 0 {
					m.startFillPlaceholders(entry, phs)
					return m, nil
				}
				m.selected = entry
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
				if m.cursor >= m.scroll+m.visibleRows() {
					m.scroll = m.cursor - m.visibleRows() + 1
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

		case "ctrl+s":
			m.toggleShowCommand()
			return m, nil
		}
	}

	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		m.input.InsertString(sanitizePaste(pasteMsg.Content))
		m.refilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refilter()

	return m, cmd
}

// visibleRows returns how many list entries fit in the current
// terminal height after accounting for the filter, borders, help
// and gaps (a fixed chrome budget). When the height hasn't been
// received yet (tea.WindowSizeMsg), it returns a conservative
// default so scrolling still works. Arrow-key scrolling inside the
// list covers entries beyond what fits in the frame.
func (m model) visibleRows() int {
	if m.height <= 0 {
		return 20
	}

	// The filter input is now inside the box, so its height is part
	// of the box content. Account for its rendered height so the list
	// doesn't overflow.
	filterHeight := 1 // fallback when width is unknown
	if m.width > 0 {
		filterHeight = lipgloss.Height(m.input.View())
	}

	// Box chrome: top border (1) + separator (1) + bottom border (1) = 3.
	// Plus help text height.
	boxChrome := 3
	if m.width > 0 {
		boxChrome += lipgloss.Height(m.wrapStyled(m.styles.help, m.listHelp()))
	} else {
		boxChrome += strings.Count(m.listHelp(), "\n") + 1
	}
	availLines := m.height - filterHeight - boxChrome - m.headerHeight()
	if availLines < 1 {
		availLines = 1
	}

	perEntry := 1
	if m.cfg.ShowCommand() {
		perEntry = 3
	}
	availEntries := availLines / perEntry
	if availEntries < 1 {
		availEntries = 1
	}
	return availEntries
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

	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		m.form.InsertString(sanitizePaste(pasteMsg.Content))
		return m, nil
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
		m.setForm(newTextArea("> ", "shell command"))
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

// updateFillPlaceholders drives modeFillPlaceholders: a single-line
// input for the current placeholder. Enter validates and advances to
// the next placeholder (or resolves and quits when all are filled);
// Esc cancels back to the list without running the command.
func (m model) updateFillPlaceholders(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.cancelForm()
			return m, nil

		case "enter":
			val := strings.TrimSpace(m.form.Value())
			// If the placeholder has no default and the value is
			// empty, reject and keep the user on the same field.
			if val == "" && m.placeholders[m.phIdx].Default == "" {
				m.pendingErr = "value required"
				return m, nil
			}
			m.phVals[m.phIdx] = val
			m.pendingErr = ""

			m.phIdx++
			if m.phIdx >= len(m.placeholders) {
				// All placeholders filled: resolve and quit.
				m.selected.Command = resolveCommand(m.selected.Command, m.placeholders, m.phVals)
				m.quitting = true
				return m, tea.Quit
			}

			// Advance to the next placeholder.
			m.setForm(newTextArea("> ", "value"))
			m.form.SetValue(m.phVals[m.phIdx])
			m.form.CursorEnd()
			return m, nil
		}
	}

	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		m.form.InsertString(sanitizePaste(pasteMsg.Content))
		return m, nil
	}

	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
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
		// Leaving AltScreen unset exits the alternate buffer and
		// restores the user's previous terminal content.
		return tea.NewView("")
	}

	var v tea.View
	switch m.mode {
	case modeAddName:
		v = m.viewForm("Add command - name:", "enter continue · esc cancel")
	case modeAddValue:
		v = m.viewForm(fmt.Sprintf("Add command %q - shell command:", m.pendingName), "enter save · esc cancel")
	case modeEditValue:
		v = m.viewForm(fmt.Sprintf("Edit %q:", m.target.Name), "enter continue · esc cancel")
	case modeRenameName:
		v = m.viewForm(fmt.Sprintf("Rename %q:", m.target.Name), "enter continue · esc cancel")
	case modeConfirmSaveEdit:
		v = tea.NewView(m.viewConfirm(fmt.Sprintf("Save %q -> %s ?", m.target.Name, m.pendingValue)))
	case modeConfirmRename:
		v = tea.NewView(m.viewConfirm(fmt.Sprintf("Rename %q -> %q ?", m.target.Name, m.pendingName)))
	case modeConfirmDelete:
		v = tea.NewView(m.viewConfirm(fmt.Sprintf("Delete %q (%s) ?", m.target.Name, m.target.Command)))
	case modeFillPlaceholders:
		v = m.viewFillPlaceholders()
	default:
		v = m.viewList()
	}
	// Always paint into the alternate screen buffer so the whole
	// frame is under Bubble Tea's control. Inline (main-buffer)
	// rendering leaves blank holes when the window grows and was
	// also what forced the old "pad with newlines" scroll hack.
	v.AltScreen = true
	return v
}

// writeBoxRow writes a single row inside the bordered box: left border,
// content padded to innerWidth, right border, newline. When width is
// unknown (m.width <= 0), borders are omitted.
func (m model) writeBoxRow(b *strings.Builder, bs lipgloss.Style, innerWidth int, line string) {
	if m.width > 0 {
		b.WriteString(bs.Render("│"))
		b.WriteString(lipgloss.NewStyle().Width(innerWidth).Render(line))
		b.WriteString(bs.Render("│"))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")
}

// writeListEntry renders a single entry inside the box: name line
// (with optional placeholder hint) and, when showCommand is on, the
// command itself indented underneath followed by a blank spacer row.
func (m model) writeListEntry(b *strings.Builder, bs lipgloss.Style, innerWidth int, entry store.Entry, isSelected bool) {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	// Name line.
	name := entry.Name
	if isSelected {
		name = m.styles.selected.Render(name)
	}
	if phs := parsePlaceholders(entry.Command); len(phs) > 0 {
		name += " " + m.styles.paramHint.Render(buildParamHint(phs))
	}
	m.writeBoxRow(b, bs, innerWidth, prefix+name)

	// Command line(s).
	if m.cfg.ShowCommand() {
		cmdIndent := strings.Repeat(" ", lipgloss.Width(prefix))
		cmdText := m.styles.command.Render(entry.Command)
		if m.width > 0 {
			wrapWidth := innerWidth - lipgloss.Width(cmdIndent)
			if wrapWidth < 10 {
				wrapWidth = 10
			}
			for _, wl := range strings.Split(lipgloss.NewStyle().Width(wrapWidth).Render(cmdText), "\n") {
				m.writeBoxRow(b, bs, innerWidth, cmdIndent+wl)
			}
		} else {
			m.writeBoxRow(b, bs, innerWidth, cmdIndent+cmdText)
		}
		// Blank spacer between entries when commands are shown.
		m.writeBoxRow(b, bs, innerWidth, "")
	}
}

func (m model) viewList() tea.View {
	var b strings.Builder

	b.WriteString(m.headerView())

	innerWidth := m.width - 2
	if innerWidth < 10 {
		innerWidth = 10
	}
	bs := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Box: top border, filter, separator.
	if m.width > 0 {
		b.WriteString(bs.Render("┌" + strings.Repeat("─", innerWidth) + "┐"))
		b.WriteString("\n")
	}
	for _, line := range strings.Split(strings.TrimRight(m.input.View(), "\n"), "\n") {
		m.writeBoxRow(&b, bs, innerWidth, line)
	}
	if m.width > 0 {
		b.WriteString(bs.Render("├" + strings.Repeat("─", innerWidth) + "┤"))
		b.WriteString("\n")
	}

	// List entries.
	maxRows := m.visibleRows()
	end := m.scroll + maxRows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	if m.scroll > 0 {
		m.writeBoxRow(&b, bs, innerWidth, m.styles.help.Render("  ↑ more above"))
	}
	for i := m.scroll; i < end; i++ {
		m.writeListEntry(&b, bs, innerWidth, m.filtered[i], i == m.cursor)
	}
	if end < len(m.filtered) {
		m.writeBoxRow(&b, bs, innerWidth, m.styles.help.Render("  ↓ more below"))
	}

	if len(m.all) == 0 {
		m.writeBoxRow(&b, bs, innerWidth, m.styles.help.Render("  empty list"))
	} else if len(m.filtered) == 0 {
		m.writeBoxRow(&b, bs, innerWidth, m.styles.help.Render("  no matching commands"))
	}

	// Box: bottom border.
	if m.width > 0 {
		b.WriteString(bs.Render("└" + strings.Repeat("─", innerWidth) + "┘"))
		b.WriteString("\n")
	}

	if m.pendingErr != "" {
		b.WriteString(m.wrapStyled(m.styles.errMsg, m.pendingErr))
		b.WriteString("\n")
	}

	b.WriteString(m.listHelp())

	v := tea.NewView(b.String())
	if c := m.input.Cursor(); c != nil {
		c.Y += m.headerHeight() + 1 // header rows + top border
		c.X += 1                     // left border "│"
		v.Cursor = c
	}
	return v
}

// listHelp renders one key binding per line, with the key itself
// bolded so it stands out from its (dimmer) description.
func (m model) listHelp() string {
	items := [][2]string{
		{"↑/↓", "move"},
		{"enter", "run selected"},
		{"ctrl+a", "add new command"},
	}
	if len(m.filtered) != 0 {
		items = append(items,
			[2]string{"ctrl+e", "edit selected"},
			[2]string{"ctrl+r", "rename selected"},
			[2]string{"ctrl+d", "delete selected"},
		)
	}
	items = append(items,
		[2]string{"ctrl+s", "command show toggle"},
		[2]string{"esc", "cancel"},
	)

	lines := make([]string, len(items))
	for i, kv := range items {
		lines[i] = m.styles.helpKey.Render(kv[0]) + " " + m.styles.help.Render(kv[1])
	}
	return strings.Join(lines, "\n")
}

func (m model) viewForm(title, help string) tea.View {
	var b strings.Builder

	titleView := m.wrap(title)
	b.WriteString(titleView)
	b.WriteString("\n")
	b.WriteString(m.form.View())
	b.WriteString("\n")

	if m.pendingErr != "" {
		b.WriteString(m.wrapStyled(m.styles.errMsg, m.pendingErr))
		b.WriteString("\n")
	}

	b.WriteString(m.wrapStyled(m.styles.help, help))

	v := tea.NewView(b.String())
	if cur := m.form.Cursor(); cur != nil {
		cur.Position.Y += lipgloss.Height(titleView) // the title occupies the row(s) above the form
		v.Cursor = cur
	}
	return v
}

func (m model) viewConfirm(prompt string) string {
	var b strings.Builder

	b.WriteString(m.wrap(prompt + " [y/N]"))
	b.WriteString("\n")

	if m.pendingErr != "" {
		b.WriteString(m.wrapStyled(m.styles.errMsg, m.pendingErr))
		b.WriteString("\n")
	}

	b.WriteString(m.wrapStyled(m.styles.help, "y confirm · n/esc cancel"))

	return b.String()
}

// viewFillPlaceholders renders the placeholder-filling screen: a
// preview of the command with resolved placeholders on top, the
// current placeholder's input form in the middle, and help at the
// bottom.
func (m model) viewFillPlaceholders() tea.View {
	var b strings.Builder

	// Preview line: the command with filled placeholders replaced.
	ph := m.placeholders[m.phIdx]
	currentText := m.form.Value()
	preview := buildPreview(m.selected.Command, m.placeholders, m.phVals, m.phIdx, currentText)
	previewView := m.wrapStyled(m.styles.command, preview)
	b.WriteString(previewView)
	b.WriteString("\n\n")

	// Form label: the placeholder name.
	labelView := m.wrap(ph.Name + ":")
	b.WriteString(labelView)
	b.WriteString("\n")
	b.WriteString(m.form.View())
	b.WriteString("\n")

	if m.pendingErr != "" {
		b.WriteString(m.wrapStyled(m.styles.errMsg, m.pendingErr))
		b.WriteString("\n")
	}

	// Help text adapts to whether this is the last placeholder.
	var help string
	if m.phIdx == len(m.placeholders)-1 {
		help = "enter run · esc cancel"
	} else {
		help = "enter continue · esc cancel"
	}
	b.WriteString(m.wrapStyled(m.styles.help, help))

	// Cursor sits on the form, below the same preview/label rows we
	// just rendered (use those exact strings so wrap height matches).
	v := tea.NewView(b.String())
	if cur := m.form.Cursor(); cur != nil {
		cur.Position.Y += lipgloss.Height(previewView) + 1 + lipgloss.Height(labelView)
		v.Cursor = cur
	}
	return v
}

// Run displays the interactive picker seeded with initialFilter and
// returns the entry chosen by the user, or the zero store.Entry
// ({}) if the user cancelled the selection. Add/edit/rename/delete
// sub-flows persist to st immediately as they're confirmed,
// independently of the final selection. cfg.ShowCommand (toggled
// in-picker with ctrl+s, persisted immediately) controls whether the
// list shows each entry's command next to its name.
func Run(initialFilter string, st *store.Store, cfg *store.Config, version string) (store.Entry, error) {
	ttyIn, ttyOut, err := tea.OpenTTY()
	if err != nil {
		return store.Entry{}, fmt.Errorf("open controlling terminal: %w", err)
	}
	defer ttyIn.Close()
	defer ttyOut.Close()

	m := newModel(initialFilter, st, cfg, newStyles())
	m.version = version

	p := tea.NewProgram(m, tea.WithInput(ttyIn), tea.WithOutput(ttyOut))

	final, err := p.Run()
	if err != nil {
		return store.Entry{}, err
	}

	fm, ok := final.(model)
	if !ok {
		return store.Entry{}, fmt.Errorf("unexpected model type")
	}

	return fm.selected, nil
}
