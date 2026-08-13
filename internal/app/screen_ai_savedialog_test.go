package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

// newTestAIPreviewModel sets up the AI screen exactly as aiDraftMsg's
// handler would after a successful generation: stage aiStagePreview, a
// draft action, and its save/discard dialog already constructed. Backed by
// a real temp actions.yaml/backup dir so accepting the dialog can safely
// exercise the real actions.Append/backup.BeforeWrite write path.
func newTestAIPreviewModel(t *testing.T) Model {
	t.Helper()
	configDir := t.TempDir()
	backupDir := t.TempDir()
	actionsFile := filepath.Join(configDir, "actions.yaml")
	if err := os.WriteFile(actionsFile, actions.DefaultActionsYAML, 0o644); err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{ConfigDir: configDir, BackupDir: backupDir, ActionsFile: actionsFile}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.ai = newAIScreen(m.text)
	m.ai.stage = aiStagePreview
	m.ai.draft = actions.Action{ID: "ki-test-aktion", Name: "KI Testaktion", Tool: actions.ToolShell, Command: "echo hi"}
	dialog, answer := newSaveActionDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), false)
	m.ai.saveDialog = dialog
	m.ai.saveAnswer = answer
	m.current = screenAI
	return m
}

func runUpdateAI(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, cmd := m.updateAI(msg)
	m = updated.(Model)
	if cmd != nil {
		if next := cmd(); next != nil {
			updated, _ = m.updateAI(next)
			m = updated.(Model)
		}
	}
	return m
}

func TestAISaveDialogDeclineDoesNotWriteAction(t *testing.T) {
	m := newTestAIPreviewModel(t)

	m = runUpdateAI(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.current != screenDashboard {
		t.Fatalf("expected 'n' to return to the dashboard, current = %v", m.current)
	}
	data, err := os.ReadFile(m.paths.ActionsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(actions.DefaultActionsYAML) {
		t.Fatal("expected declining to leave actions.yaml untouched")
	}
}

func TestAISaveDialogEscDiscardsWithoutWriting(t *testing.T) {
	m := newTestAIPreviewModel(t)

	updated, _ := m.updateAI(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected esc to return to the dashboard, current = %v", m.current)
	}
}

func TestAISaveDialogRKeyReturnsToInputEvenWhileDialogFocused(t *testing.T) {
	m := newTestAIPreviewModel(t)

	updated, cmd := m.updateAI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(Model)

	if m.ai.stage != aiStageInput {
		t.Fatalf("expected 'r' to return to aiStageInput, got %v", m.ai.stage)
	}
	if cmd == nil {
		t.Fatal("expected 'r' to refocus the textarea via a command")
	}
}

// TestAISaveDialogArrowToggleThenEnterSaves is the same regression class as
// the other three dialogs' arrow-toggle tests: without
// WithKeyMap(huh.NewDefaultKeyMap()), a standalone huh.Confirm's
// toggle/submit keys silently do nothing.
func TestAISaveDialogArrowToggleThenEnterSaves(t *testing.T) {
	m := newTestAIPreviewModel(t)

	updated, _ := m.updateAI(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if !*m.ai.saveAnswer {
		t.Fatal("expected the left/right toggle key to flip the highlighted button to Yes")
	}

	m = runUpdateAI(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.ai.saveErr != nil {
		t.Fatalf("unexpected save error: %v", m.ai.saveErr)
	}
	if m.ai.stage != aiStageSaved {
		t.Fatalf("expected accepting the dialog to save and reach aiStageSaved, got %v", m.ai.stage)
	}
	set, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Find(m.ai.draft.ID); !ok {
		t.Fatalf("expected %q to be appended to actions.yaml", m.ai.draft.ID)
	}
}
