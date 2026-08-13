package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

func newTestConfirmModel(a actions.Action) Model {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.executor = actions.NewExecutor("adb.exe", nil)
	m.dashboard.serial = "SERIAL1"
	m.confirm = newConfirmScreen(a, nil, m.text, m.huhTheme, m.rightPaneContentWidth())
	m.current = screenConfirm
	return m
}

// TestConfirmDialogAcceptRunsTheAction is a regression test for the
// pointer-aliasing hazard called out in confirmScreen's doc comment: the
// huh.Confirm dialog's bound result must still be readable correctly after
// confirmScreen has been copied into Model.confirm (which happens on every
// value-receiver Update call), not just immediately after construction.
func TestConfirmDialogAcceptRunsTheAction(t *testing.T) {
	action := actions.Action{ID: "a1", Name: "Testaktion", Tool: actions.ToolShell, Command: "echo hi", Confirm: true}
	m := newTestConfirmModel(action)

	updated, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)

	if m.current != screenRunner {
		t.Fatalf("expected 'y' to accept and start the action (screenRunner), current = %v", m.current)
	}
}

func TestConfirmDialogRejectReturnsToDashboardWithoutRunning(t *testing.T) {
	action := actions.Action{ID: "a1", Name: "Testaktion", Tool: actions.ToolShell, Command: "echo hi", Confirm: true}
	m := newTestConfirmModel(action)

	updated, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected 'n' to decline and return to the dashboard, current = %v", m.current)
	}
}

func TestConfirmDialogEscCancelsWithoutRunning(t *testing.T) {
	action := actions.Action{ID: "a1", Name: "Testaktion", Tool: actions.ToolShell, Command: "echo hi", Confirm: true}
	m := newTestConfirmModel(action)

	updated, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected esc to cancel and return to the dashboard, current = %v", m.current)
	}
}

// TestConfirmDialogTogglingThenEnterRespectsTheHighlightedButton exercises
// the arrow-key toggle path (not just the y/n shortcuts) to make sure
// navigating to "No" and pressing enter doesn't run the action - the
// underlying reason the *bool-pointer pattern exists is precisely so this
// keeps working after confirmScreen gets copied around.
func TestConfirmDialogTogglingThenEnterRespectsTheHighlightedButton(t *testing.T) {
	action := actions.Action{ID: "a1", Name: "Testaktion", Tool: actions.ToolShell, Command: "echo hi", Confirm: true}
	m := newTestConfirmModel(action)

	// Default highlighted button for a fresh confirm is "No" (accessor
	// starts false) - pressing enter immediately must decline, not run.
	updated, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.current != screenDashboard {
		t.Fatalf("expected enter on the default (No) selection to decline, current = %v", m.current)
	}
}

// TestConfirmDialogArrowToggleThenEnterAccepts exercises the actual
// left/right toggle keys (not the y/n shortcuts) followed by enter,
// matching how someone would navigate the dialog with arrow keys - the
// scenario that first exposed the missing-keymap bug (huh.NewConfirm used
// standalone never gets its internal keymap populated, so Toggle/Submit
// silently did nothing until WithKeyMap(huh.NewDefaultKeyMap()) was added).
func TestConfirmDialogArrowToggleThenEnterAccepts(t *testing.T) {
	action := actions.Action{ID: "a1", Name: "Testaktion", Tool: actions.ToolShell, Command: "echo hi", Confirm: true}
	m := newTestConfirmModel(action)

	updated, _ := m.updateConfirm(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if !*m.confirm.result {
		t.Fatal("expected the left/right toggle key to flip the highlighted button to Yes")
	}

	updated, _ = m.updateConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.current != screenRunner {
		t.Fatalf("expected enter after toggling to Yes to run the action, current = %v", m.current)
	}
}
