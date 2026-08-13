package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/ai"
	"android-toolbox/internal/config"
)

// fakeAIProvider is a minimal ai.Provider stub so submitAIRequest can be
// exercised without shelling out to a real AI CLI.
type fakeAIProvider struct {
	draft   ai.ActionDraft
	err     error
	lastReq ai.GenerateRequest
}

func (f *fakeAIProvider) Name() string     { return "fake" }
func (f *fakeAIProvider) Available() error { return nil }
func (f *fakeAIProvider) GenerateAction(_ context.Context, req ai.GenerateRequest) (ai.ActionDraft, error) {
	f.lastReq = req
	return f.draft, f.err
}

// flattenBatchCmd invokes cmd and, if it produced a tea.BatchMsg (as
// tea.Batch's returned Cmd does whenever more than one sub-command was
// given, which is always true here: submitAIRequest batches the generate
// call with the spinner tick), runs every batched sub-command too so tests
// can find the aiDraftMsg among them - exactly what the real bubbletea
// runtime would eventually deliver to Update, just synchronously.
func flattenBatchCmd(cmd tea.Cmd) []tea.Msg {
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			if c == nil {
				continue
			}
			msgs = append(msgs, c())
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func newTestActionEditorModel(t *testing.T, existing actions.Action, provider ai.Provider) Model {
	t.Helper()
	configDir := t.TempDir()
	actionsFile := filepath.Join(configDir, "actions.yaml")
	if err := actions.Append(actionsFile, existing); err != nil {
		t.Fatal(err)
	}
	set, err := actions.Load(actionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{ConfigDir: configDir, ActionsFile: actionsFile, BackupDir: filepath.Join(configDir, ".backup")}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.actionSet = set
	m.aiProvider = provider
	m.actionEdit = newActionEditScreen(existing)
	m.current = screenActionEdit
	return m
}

// newTestAIEditPreviewModel sets up the AI screen exactly as aiDraftMsg's
// handler would after a successful edit-mode generation: stage
// aiStagePreview, editingAction pointing at the original action, and the
// save/discard dialog already constructed. Backed by a real temp
// actions.yaml so accepting can exercise the real actions.Update path.
func newTestAIEditPreviewModel(t *testing.T, existing actions.Action, draft actions.Action) Model {
	t.Helper()
	m := newTestActionEditorModel(t, existing, &fakeAIProvider{})
	m.ai = newAIEditScreen(m.text, existing)
	m.ai.stage = aiStagePreview
	m.ai.draft = draft
	dialog, answer := newSaveActionDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), true)
	m.ai.saveDialog = dialog
	m.ai.saveAnswer = answer
	m.current = screenAI
	return m
}

func TestActionEditorAKeyOpensAIEditScreenForCurrentAction(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"}
	m := newTestActionEditorModel(t, existing, &fakeAIProvider{})

	updated, cmd := m.updateActionEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)

	if m.current != screenAI {
		t.Fatalf("expected 'a' to open the AI screen, current = %v", m.current)
	}
	if !m.ai.isEditing() {
		t.Fatal("expected the AI screen to be in editing mode")
	}
	if m.ai.editingAction.ID != existing.ID {
		t.Fatalf("expected the AI screen to carry the current action as edit context, got %+v", m.ai.editingAction)
	}
	if cmd == nil {
		t.Fatal("expected the textarea to be focused via a command")
	}
}

func TestViewAIEditShowsEditModeTitle(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "X", Tool: actions.ToolShell, Command: "echo hi"}
	m := newTestActionEditorModel(t, existing, &fakeAIProvider{})
	m.ai = newAIEditScreen(m.text, existing)
	m.current = screenAI

	view := m.viewAI()
	if !strings.Contains(view, m.text.AIEditTitle) {
		t.Fatalf("expected the edit-mode title %q in the view, got: %s", m.text.AIEditTitle, view)
	}
	if strings.Contains(view, m.text.AITitle) {
		t.Fatalf("expected the create-mode title to NOT appear in edit mode, got: %s", view)
	}
}

// TestAIEditSubmitForcesOriginalActionIDAndPassesEditContext is the safety
// net for the one rule that must never break: no matter what id the AI
// returns, an edit can never change - or collide with - the action's own
// id, since the whole point is to update the SAME action in place.
func TestAIEditSubmitForcesOriginalActionIDAndPassesEditContext(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"}
	provider := &fakeAIProvider{draft: ai.ActionDraft{ID: "some-other-id", Name: "Renamed", Tool: "shell", Command: "echo new"}}
	m := newTestActionEditorModel(t, existing, provider)
	m.ai = newAIEditScreen(m.text, existing)
	m.ai.textarea.Focus()
	m.ai.textarea.SetValue("make it better")
	m.current = screenAI

	_, cmd := m.updateAI(tea.KeyMsg{Type: tea.KeyCtrlD})
	if cmd == nil {
		t.Fatal("expected ctrl+d to return a command")
	}

	var draftMsg *aiDraftMsg
	for _, msg := range flattenBatchCmd(cmd) {
		if d, ok := msg.(aiDraftMsg); ok {
			draftMsg = &d
		}
	}
	if draftMsg == nil {
		t.Fatal("expected an aiDraftMsg among the batched commands")
	}
	if draftMsg.err != nil {
		t.Fatalf("unexpected error: %v", draftMsg.err)
	}
	if draftMsg.action.ID != existing.ID {
		t.Fatalf("expected the AI's returned id %q to be overridden with the original %q", "some-other-id", existing.ID)
	}
	if provider.lastReq.ExistingAction == nil {
		t.Fatal("expected the provider to receive the existing action as edit context")
	}
	if provider.lastReq.ExistingAction.ID != existing.ID {
		t.Fatalf("expected the edit context's id to be %q, got %q", existing.ID, provider.lastReq.ExistingAction.ID)
	}
}

func TestAIEditAcceptPersistsInPlaceAndReturnsToActionEditor(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "Original", Description: "orig desc", Tool: actions.ToolShell, Command: "echo hi"}
	updatedDraft := actions.Action{ID: "my-custom-action", Name: "Updated Name", Description: "orig desc", Tool: actions.ToolShell, Command: "echo hi"}
	m := newTestAIEditPreviewModel(t, existing, updatedDraft)

	m = runUpdateAI(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.ai.stage != aiStageSaved {
		t.Fatalf("expected accepting to reach aiStageSaved, got %v", m.ai.stage)
	}

	m = runUpdateAI(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.current != screenActionEdit {
		t.Fatalf("expected leaving the saved screen to return to the action editor, current = %v", m.current)
	}
	if m.actionEdit.action.Name != "Updated Name" {
		t.Fatalf("expected the action editor to reflect the freshly saved action, got %q", m.actionEdit.action.Name)
	}

	persisted, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := persisted.Find("my-custom-action")
	if !ok {
		t.Fatal("expected the action to still exist under its original id")
	}
	if got.Name != "Updated Name" {
		t.Fatalf("expected the persisted action to be updated, got %q", got.Name)
	}
	count := 0
	for _, a := range persisted.Actions {
		if a.ID == "my-custom-action" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected actions.Update to replace in place rather than append, found %d actions with id %q", count, "my-custom-action")
	}
}

func TestAIEditDeclineReturnsToActionEditorUnchanged(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"}
	draft := actions.Action{ID: "my-custom-action", Name: "Should Not Persist", Tool: actions.ToolShell, Command: "echo hi"}
	m := newTestAIEditPreviewModel(t, existing, draft)

	m = runUpdateAI(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.current != screenActionEdit {
		t.Fatalf("expected declining to return to the action editor, current = %v", m.current)
	}
	if m.actionEdit.action.Name != "Original" {
		t.Fatalf("expected the action editor to show the original, unmodified action, got %q", m.actionEdit.action.Name)
	}

	persisted, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := persisted.Find("my-custom-action")
	if got.Name != "Original" {
		t.Fatalf("expected declining to never write actions.yaml, got %q", got.Name)
	}
}

func TestAIEditEscFromInputReturnsToActionEditor(t *testing.T) {
	existing := actions.Action{ID: "my-custom-action", Name: "Original", Tool: actions.ToolShell, Command: "echo hi"}
	m := newTestActionEditorModel(t, existing, &fakeAIProvider{})
	m.ai = newAIEditScreen(m.text, existing)
	m.current = screenAI

	updated, _ := m.updateAI(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenActionEdit {
		t.Fatalf("expected esc to return to the action editor, current = %v", m.current)
	}
	if m.actionEdit.action.ID != existing.ID {
		t.Fatalf("expected the action editor to still show the same action, got %q", m.actionEdit.action.ID)
	}
}
