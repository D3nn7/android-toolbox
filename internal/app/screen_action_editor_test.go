package app

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

// newTestActionEditModel sets up a Model with one custom (editable) action
// already persisted to a real temp actions.yaml, and the dashboard
// showing/selecting it - so tests can drive the "e" key exactly like a
// real user would, then exercise the editor itself.
func newTestActionEditModel(t *testing.T) Model {
	t.Helper()
	configDir := t.TempDir()
	actionsFile := filepath.Join(configDir, "actions.yaml")
	backupDir := filepath.Join(configDir, ".backup")

	custom := actions.Action{ID: "my-custom-action", Name: "My Custom Action", Description: "Does a thing", Category: "Custom", Tool: actions.ToolShell, Command: "echo hi"}
	if err := actions.Append(actionsFile, custom); err != nil {
		t.Fatal(err)
	}
	set, err := actions.Load(actionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{ConfigDir: configDir, ActionsFile: actionsFile, BackupDir: backupDir}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.dashboard.serial = "SERIAL1"
	m.actionSet = set
	dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
	m.dashboard = dash
	leftW, _ := paneWidths(m.width)
	leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
	m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))
	m.current = screenDashboard

	// Select the custom action specifically (its position among the
	// defaults isn't guaranteed to be index 0).
	for i, it := range m.dashboard.actionList.Items() {
		if it.(actionItem).a.ID == "my-custom-action" {
			m.dashboard.actionList.Select(i)
			break
		}
	}
	return m
}

func advanceActionEdit(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.updateActionEdit(msg)
	return updated.(Model)
}

// TestDashboardEKeyEntersActionEditorForEditableAction proves the entry
// point actually works end to end from the dashboard.
func TestDashboardEKeyEntersActionEditorForEditableAction(t *testing.T) {
	m := newTestActionEditModel(t)

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)

	if m.current != screenActionEdit {
		t.Fatalf("expected 'e' to enter the action editor, current = %v", m.current)
	}
	if m.actionEdit.action.ID != "my-custom-action" {
		t.Fatalf("expected the editor to open for the selected action, got %q", m.actionEdit.action.ID)
	}
}

// TestDashboardEKeyOnBuiltinShowsStatusInstead is a regression test for the
// "built-in actions stay as shipped" rule: pressing 'e' on one must not
// open the editor, and should tell the user why instead of silently doing
// nothing.
func TestDashboardEKeyOnBuiltinShowsStatusInstead(t *testing.T) {
	m := newTestActionEditModel(t)
	for i, it := range m.dashboard.actionList.Items() {
		if actions.IsBuiltinID(it.(actionItem).a.ID) {
			m.dashboard.actionList.Select(i)
			break
		}
	}

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected 'e' on a built-in action to stay on the dashboard, current = %v", m.current)
	}
	if m.dashboard.status != m.text.ActionNotEditableStatus {
		t.Fatalf("expected the not-editable status message, got %q", m.dashboard.status)
	}
}

func TestActionEditEscFromBrowsingReturnsToDashboard(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "X", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.current != screenDashboard {
		t.Fatalf("expected esc to return to the dashboard, current = %v", m.current)
	}
}

func TestActionEditCursorNavigation(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit

	if m.actionEdit.cursor != 0 {
		t.Fatalf("expected cursor to start at 0, got %d", m.actionEdit.cursor)
	}
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.actionEdit.cursor != 0 {
		t.Fatalf("expected up at the top row to stay clamped at 0, got %d", m.actionEdit.cursor)
	}
	for i := 0; i < len(actionEditFieldsOrder)+2; i++ {
		m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	want := len(actionEditFieldsOrder) - 1
	if m.actionEdit.cursor != want {
		t.Fatalf("expected down past the last row to clamp at %d, got %d", want, m.actionEdit.cursor)
	}
}

func TestActionEditEnterOnRowStartsEditingThatField(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit
	m.actionEdit.cursor = 1 // Description row

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.actionEdit.stage != actionEditEditing {
		t.Fatalf("expected enter to start editing, stage = %v", m.actionEdit.stage)
	}
	if m.actionEdit.editingField != actionEditFieldDescription {
		t.Fatalf("expected to be editing the Description field, got %v", m.actionEdit.editingField)
	}
	if m.actionEdit.editField == nil || m.actionEdit.editValue == nil {
		t.Fatal("expected an editor field and bound value to be constructed")
	}
}

func TestActionEditEscWhileEditingDiscardsChange(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing Name
	*m.actionEdit.editValue = "Changed"
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.actionEdit.stage != actionEditBrowsing {
		t.Fatalf("expected esc to return to browsing, stage = %v", m.actionEdit.stage)
	}
	if m.actionEdit.action.Name != "Original" {
		t.Fatalf("expected esc to discard the edit, Name = %q", m.actionEdit.action.Name)
	}
}

func TestActionEditSubmittingUnchangedValueSkipsConfirm(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing Name (unchanged)
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // submit without changing it

	if m.actionEdit.stage != actionEditBrowsing {
		t.Fatalf("expected an unchanged value to skip the confirm dialog, stage = %v", m.actionEdit.stage)
	}
}

// TestActionEditChangeNameAndAccept is the full happy path: edit a field,
// confirm, and verify the change is both reflected immediately and
// actually persisted to actions.yaml (with the rest of the action
// untouched).
func TestActionEditChangeNameAndAccept(t *testing.T) {
	m := newTestActionEditModel(t)
	original, ok := m.actionSet.Find("my-custom-action")
	if !ok {
		t.Fatal("expected the custom action to exist")
	}
	m.actionEdit = newActionEditScreen(original)
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing Name
	*m.actionEdit.editValue = "Renamed Action"
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // submit -> should ask for confirmation

	if m.actionEdit.stage != actionEditConfirming {
		t.Fatalf("expected submitting a real change to open a confirm dialog, stage = %v", m.actionEdit.stage)
	}
	if m.actionEdit.confirmNewValue != "Renamed Action" {
		t.Fatalf("expected the pending change to be %q, got %q", "Renamed Action", m.actionEdit.confirmNewValue)
	}

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.actionEdit.stage != actionEditBrowsing {
		t.Fatalf("expected accepting to return to browsing, stage = %v", m.actionEdit.stage)
	}
	if m.actionEdit.action.Name != "Renamed Action" {
		t.Fatalf("expected the in-memory action to be renamed, got %q", m.actionEdit.action.Name)
	}

	persisted, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := persisted.Find("my-custom-action")
	if !ok {
		t.Fatal("expected the action to still exist after the edit")
	}
	if got.Name != "Renamed Action" {
		t.Fatalf("expected the persisted Name to be updated, got %q", got.Name)
	}
	if got.Command != original.Command || got.Description != original.Description {
		t.Fatalf("expected every other field to stay untouched, got %+v", got)
	}
}

func TestActionEditChangeDeclineKeepsOriginal(t *testing.T) {
	m := newTestActionEditModel(t)
	original, ok := m.actionSet.Find("my-custom-action")
	if !ok {
		t.Fatal("expected the custom action to exist")
	}
	m.actionEdit = newActionEditScreen(original)
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	*m.actionEdit.editValue = "Should Not Stick"
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.actionEdit.action.Name != original.Name {
		t.Fatalf("expected declining to leave the name unchanged, got %q", m.actionEdit.action.Name)
	}
	persisted, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := persisted.Find("my-custom-action")
	if got.Name != original.Name {
		t.Fatalf("expected declining to never write actions.yaml, persisted Name = %q", got.Name)
	}
}

// TestActionEditNameCannotBeEmpty proves the Name field's own Validate
// rejects a blank value before the edit can even be submitted.
func TestActionEditNameCannotBeEmpty(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"})
	m.current = screenActionEdit

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing Name
	*m.actionEdit.editValue = ""
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // try to submit empty

	if m.actionEdit.stage != actionEditEditing {
		t.Fatalf("expected an empty Name to stay in editing rather than proceed to confirm, stage = %v", m.actionEdit.stage)
	}
}

// TestActionEditToggleConfirmField exercises one of the two boolean fields
// through the actual Select editor (arrow key, not just direct assignment)
// to prove its keymap works standalone - same regression class as every
// other huh.Select used outside a Form in this app.
func TestActionEditToggleConfirmField(t *testing.T) {
	m := newTestActionEditModel(t)
	m.actionEdit = newActionEditScreen(actions.Action{ID: "my-custom-action", Name: "X", Tool: actions.ToolShell, Command: "echo hi", Confirm: false})
	m.current = screenActionEdit
	m.actionEdit.cursor = 5 // Confirm row (name, description, category, tool, command, confirm, interactive)

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.actionEdit.editingField != actionEditFieldConfirm {
		t.Fatalf("expected to be editing the Confirm field, got %v", m.actionEdit.editingField)
	}

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if *m.actionEdit.editValue != "true" {
		t.Fatalf("expected the arrow key to toggle the highlighted option to true, got %q", *m.actionEdit.editValue)
	}

	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceActionEdit(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if !m.actionEdit.action.Confirm {
		t.Fatal("expected Confirm to actually be set to true after accepting")
	}
}
