package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// TestDashboardXKeyDismissesUpdateNotice and
// TestHealthcheckXKeyDismissesUpdateNotice cover the "x" dismiss key on the
// two screens that render the update banner - each just needs to prove the
// key reaches Model.updateNoticeDismissed, since renderUpdateNotice's own
// behavior once dismissed is already covered by
// TestRenderUpdateNoticeRespectsDismissal (toolsupdate_check_test.go).
func TestDashboardXKeyDismissesUpdateNotice(t *testing.T) {
	m := Model{text: uiTextEN, styles: newStyles(), current: screenDashboard}
	m.dashboard.actionList = list.New(nil, newWrappingDelegate(3), 0, 0)
	m.latestKnownVersion = "999.0.0"

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)

	if !m.updateNoticeDismissed {
		t.Fatal("expected 'x' on the dashboard to dismiss the update notice")
	}
}

func TestHealthcheckXKeyDismissesUpdateNotice(t *testing.T) {
	m := Model{text: uiTextEN, styles: newStyles(), current: screenHealthcheck}
	m.health.done = true
	m.latestKnownVersion = "999.0.0"

	updated, _ := m.updateHealthcheck(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)

	if !m.updateNoticeDismissed {
		t.Fatal("expected 'x' on the healthcheck screen to dismiss the update notice")
	}
}
