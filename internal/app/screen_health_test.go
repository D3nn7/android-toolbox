package app

import (
	"context"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/config"
)

func newTestHealthInstallPromptModel(t *testing.T) Model {
	t.Helper()
	paths := config.Paths{StateFile: filepath.Join(t.TempDir(), "state.json")}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.health.done = true
	dialog, answer := newInstallPromptDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), "atbx")
	m.health.askInstall = true
	m.health.installDialog = dialog
	m.health.installAnswer = answer
	m.current = screenHealthcheck
	return m
}

// TestInstallPromptDeclineMarksFirstRunCompleteWithoutInstalling makes sure
// declining the first-run PATH prompt never runs install.Install (which
// would actually touch the real machine's registry PATH and copy files -
// far too destructive to risk exercising the "yes" path in a unit test) and
// still records that first-run has been handled so the prompt isn't asked
// again.
func TestInstallPromptDeclineMarksFirstRunCompleteWithoutInstalling(t *testing.T) {
	m := newTestHealthInstallPromptModel(t)

	updated, _ := m.updateInstallPrompt(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)

	if m.health.askInstall {
		t.Fatal("expected 'n' to dismiss the install prompt")
	}
	if m.health.installOutcome != nil || m.health.installErr != nil {
		t.Fatal("expected 'n' to skip install.Install entirely")
	}
	if m.state.IsFirstRun() {
		t.Fatal("expected declining to still mark first-run complete")
	}
}

func TestInstallPromptEscDeclinesWithoutInstalling(t *testing.T) {
	m := newTestHealthInstallPromptModel(t)

	updated, _ := m.updateInstallPrompt(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.health.askInstall {
		t.Fatal("expected esc to dismiss the install prompt")
	}
	if m.health.installOutcome != nil || m.health.installErr != nil {
		t.Fatal("expected esc to skip install.Install entirely")
	}
	if m.state.IsFirstRun() {
		t.Fatal("expected esc to still mark first-run complete")
	}
}

// TestInstallPromptArrowToggleFlipsAnswer is the same regression class as
// screen_confirm_test.go's TestConfirmDialogArrowToggleThenEnterAccepts:
// huh.NewConfirm used standalone (outside a huh.Form) never gets its keymap
// populated unless WithKeyMap(huh.NewDefaultKeyMap()) is set, so without it
// the arrow/toggle keys silently do nothing. This stops short of pressing
// enter afterward - unlike the confirm-action dialog, accepting here would
// call the real install.Install and mutate this machine's PATH registry
// key, which a unit test must never do.
func TestInstallPromptArrowToggleFlipsAnswer(t *testing.T) {
	m := newTestHealthInstallPromptModel(t)

	updated, _ := m.updateInstallPrompt(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)

	if !*m.health.installAnswer {
		t.Fatal("expected the left/right toggle key to flip the highlighted button to Yes")
	}
	if !m.health.askInstall {
		t.Fatal("toggling must not itself submit the dialog")
	}
}
