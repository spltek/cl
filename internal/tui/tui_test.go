package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
	m := newModel("", testStore(t), testStyles())

	if len(m.filtered) != 3 {
		t.Fatalf("filtered len = %d, want 3", len(m.filtered))
	}
}

func TestNewModel_FilterNarrowsByName(t *testing.T) {
	m := newModel("bui", testStore(t), testStyles())

	if len(m.filtered) != 1 || m.filtered[0].Name != "build" {
		t.Fatalf("filtered = %+v, want just %q", m.filtered, "build")
	}
}

func TestUpdate_ArrowKeysMoveCursorWithinBounds(t *testing.T) {
	m := newModel("", testStore(t), testStyles())

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
	m := newModel("clean", testStore(t), testStyles())

	m, cmd := update(m, key(tea.KeyEnter))

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
	m := newModel("build", testStore(t), testStyles())

	m, cmd := update(m, key(tea.KeyEsc))

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
	m := newModel("", testStore(t), testStyles())

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
	m := newModel("", st, testStyles())

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
	m := newModel("", st, testStyles())

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
	m := newModel("", st, testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "my build")
	m, _ = update(m, key(tea.KeyEnter))

	if m.mode != modeAddValue {
		t.Fatalf("mode = %v, want modeAddValue after a name containing spaces (pendingErr=%q)", m.mode, m.pendingErr)
	}
}

func TestAdd_EmptyNameShowsErrorAndStaysInAddName(t *testing.T) {
	m := newModel("", testStore(t), testStyles())

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
	m := newModel("", testStore(t), testStyles())

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
	m := newModel("", st, testStyles())

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
	m := newModel("", testStore(t), testStyles())

	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "whatever")
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc", m.mode)
	}
}

func TestAdd_EscFromAddValueDiscardsWithoutSaving(t *testing.T) {
	st := testStore(t)
	m := newModel("", st, testStyles())

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
	m := newModel("build", testStore(t), testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", testStore(t), testStyles())

	m, _ = update(m, ctrlKey('e'))
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList directly after esc, without a confirmation step", m.mode)
	}
}

func TestEdit_EmptyValueShowsErrorAndStaysInEditValue(t *testing.T) {
	st := testStore(t)
	m := newModel("build", st, testStyles())

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
	m := newModel("does-not-exist", testStore(t), testStyles())
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
	m := newModel("build", testStore(t), testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", testStore(t), testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", testStore(t), testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("does-not-exist", testStore(t), testStyles())
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
	m := newModel("build", testStore(t), testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", st, testStyles())

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
	m := newModel("build", st, testStyles())

	m, _ = update(m, ctrlKey('d'))
	m, _ = update(m, key(tea.KeyEsc))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList after esc", m.mode)
	}
	if _, ok := st.Get("build"); !ok {
		t.Fatalf("store.Get(build) missing, want it to still exist after esc")
	}
}

// --- View rendering ---
//
// These don't need a real TTY: View() is a pure function of the
// model's state, so we can assert on its output the same way we
// assert on any other field after driving Update() with synthetic
// key messages.

func TestView_Init_ReturnsNil(t *testing.T) {
	m := newModel("", testStore(t), testStyles())
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init() = %v, want nil", cmd)
	}
}

func TestView_QuittingRendersEmptyString(t *testing.T) {
	m := newModel("build", testStore(t), testStyles())
	m, _ = update(m, key(tea.KeyEnter))

	if got := m.View().Content; got != "" {
		t.Fatalf("View() after quitting = %q, want empty", got)
	}
}

func TestView_ListShowsEntriesAndHelpWithoutCommands(t *testing.T) {
	m := newModel("", testStore(t), testStyles())

	view := m.View().Content
	for _, want := range []string{
		"backup", "build", "cleanup",
		"ctrl+a", "add", "ctrl+e", "edit", "ctrl+r", "rename", "ctrl+d", "delete",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestView_EmptyListOnlyMentionsAdd(t *testing.T) {
	m := newModel("", emptyStore(t), testStyles())

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
	m := newModel("", testStore(t), testStyles())
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
	m := newModel("", testStore(t), testStyles())
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "build")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	if !strings.Contains(view, "already used") {
		t.Errorf("View() = %q, want it to show the duplicate-name error inline", view)
	}
}

func TestView_AddValueShowsPendingName(t *testing.T) {
	m := newModel("", testStore(t), testStyles())
	m, _ = update(m, ctrlKey('a'))
	m = typeString(m, "deploy")
	m, _ = update(m, key(tea.KeyEnter))

	view := m.View().Content
	if !strings.Contains(view, "deploy") {
		t.Errorf("View() = %q, want it to mention the pending name %q", view, "deploy")
	}
}

func TestView_EditValueShowsTargetName(t *testing.T) {
	m := newModel("build", testStore(t), testStyles())
	m, _ = update(m, ctrlKey('e'))

	view := m.View().Content
	if !strings.Contains(view, "build") {
		t.Errorf("View() = %q, want it to mention the entry being edited", view)
	}
}

func TestView_ConfirmSaveEditShowsOldNameAndNewValue(t *testing.T) {
	m := newModel("build", testStore(t), testStyles())
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
	m := newModel("build", testStore(t), testStyles())
	m, _ = update(m, ctrlKey('r'))

	view := m.View().Content
	if !strings.Contains(view, "Rename") || !strings.Contains(view, "build") {
		t.Errorf("View() = %q, want it to mention renaming the entry", view)
	}
}

func TestView_ConfirmRenameShowsOldAndNewName(t *testing.T) {
	m := newModel("build", testStore(t), testStyles())
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
	m := newModel("build", testStore(t), testStyles())
	m, _ = update(m, ctrlKey('d'))

	view := m.View().Content
	for _, want := range []string{"build", "npm run build", "[y/N]"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() = %q, want it to contain %q", view, want)
		}
	}
}

func TestDelete_CtrlDOnEmptyFilteredListIsNoop(t *testing.T) {
	m := newModel("does-not-exist", testStore(t), testStyles())
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}

	m, _ = update(m, ctrlKey('d'))

	if m.mode != modeList {
		t.Fatalf("mode = %v, want modeList to stay when there is nothing to remove", m.mode)
	}
}
