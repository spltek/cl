package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/silvio/cl/internal/store"
)

func testEntries() []store.Entry {
	return []store.Entry{
		{Name: "backup", Command: "rsync -av ./src ./backup"},
		{Name: "build", Command: "npm run build"},
		{Name: "cleanup", Command: "rm -rf dist"},
	}
}

// testStore returns a *store.Store backed by a temporary config dir
// (so Save() calls made by the model during a test never touch the
// real user config), pre-populated with testEntries().
func testStore(t *testing.T) *store.Store {
	t.Helper()

	t.Setenv("CL_CONFIG_DIR", t.TempDir())

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}

	for _, e := range testEntries() {
		s.Set(e.Name, e.Command)
	}

	return s
}

// emptyStore is like testStore but starts with no entries at all,
// for exercising the empty-list add flow.
func emptyStore(t *testing.T) *store.Store {
	t.Helper()

	t.Setenv("CL_CONFIG_DIR", t.TempDir())

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}

	return s
}

// testConfig returns a *store.Config backed by its own temporary
// config dir, with every setting at its default (showCommand:
// false).
func testConfig(t *testing.T) *store.Config {
	t.Helper()

	t.Setenv("CL_CONFIG_DIR", t.TempDir())

	c, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("store.LoadConfig() error = %v", err)
	}

	return c
}

// testStyles returns a styles value suitable for tests, which don't
// care about actual color output.
func testStyles() styles {
	return newStyles()
}

// key builds a synthetic press of a special key (arrows, enter, esc,
// backspace, ...), identified by its tea.Key* rune code.
func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// ctrlKey builds a synthetic ctrl+<r> key press, e.g. ctrlKey('a')
// for ctrl+a.
func ctrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

// runeKey builds a synthetic press of a plain printable character,
// as textinput reads the typed text from Text rather than Code.
func runeKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func update(m model, msg tea.Msg) (model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(model), cmd
}

func typeString(m model, s string) model {
	for _, r := range s {
		m, _ = update(m, runeKey(r))
	}
	return m
}

func TestNewModel_NoFilterShowsAllEntries(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	if len(m.filtered) != 3 {
		t.Fatalf("filtered len = %d, want 3", len(m.filtered))
	}
}

func TestNewModel_FilterNarrowsByName(t *testing.T) {
	m := newModel("bui", testStore(t), testConfig(t), testStyles())

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered = %+v, want just %q", m.filtered, "build")
	}
}

func TestNewModel_FilterIsCaseInsensitive(t *testing.T) {
	m := newModel("BUI", testStore(t), testConfig(t), testStyles())

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered = %+v, want just %q", m.filtered, "build")
	}
}

func TestNewModel_FilterRequiresContiguousSubstringNotFuzzySubsequence(t *testing.T) {
	// "bd" is a fuzzy subsequence of "build" (b, then d) but not a
	// substring of it - the whole typed sequence has to appear
	// together, letters can't be scattered across the name.
	m := newModel("bd", testStore(t), testConfig(t), testStyles())

	if len(m.filtered) != 0 {
		t.Fatalf("filtered = %+v, want none: %q is not a contiguous substring of any entry name", m.filtered, "bd")
	}
}

func TestUpdate_ArrowKeysMoveCursorWithinBounds(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	// Moving up from the first row should have no effect.
	m, _ = update(m, key(tea.KeyUp))
	if m.cursor != 0 {
		t.Fatalf("cursor after Up at top = %d, want 0", m.cursor)
	}

	m, _ = update(m, key(tea.KeyDown))
	if m.cursor != 1 {
		t.Fatalf("cursor after Down = %d, want 1", m.cursor)
	}

	// Moving down past the last row should clamp at len-1.
	for i := 0; i < 10; i++ {
		m, _ = update(m, key(tea.KeyDown))
	}
	if m.cursor != len(m.filtered)-1 {
		t.Fatalf("cursor after many Down = %d, want %d", m.cursor, len(m.filtered)-1)
	}
}

func TestUpdate_EnterSelectsHighlightedCommand(t *testing.T) {
	m := newModel("clean", testStore(t), testConfig(t), testStyles())

	m, cmd := update(m, key(tea.KeyEnter))

	if m.selected.Command != "rm -rf dist" {
		t.Fatalf("selected.Command = %q, want %q", m.selected.Command, "rm -rf dist")
	}
	if m.selected.Name != "cleanup" {
		t.Fatalf("selected.Name = %q, want %q", m.selected.Name, "cleanup")
	}
	if !m.quitting {
		t.Fatalf("quitting = false, want true after Enter")
	}
	if cmd == nil {
		t.Fatalf("expected a tea.Quit command, got nil")
	}
}

func TestUpdate_EscCancelsWithoutSelection(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, cmd := update(m, key(tea.KeyEsc))

	if m.selected != (store.Entry{}) {
		t.Fatalf("selected = %+v, want zero value after Esc", m.selected)
	}
	if !m.quitting {
		t.Fatalf("quitting = false, want true after Esc")
	}
	if cmd == nil {
		t.Fatalf("expected a tea.Quit command, got nil")
	}
}

func TestUpdate_TypingNarrowsFilterAndResetsCursor(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyDown))
	if m.cursor != 1 {
		t.Fatalf("cursor before typing = %d, want 1", m.cursor)
	}

	m = typeString(m, "build")

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered after typing 'build' = %+v", m.filtered)
	}
	if m.cursor != 0 {
		t.Fatalf("cursor after typing = %d, want reset to 0", m.cursor)
	}
}

// --- Add flow (ctrl+a) ---

func TestAdd_CtrlAEntersAddNameModeEvenOnEmptyList(t *testing.T) {
	st := emptyStore(t)
	m := newModel("", st, testConfig(t), testStyles())

	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0 for an empty store", len(m.filtered))
	}

	m, _ = update(m, ctrlKey('a'))
	if m.mode != modeAddName {
		t.Fatalf("mode after ctrl+a on empty list = %v, want modeAddName", m.mode)
	}
}

func TestAdd_FullFlowPersistsToStore(t *testing.T) {
	st := testStore(t)
	m := newModel("", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	if m.mode != modeAddName {
		t.Fatalf("mode after ctrl+a = %v, want modeAddName", m.mode)
	}

	m = typeString(m, "deploy prod")
	m, _ = update(m, key(tea.KeyEnter))
	if m.mode != modeAddValue {
		t.Fatalf("mode after entering unique name = %v, want modeAddValue (pendingErr=%q)", m.mode, m.pendingErr)
	}
	if m.pendingName != "deploy prod" {
		t.Fatalf("pendingName = %q, want %q", m.pendingName, "deploy prod")
	}

	m = typeString(m, "kubectl apply -f prod.yaml")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeList {
		t.Fatalf("mode after saving new command = %v, want modeList (pendingErr=%q)", m.mode, m.pendingErr)
	}

	got, ok := st.Get("deploy prod")
	if !ok || got != "kubectl apply -f prod.yaml" {
		t.Fatalf("store.Get(%q) = (%q, %v), want (%q, true)", "deploy prod", got, ok, "kubectl apply -f prod.yaml")
	}

	found := false
	for _, e := range m.all {
		if e.Name == "deploy prod" {
			found = true
		}
	}
	if !found {
		t.Fatalf("model.all does not contain the newly added entry: %+v", m.all)
	}
}

func TestAdd_NameCanContainSpaces(t *testing.T) {
	st := testStore(t)
	m := newModel("", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "my build")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeAddValue {
		t.Fatalf("mode = %v, want modeAddValue after a name containing spaces (pendingErr=%q)", m.mode, m.pendingErr)
	}
}

func TestAdd_EmptyNameShowsErrorAndStaysInAddName(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "   ") // whitespace-only, should trim to empty
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeAddName {
		t.Fatalf("mode = %v, want modeAddName to stay after empty name", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty validation message")
	}
}

func TestAdd_DuplicateNameShowsErrorAndStaysInAddName(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "build") // already exists in testEntries()
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeAddName {
		t.Fatalf("mode = %v, want modeAddName to stay after duplicate name", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty duplicate-name message")
	}
}

func TestAdd_EmptyCommandShowsErrorAndStaysInAddValue(t *testing.T) {
	st := testStore(t)
	m := newModel("", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "newcmd")
	m, _ = update(m, key(tea.KeyEnter))
	if m.mode != modeAddValue {
		t.Fatalf("mode = %v, want modeAddValue", m.mode)
	}

	m = typeString(m, "   ")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeAddValue {
		t.Fatalf("mode = %v, want modeAddValue to stay after empty command", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty validation message")
	}
	if _, ok := st.Get("newcmd"); ok {
		t.Fatalf("store.Get(newcmd) exists, want it not to be saved with an empty command")
	}
}

func TestAdd_EscFromAddNameDiscardsAndReturnsToList(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "whatever")
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc", m.mode)
	}
}

func TestAdd_EscFromAddValueDiscardsWithoutSaving(t *testing.T) {
	st := testStore(t)
	m := newModel("", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "newcmd")
	m, _ = update(m, key(tea.KeyEnter))

	m = typeString(m, "echo hi")
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc", m.mode)
	}
	if _, ok := st.Get("newcmd"); ok {
		t.Fatalf("store.Get(newcmd) exists, want the add to have been discarded by esc")
	}
}

// --- Edit flow (ctrl+e) ---

func TestEdit_CtrlEPrefillsFormWithCurrentCommand(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))

	if m.mode != modeEditValue {
		t.Fatalf("mode = %v, want modeEditValue", m.mode)
	}
	if m.form.Value() != "npm run build" {
		t.Fatalf("form value = %q, want prefilled with %q", m.form.Value(), "npm run build")
	}
}

func TestEdit_FullFlowPersistsAfterConfirm(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))
	m = typeString(m, " --watch")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeConfirmSaveEdit {
		t.Fatalf("mode = %v, want modeConfirmSaveEdit (pendingErr=%q)", m.mode, m.pendingErr)
	}

	m, _ = update(m, runeKey('y'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after confirming save", m.mode)
	}

	got, ok := st.Get("build")
	want := "npm run build --watch"
	if !ok || got != want {
		t.Fatalf("store.Get(build) = (%q, %v), want (%q, true)", got, ok, want)
	}
}

func TestEdit_DecliningConfirmationLeavesStoreUnchanged(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))
	m = typeString(m, " --watch")
	m, _ = update(m, key(tea.KeyEnter))
	m, _ = update(m, runeKey('n'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after declining", m.mode)
	}

	got, _ := st.Get("build")
	if got != "npm run build" {
		t.Fatalf("store.Get(build) = %q, want unchanged %q after declining", got, "npm run build")
	}
}

func TestEdit_EscAtConfirmationLeavesStoreUnchanged(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))
	m = typeString(m, " --watch")
	m, _ = update(m, key(tea.KeyEnter))
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc at confirmation", m.mode)
	}

	got, _ := st.Get("build")
	if got != "npm run build" {
		t.Fatalf("store.Get(build) = %q, want unchanged %q", got, "npm run build")
	}
}

func TestEdit_EscBeforeConfirmationSkipsConfirmationEntirely(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList directly after esc, without a confirmation step", m.mode)
	}
}

func TestEdit_EmptyValueShowsErrorAndStaysInEditValue(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('e'))

	// Clear the prefilled value entirely, then submit whitespace only.
	for range "npm run build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "   ")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeEditValue {
		t.Fatalf("mode = %v, want modeEditValue to stay after empty command", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty validation message")
	}

	got, _ := st.Get("build")
	if got != "npm run build" {
		t.Fatalf("store.Get(build) = %q, want unchanged", got)
	}
}

func TestEdit_CtrlEOnEmptyFilteredListIsNoop(t *testing.T) {
	m := newModel("does-not-exist", testStore(t), testConfig(t), testStyles())
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}

	m, _ = update(m, ctrlKey('e'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList to stay when there is nothing to edit", m.mode)
	}
}

// --- Rename flow (ctrl+r) ---

func TestRename_CtrlRPrefillsFormWithCurrentName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))

	if m.mode != modeRenameName {
		t.Fatalf("mode = %v, want modeRenameName", m.mode)
	}
	if m.form.Value() != "build" {
		t.Fatalf("form value = %q, want prefilled with %q", m.form.Value(), "build")
	}
}

func TestRename_FullFlowPersistsAfterConfirm(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))
	for range "build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "compile")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeConfirmRename {
		t.Fatalf("mode = %v, want modeConfirmRename (pendingErr=%q)", m.mode, m.pendingErr)
	}

	m, _ = update(m, runeKey('y'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after confirming rename", m.mode)
	}

	if _, ok := st.Get("build"); ok {
		t.Fatalf("store.Get(build) still exists, want it renamed away")
	}
	got, ok := st.Get("compile")
	if !ok || got != "npm run build" {
		t.Fatalf("store.Get(compile) = (%q, %v), want (%q, true)", got, ok, "npm run build")
	}
}

func TestRename_DuplicateNameShowsErrorAndStaysInRenameName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))
	for range "build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "cleanup") // already exists in testEntries()
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeRenameName {
		t.Fatalf("mode = %v, want modeRenameName to stay after duplicate name", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty duplicate-name message")
	}
}

func TestRename_ToSameNameIsAllowed(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))
	m, _ = update(m, key(tea.KeyEnter)) // resubmit the prefilled, unchanged name

	if m.mode != modeConfirmRename {
		t.Fatalf("mode = %v, want modeConfirmRename (pendingErr=%q)", m.mode, m.pendingErr)
	}

	m, _ = update(m, runeKey('y'))

	got, ok := st.Get("build")
	if !ok || got != "npm run build" {
		t.Fatalf("store.Get(build) = (%q, %v), want it to still exist unchanged", got, ok)
	}
}

func TestRename_EmptyNameShowsErrorAndStaysInRenameName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))
	for range "build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "   ")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeRenameName {
		t.Fatalf("mode = %v, want modeRenameName to stay after empty name", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty validation message")
	}
}

func TestRename_DecliningConfirmationLeavesStoreUnchanged(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('r'))
	for range "build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "compile")
	m, _ = update(m, key(tea.KeyEnter))
	m, _ = update(m, runeKey('n'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after declining", m.mode)
	}
	if _, ok := st.Get("build"); !ok {
		t.Fatalf("store.Get(build) missing, want it unchanged after declining the rename")
	}
}

func TestRename_CtrlROnEmptyFilteredListIsNoop(t *testing.T) {
	m := newModel("does-not-exist", testStore(t), testConfig(t), testStyles())
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}

	m, _ = update(m, ctrlKey('r'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList to stay when there is nothing to rename", m.mode)
	}
}

// --- Delete flow (ctrl+d) ---

func TestDelete_CtrlDEntersConfirmDeleteForHighlightedEntry(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('d'))

	if m.mode != modeConfirmDelete {
		t.Fatalf("mode = %v, want modeConfirmDelete", m.mode)
	}
	if m.target.Name != "build" {
		t.Fatalf("target.Name = %q, want %q", m.target.Name, "build")
	}
}

func TestDelete_ConfirmingRemovesFromStore(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('d'))
	m, _ = update(m, runeKey('y'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after confirming delete", m.mode)
	}
	if _, ok := st.Get("build"); ok {
		t.Fatalf("store.Get(build) still exists, want it removed")
	}
	for _, e := range m.all {
		if e.Name == "build" {
			t.Fatalf("model.all still contains the deleted entry: %+v", m.all)
		}
	}
}

func TestDelete_DecliningLeavesStoreUnchanged(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('d'))
	m, _ = update(m, runeKey('n'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after declining delete", m.mode)
	}
	if _, ok := st.Get("build"); !ok {
		t.Fatalf("store.Get(build) missing, want it to still exist after declining")
	}
}

func TestDelete_EscDeclinesLeavesStoreUnchanged(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('d'))
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc", m.mode)
	}
	if _, ok := st.Get("build"); !ok {
		t.Fatalf("store.Get(build) missing, want it to still exist after esc")
	}
}

// --- showCommand toggle (ctrl+s) ---

func TestShowCommand_DefaultsToFalse(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	if m.cfg.ShowCommand() {
		t.Fatalf("ShowCommand() = true, want false by default")
	}
	if strings.Contains(m.View().Content, "npm run build") {
		t.Fatalf("View() = %q, want it not to show command values by default", m.View().Content)
	}
}

func TestShowCommand_CtrlSTogglesAndPersistsSetting(t *testing.T) {
	cfg := testConfig(t)
	m := newModel("build", testStore(t), cfg, testStyles())

	m, _ = update(m, ctrlKey('s'))
	if !m.cfg.ShowCommand() {
		t.Fatalf("ShowCommand() after ctrl+s = false, want true")
	}
	if !cfg.ShowCommand() {
		t.Fatalf("the underlying store.Config was not persisted (Save wasn't called, or cfg isn't shared)")
	}
	if m.pendingErr != "" {
		t.Fatalf("pendingErr = %q, want empty after a successful toggle", m.pendingErr)
	}
	if !strings.Contains(m.View().Content, "npm run build") {
		t.Errorf("View() = %q, want it to show the command once ShowCommand is enabled", m.View().Content)
	}

	m, _ = update(m, ctrlKey('s'))
	if m.cfg.ShowCommand() {
		t.Fatalf("ShowCommand() after a second ctrl+s = true, want false")
	}
	if strings.Contains(m.View().Content, "npm run build") {
		t.Errorf("View() = %q, want it to hide the command again after toggling back off", m.View().Content)
	}
}

func TestShowCommand_ReloadedConfigReflectsPersistedToggle(t *testing.T) {
	// Keep everything under a single, fixed config dir for this test
	// - unlike testStore/testConfig, which each point CL_CONFIG_DIR
	// at their own fresh temp dir, here the whole point is to reload
	// from the very same directory ctrl+s just saved to.
	t.Setenv("CL_CONFIG_DIR", t.TempDir())

	st, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}
	st.Set("build", "npm run build")

	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("store.LoadConfig() error = %v", err)
	}

	m := newModel("build", st, cfg, testStyles())
	m, _ = update(m, ctrlKey('s'))

	reloaded, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("store.LoadConfig() (reload) error = %v", err)
	}
	if !reloaded.ShowCommand() {
		t.Fatalf("ShowCommand() on a freshly reloaded Config = false, want true after ctrl+s was saved")
	}
}

// --- View rendering ---
//
// These don't need a real TTY: View() is a pure function of the
// model's state, so we can assert on its output the same way we
// assert on any other field after driving Update() with synthetic
// key messages.

func TestView_Init_ReturnsNil(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init() = %v, want nil", cmd)
	}
}

func TestView_QuittingRendersEmptyString(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, key(tea.KeyEnter))

	if got := m.View().Content; got != "" {
		t.Fatalf("View() after quitting = %q, want empty", got)
	}
}

func TestView_ListShowsEntriesAndHelpWithoutCommands(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	view := m.View().Content
	for _, want := range []string{
		"backup", "build", "cleanup",
		"ctrl+a", "add", "ctrl+s", "command show", "ctrl+e", "edit", "ctrl+r", "rename", "ctrl+d", "delete",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestView_ListHelpLinesAreInExpectedOrder(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	help := m.listHelp()
	wantOrder := []string{
		"move", "run selected", "add new command",
		"edit selected", "rename selected", "delete selected",
		"command show toggle", "cancel",
	}

	lastIdx := -1
	for _, want := range wantOrder {
		idx := strings.Index(help, want)
		if idx == -1 {
			t.Fatalf("listHelp() = %q, want it to contain %q", help, want)
		}
		if idx < lastIdx {
			t.Fatalf("listHelp() = %q, want %q to come after the previous item", help, want)
		}
		lastIdx = idx
	}
}

func TestView_EmptyListOnlyMentionsAdd(t *testing.T) {
	m := newModel("", emptyStore(t), testConfig(t), testStyles())

	view := m.View().Content
	if !strings.Contains(view, "ctrl+a") || !strings.Contains(view, "add") {
		t.Errorf("View() = %q, want it to mention ctrl+a add", view)
	}
	if strings.Contains(view, "ctrl+e") || strings.Contains(view, "ctrl+r") || strings.Contains(view, "ctrl+d") {
		t.Errorf("View() = %q, want it not to mention edit/rename/delete when the list is empty", view)
	}
	if !strings.Contains(view, "no matching commands") {
		t.Errorf("View() = %q, want it to mention there are no matching commands", view)
	}
}

func TestView_AddNameShowsPromptAndForm(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")

	view := m.View().Content
	if !strings.Contains(view, "Add command") {
		t.Errorf("View() = %q, want it to mention adding a command", view)
	}
	if !strings.Contains(view, "deploy") {
		t.Errorf("View() = %q, want it to contain the typed name %q", view, "deploy")
	}
}

func TestView_AddNameShowsInlineErrorOnDuplicate(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "build")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	if !strings.Contains(view, "already used") {
		t.Errorf("View() = %q, want it to show the duplicate-name error inline", view)
	}
}

func TestView_AddValueShowsPendingName(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	if !strings.Contains(view, "deploy") {
		t.Errorf("View() = %q, want it to mention the pending name %q", view, "deploy")
	}
}

func TestView_EditValueShowsTargetName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('e'))

	view := m.View().Content
	if !strings.Contains(view, "build") {
		t.Errorf("View() = %q, want it to mention the entry being edited", view)
	}
}

func TestView_ConfirmSaveEditShowsOldNameAndNewValue(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('e'))
	m = typeString(m, " --watch")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	for _, want := range []string{"build", "npm run build --watch", "[y/N]"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestView_RenameNameShowsTargetName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('r'))

	view := m.View().Content
	if !strings.Contains(view, "Rename") || !strings.Contains(view, "build") {
		t.Errorf("View() = %q, want it to mention renaming the entry", view)
	}
}

func TestView_ConfirmRenameShowsOldAndNewName(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('r'))
	for range "build" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "compile")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	for _, want := range []string{"build", "compile", "[y/N]"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestView_ConfirmDeleteShowsNameAndCommand(t *testing.T) {
	m := newModel("build", testStore(t), testConfig(t), testStyles())
	m, _ = update(m, ctrlKey('d'))

	view := m.View().Content
	for _, want := range []string{"build", "npm run build", "[y/N]"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestDelete_CtrlDOnEmptyFilteredListIsNoop(t *testing.T) {
	m := newModel("does-not-exist", testStore(t), testConfig(t), testStyles())
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}

	m, _ = update(m, ctrlKey('d'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList to stay when there is nothing to remove", m.mode)
	}
}

// --- Terminal width handling ---
//
// A value longer than the terminal is wide must word-wrap onto
// extra rows of its own - like any other long line in the terminal
// - rather than scroll horizontally or overflow onto the terminal's
// next row (which would throw off the row math below it: help text,
// cursor Y).

func TestWidth_WindowSizeConstrainsFilterInput(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.WindowSizeMsg{Width: 20, Height: 24})

	if got, want := m.input.Width(), 20-lipgloss.Width(m.input.Prompt); got != want {
		t.Fatalf("input.Width() = %d, want %d", got, want)
	}
}

func TestWidth_AppliedToFormCreatedBeforeResize(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a')) // form created while width is still unknown
	m, _ = update(m, tea.WindowSizeMsg{Width: 20, Height: 24})

	if got, want := m.form.Width(), 20-lipgloss.Width(m.form.Prompt); got != want {
		t.Fatalf("form.Width() = %d, want %d", got, want)
	}
}

func TestWidth_PersistsAcrossFormReplacement(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")
	m, _ = update(m, key(tea.KeyEnter)) // modeAddName -> modeAddValue replaces m.form

	if m.mode != modeAddValue {
		t.Fatalf("mode = %v, want modeAddValue (pendingErr=%q)", m.mode, m.pendingErr)
	}
	if got, want := m.form.Width(), 20-lipgloss.Width(m.form.Prompt); got != want {
		t.Fatalf("form.Width() after replacement = %d, want %d (width was lost)", got, want)
	}
}

func TestWidth_LongValueWrapsOntoMultipleRowsInsteadOfScrolling(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")
	m, _ = update(m, key(tea.KeyEnter))

	long := "sshpass -p 'memori' ssh -T -nC -o ServerAliveInterval=120 -L 5432:aclambda-postgresqldb.example.rds.amazonaws.com:5432 example.online"
	m = typeString(m, long)

	if rows := m.form.Height(); rows <= 1 {
		t.Fatalf("form.Height() = %d, want >1 rows for a value much longer than the terminal", rows)
	}

	view := m.View().Content
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Fatalf("line %d rendered at width %d > terminal width 40: %q", i, w, line)
		}
	}

	// The full value must still be held by the form, untruncated and
	// unaltered by wrapping - wrapping is purely a rendering concern.
	if got := m.form.Value(); got != long {
		t.Fatalf("form.Value() = %q, want the full untruncated value %q", got, long)
	}
}

func TestWidth_PromptOnlyShownOnFirstWrappedRow(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")
	m, _ = update(m, key(tea.KeyEnter))
	m = typeString(m, "sshpass -p 'memori' ssh -T -nC -o ServerAliveInterval=120 -L 5432:aclambda-postgresqldb.example.rds.amazonaws.com:5432 example.online")

	if rows := m.form.Height(); rows <= 1 {
		t.Fatalf("form.Height() = %d, want >1 rows for this test to be meaningful", rows)
	}

	lines := strings.Split(m.form.View(), "\n")
	if got := strings.Count(lines[0], "> "); got != 1 {
		t.Fatalf("first rendered row = %q, want exactly one %q", lines[0], "> ")
	}
	for i, line := range lines[1:] {
		if strings.Contains(line, ">") {
			t.Fatalf("continuation row %d = %q, want no prompt on wrap-continuation rows", i+1, line)
		}
	}
}

func TestWidth_FormShrinksBackToOneRowWhenValueIsEmptied(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.WindowSizeMsg{Width: 20, Height: 24})
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "a very long name that will wrap across rows")

	if rows := m.form.Height(); rows <= 1 {
		t.Fatalf("form.Height() = %d, want >1 after typing a long name", rows)
	}

	for range "a very long name that will wrap across rows" {
		m, _ = update(m, key(tea.KeyBackspace))
	}

	if rows := m.form.Height(); rows != 1 {
		t.Fatalf("form.Height() = %d, want 1 after clearing the value", rows)
	}
}

// PasteMsg can carry arbitrary clipboard content, including embedded
// newlines; input/form must stay a single logical line regardless.

func TestPaste_NewlinesInPastedFilterAreCollapsedToSpaces(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, tea.PasteMsg{Content: "buil\nd"})

	if got, want := m.input.Value(), "buil d"; got != want {
		t.Fatalf("input.Value() = %q, want %q", got, want)
	}
}

func TestPaste_NewlinesInPastedFormValueAreCollapsedToSpaces(t *testing.T) {
	m := newModel("", testStore(t), testConfig(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m, _ = update(m, tea.PasteMsg{Content: "line one\nline two"})

	if got, want := m.form.Value(), "line one line two"; got != want {
		t.Fatalf("form.Value() = %q, want %q", got, want)
	}
}

// --- Placeholder flow ---

// placeholderStore returns a store pre-populated with commands that
// contain {{placeholder}} patterns.
func placeholderStore(t *testing.T) *store.Store {
	t.Helper()

	t.Setenv("CL_CONFIG_DIR", t.TempDir())

	s, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}

	s.Set("ssh", "ssh {{user}}@{{host}}")
	s.Set("git push", "git push {{remote:origin}} {{branch:main}}")
	s.Set("build", "npm run build")

	return s
}

func TestPlaceholder_EnterOnCommandWithPlaceholdersEntersFillMode(t *testing.T) {
	m := newModel("ssh", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode after Enter on command with placeholders = %v, want modeFillPlaceholders", m.mode)
	}
	if m.selected.Name != "ssh" {
		t.Fatalf("selected.Name = %q, want %q", m.selected.Name, "ssh")
	}
	if len(m.placeholders) != 2 {
		t.Fatalf("len(placeholders) = %d, want 2", len(m.placeholders))
	}
	if m.phIdx != 0 {
		t.Fatalf("phIdx = %d, want 0 (first placeholder)", m.phIdx)
	}
}

func TestPlaceholder_EnterOnCommandWithoutPlaceholdersBehavesNormally(t *testing.T) {
	m := newModel("build", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	if !m.quitting {
		t.Fatalf("quitting = false, want true after Enter on command without placeholders")
	}
	if m.selected.Name != "build" {
		t.Fatalf("selected.Name = %q, want %q", m.selected.Name, "build")
	}
}

func TestPlaceholder_FullFlowResolvesAndQuits(t *testing.T) {
	m := newModel("ssh", placeholderStore(t), testConfig(t), testStyles())

	// Enter the placeholder-filling flow.
	m, _ = update(m, key(tea.KeyEnter))
	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode = %v, want modeFillPlaceholders", m.mode)
	}

	// Fill the first placeholder ("user") - no default, so required.
	m = typeString(m, "admin")
	m, _ = update(m, key(tea.KeyEnter))
	if m.phIdx != 1 {
		t.Fatalf("phIdx after first Enter = %d, want 1 (second placeholder)", m.phIdx)
	}
	if m.phVals[0] != "admin" {
		t.Fatalf("phVals[0] = %q, want %q", m.phVals[0], "admin")
	}

	// Fill the second placeholder ("host") - no default.
	m = typeString(m, "prod.example.com")
	m, _ = update(m, key(tea.KeyEnter))

	if !m.quitting {
		t.Fatalf("quitting = false, want true after filling all placeholders")
	}
	if got, want := m.selected.Command, "ssh admin@prod.example.com"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_DefaultsArePreFilled(t *testing.T) {
	m := newModel("git push", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// First placeholder "remote" has default "origin" - pre-filled.
	if got, want := m.form.Value(), "origin"; got != want {
		t.Fatalf("form.Value() for first placeholder = %q, want pre-filled default %q", got, want)
	}
	if m.phVals[0] != "origin" {
		t.Fatalf("phVals[0] = %q, want default %q", m.phVals[0], "origin")
	}

	// Accept the default and move to the next.
	m, _ = update(m, key(tea.KeyEnter))

	if m.pendingErr != "" {
		t.Fatalf("pendingErr = %q after accepting non-empty default, want empty", m.pendingErr)
	}
	if m.phIdx != 1 {
		t.Fatalf("phIdx = %d, want 1", m.phIdx)
	}
}

func TestPlaceholder_AcceptAllDefaultsProducesCorrectCommand(t *testing.T) {
	m := newModel("git push", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))
	// Accept default "origin" for remote.
	m, _ = update(m, key(tea.KeyEnter))
	// Accept default "main" for branch.
	m, _ = update(m, key(tea.KeyEnter))

	if !m.quitting {
		t.Fatalf("quitting = false, want true")
	}
	if got, want := m.selected.Command, "git push origin main"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_OverrideDefault(t *testing.T) {
	m := newModel("git push", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// Override the pre-filled "origin" with "upstream".
	for range "origin" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "upstream")
	m, _ = update(m, key(tea.KeyEnter))

	// Accept default "main" for branch.
	m, _ = update(m, key(tea.KeyEnter))

	if got, want := m.selected.Command, "git push upstream main"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_EmptyValueRejectedWhenNoDefault(t *testing.T) {
	m := newModel("ssh", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// First placeholder "user" has no default - submit empty.
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode = %v, want modeFillPlaceholders to stay after empty required value", m.mode)
	}
	if m.pendingErr == "" {
		t.Fatalf("pendingErr = empty, want a non-empty validation message")
	}
	// Should still be on index 0.
	if m.phIdx != 0 {
		t.Fatalf("phIdx = %d, want 0 (did not advance)", m.phIdx)
	}
}

func TestPlaceholder_EscReturnsToList(t *testing.T) {
	m := newModel("ssh", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))
	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode = %v, want modeFillPlaceholders", m.mode)
	}

	// Type some value then Esc.
	m = typeString(m, "admin")
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode after Esc = %v, want modeList", m.mode)
	}
	if m.quitting {
		t.Fatalf("quitting = true, want false after cancel")
	}
	if m.selected.Command != "" {
		t.Fatalf("selected.Command = %q, want empty after cancel", m.selected.Command)
	}
}

func TestPlaceholder_ViewShowsPreviewWithResolvedPlaceholders(t *testing.T) {
	m := newModel("ssh", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// Type a value to see it in the preview.
	m = typeString(m, "admin")

	view := m.View().Content
	if !strings.Contains(view, "admin") {
		t.Fatalf("View() = %q, want the preview to show the typed value %q", view, "admin")
	}
	if strings.Contains(view, "{{user}}") {
		t.Fatalf("View() = %q, want the current placeholder replaced by typed text in the preview", view)
	}
	// The second placeholder should still appear.
	if !strings.Contains(view, "{{host}}") {
		t.Fatalf("View() = %q, want remaining placeholder %q to still appear", view, "{{host}}")
	}
	// Help text for non-last placeholder.
	if !strings.Contains(view, "continue") {
		t.Fatalf("View() = %q, want 'enter continue' for non-last placeholder", view)
	}
}

func TestPlaceholder_ViewLastPlaceholderShowsRunHelp(t *testing.T) {
	// Single placeholder: it's also the last one.
	st := placeholderStore(t)
	st.Set("greet", "echo hello {{name}}")
	m := newModel("greet", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	if !strings.Contains(view, "enter run") {
		t.Fatalf("View() = %q, want 'enter run' for the last placeholder", view)
	}
	if strings.Contains(view, "continue") {
		t.Fatalf("View() = %q, want 'run' not 'continue' for the last placeholder", view)
	}
}

func TestPlaceholder_ViewPreviewFollowsLiveTypingOverDefault(t *testing.T) {
	m := newModel("git push", placeholderStore(t), testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))
	// Prefill is "origin"; override by typing (after clearing).
	for range "origin" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "upstream")

	view := m.View().Content
	if !strings.Contains(view, "upstream") {
		t.Fatalf("View() = %q, want live typed text %q in the preview", view, "upstream")
	}
	// The current field's stale default must not linger in the preview.
	if strings.Contains(view, "git push origin ") {
		t.Fatalf("View() = %q, want the prefilled default replaced by live typing", view)
	}
}

func TestPlaceholder_DefaultWithSpecialChars(t *testing.T) {
	st := placeholderStore(t)
	st.Set("connect", "psql -h {{host:localhost}} -p {{port:5432}} -U {{user}}")
	m := newModel("connect", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// First placeholder "host" has default "localhost".
	if m.phVals[0] != "localhost" {
		t.Fatalf("phVals[0] = %q, want %q", m.phVals[0], "localhost")
	}

	// Accept default, move to "port".
	m, _ = update(m, key(tea.KeyEnter))
	// Accept default "5432", move to "user".
	m, _ = update(m, key(tea.KeyEnter))
	// Fill required "user".
	m = typeString(m, "postgres")
	m, _ = update(m, key(tea.KeyEnter))

	if got, want := m.selected.Command, "psql -h localhost -p 5432 -U postgres"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_SinglePlaceholderEntireCommand(t *testing.T) {
	st := placeholderStore(t)
	st.Set("greet", "{{msg}}")
	m := newModel("greet", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode = %v, want modeFillPlaceholders", m.mode)
	}
	if len(m.placeholders) != 1 {
		t.Fatalf("len(placeholders) = %d, want 1", len(m.placeholders))
	}

	// Fill and execute.
	m = typeString(m, "hello world")
	m, _ = update(m, key(tea.KeyEnter))

	if !m.quitting {
		t.Fatalf("quitting = false, want true")
	}
	if got, want := m.selected.Command, "hello world"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_DefaultWithBackslashesAndColons(t *testing.T) {
	st := placeholderStore(t)
	st.Set("docker", "docker run -v {{src:/host/path:ro}}:{{dest:/container/path}} {{image}}")
	m := newModel("docker", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	if m.phVals[0] != "/host/path:ro" {
		t.Fatalf("phVals[0] = %q, want %q", m.phVals[0], "/host/path:ro")
	}

	// Accept default for src.
	m, _ = update(m, key(tea.KeyEnter))

	if m.phVals[1] != "/container/path" {
		t.Fatalf("phVals[1] = %q, want %q", m.phVals[1], "/container/path")
	}

	// Accept default for dest.
	m, _ = update(m, key(tea.KeyEnter))
	// Fill required image.
	m = typeString(m, "node:18")
	m, _ = update(m, key(tea.KeyEnter))

	if got, want := m.selected.Command, "docker run -v /host/path:ro:/container/path node:18"; got != want {
		t.Fatalf("selected.Command = %q, want %q", got, want)
	}
}

func TestPlaceholder_ShowCommandTrueStillResolvesPlaceholders(t *testing.T) {
	cfg := testConfig(t)
	cfg.SetShowCommand(true)

	st := placeholderStore(t)
	m := newModel("ssh", st, cfg, testStyles())

	m, _ = update(m, key(tea.KeyEnter))
	if m.mode != modeFillPlaceholders {
		t.Fatalf("mode = %v, want modeFillPlaceholders even when showCommand is true", m.mode)
	}

	// Fill both placeholders.
	m = typeString(m, "admin")
	m, _ = update(m, key(tea.KeyEnter))
	m = typeString(m, "prod.example.com")
	m, _ = update(m, key(tea.KeyEnter))

	if !m.quitting {
		t.Fatalf("quitting = false, want true")
	}
	// The caller always runs the resolved command directly.
	if got, want := m.selected.Command, "ssh admin@prod.example.com"; got != want {
		t.Fatalf("selected.Command = %q, want resolved command %q", got, want)
	}
}

func TestPlaceholder_PasteInPlaceholderForm(t *testing.T) {
	st := placeholderStore(t)
	m := newModel("ssh", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// Paste with embedded newline into the placeholder form.
	m, _ = update(m, tea.PasteMsg{Content: "ad\nmin"})

	if got, want := m.form.Value(), "ad min"; got != want {
		t.Fatalf("form.Value() after paste = %q, want newlines collapsed to spaces %q", got, want)
	}
}

func TestPlaceholder_BackspaceInPlaceholderForm(t *testing.T) {
	st := placeholderStore(t)
	st.Set("git push", "git push {{remote:origin}} {{branch:main}}")
	m := newModel("git push", st, testConfig(t), testStyles())

	m, _ = update(m, key(tea.KeyEnter))

	// Default is "origin" - backspace to clear, then type new value.
	for range "origin" {
		m, _ = update(m, key(tea.KeyBackspace))
	}
	m = typeString(m, "upstream")

	if got, want := m.form.Value(), "upstream"; got != want {
		t.Fatalf("form.Value() = %q, want %q", got, want)
	}

	m, _ = update(m, key(tea.KeyEnter))
	if m.phVals[0] != "upstream" {
		t.Fatalf("phVals[0] = %q, want %q", m.phVals[0], "upstream")
	}
}
