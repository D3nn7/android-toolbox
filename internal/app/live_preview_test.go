package app

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

// waitForLivePreviewMsg drains cmd (and any tea.BatchMsg it produces) until
// it finds a livePreviewMsg, applying every other message to m along the
// way. Fails the test if none arrives within the timeout.
func waitForLivePreviewMsg(t *testing.T, m Model, cmd tea.Cmd) (Model, livePreviewMsg) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for cmd != nil && time.Now().Before(deadline) {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				if lp, ok := c().(livePreviewMsg); ok {
					updated, _ := m.updateDashboard(lp)
					return updated.(Model), lp
				}
			}
			return m, livePreviewMsg{}
		}
		if lp, ok := msg.(livePreviewMsg); ok {
			updated, _ := m.updateDashboard(lp)
			return updated.(Model), lp
		}
		return m, livePreviewMsg{}
	}
	t.Fatal("no livePreviewMsg produced before timeout")
	return m, livePreviewMsg{}
}

// TestLivePreviewOnlyAutoRunsEligibleActionsAndNeverExecutesOthers is the
// direct regression test for the safety requirement behind this feature:
// merely navigating (highlighting a different action) must NEVER run an
// action for real (never leaves screenDashboard, never opens the
// confirm/param/runner screens) - except for the small, explicitly-flagged
// set of read-only actions (LivePreviewEligible), which auto-run into the
// preview area only, still without leaving screenDashboard.
func TestLivePreviewOnlyAutoRunsEligibleActionsAndNeverExecutesOthers(t *testing.T) {
	liveAction := actions.Action{ID: "battery", Name: "Akku", Tool: actions.ToolShell, Command: "echo level: 87", Format: "keyvalue", LivePreview: true}
	destructiveAction := actions.Action{ID: "reboot", Name: "Neustart", Tool: actions.ToolShell, Command: "echo would-reboot", Confirm: true}
	plainAction := actions.Action{ID: "logs", Name: "Logs", Tool: actions.ToolShell, Command: "echo some-logs"}

	set := actions.ActionSet{Actions: []actions.Action{liveAction, destructiveAction, plainAction}}

	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 120, 32
	m.executor = actions.NewExecutor("adb.exe", nil)
	dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
	m.dashboard = dash
	m.current = screenDashboard

	// Selecting the live-preview action (it's first in the list already)
	// must trigger an auto-run, but must NOT change m.current.
	updated, cmd := m.syncLivePreview()
	m = updated
	if m.current != screenDashboard {
		t.Fatalf("selecting a live-preview action must not leave the dashboard, current = %v", m.current)
	}
	if !m.dashboard.livePreviewLoading || m.dashboard.livePreviewActionID != "battery" {
		t.Fatalf("expected a live-preview fetch to start for 'battery', got %+v", m.dashboard)
	}

	m, lp := waitForLivePreviewMsg(t, m, cmd)
	if lp.actionID != "battery" || lp.err != nil {
		t.Fatalf("unexpected live preview result: %+v", lp)
	}
	if m.dashboard.livePreviewLoading {
		t.Fatal("expected loading to be false once the result arrived")
	}
	if got := m.renderOutputLine(liveAction.Format, "level: 87"); got == "" {
		t.Fatal("sanity check on renderOutputLine failed")
	}
	if m.dashboard.livePreviewOutput == "" {
		t.Fatal("expected live preview output to be populated")
	}

	// Now move selection to the destructive (Confirm: true) action. Down()
	// on the list simulates arrow-key navigation exactly as updateDashboard
	// would apply it.
	m.dashboard.actionList.CursorDown()
	updated, cmd = m.syncLivePreview()
	m = updated
	if m.current != screenDashboard {
		t.Fatalf("navigating onto a destructive action must never execute it - current = %v", m.current)
	}
	if cmd != nil {
		t.Fatal("navigating onto a non-live-preview action must not start any command")
	}
	if m.dashboard.livePreviewActionID != "" {
		t.Fatalf("expected live preview state to clear for an ineligible action, got %+v", m.dashboard)
	}

	// And the plain (no confirm, but also not flagged LivePreview) action:
	// same story - must not auto-run either.
	m.dashboard.actionList.CursorDown()
	updated, cmd = m.syncLivePreview()
	m = updated
	if m.current != screenDashboard || cmd != nil {
		t.Fatalf("a plain action without LivePreview must never auto-run, current=%v cmd-is-nil=%v", m.current, cmd == nil)
	}
}
