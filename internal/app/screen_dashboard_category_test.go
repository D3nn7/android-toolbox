package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

func newTestCategoryModel(t *testing.T) Model {
	t.Helper()
	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "geraet-1", Name: "Geraet Info", Category: "Geraet", Tool: actions.ToolADB, Command: "shell true"},
		{ID: "logs-1", Name: "Logcat", Category: "Logs", Tool: actions.ToolADB, Command: "logcat"},
		{ID: "logs-2", Name: "Logcat leeren", Category: "Logs", Tool: actions.ToolADB, Command: "logcat -c"},
	}}
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 120, 32
	dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
	m.dashboard = dash
	leftW, _ := paneWidths(m.width)
	leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
	m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))
	m.current = screenDashboard
	return m
}

// TestDashboardStartsWithAllCategoriesShown is a baseline check that the
// default "Alle" pill (categories[0] == "") really does show every action
// unfiltered, matching how the list looked before category pills existed.
func TestDashboardStartsWithAllCategoriesShown(t *testing.T) {
	m := newTestCategoryModel(t)

	if got, want := m.dashboard.categoryIdx, 0; got != want {
		t.Fatalf("categoryIdx = %d, want %d (Alle)", got, want)
	}
	if got, want := len(m.dashboard.actionList.Items()), 3; got != want {
		t.Fatalf("expected all 3 actions visible under Alle, got %d", got)
	}
}

// TestTabCyclesToNextCategoryAndFiltersTheList is the core of the feature:
// pressing tab moves off "Alle" onto the first real category and the action
// list narrows down to just that category's actions.
func TestTabCyclesToNextCategoryAndFiltersTheList(t *testing.T) {
	m := newTestCategoryModel(t)

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	if got, want := m.dashboard.categories[m.dashboard.categoryIdx], "Geraet"; got != want {
		t.Fatalf("expected tab to select category %q, got %q", want, got)
	}
	items := m.dashboard.actionList.Items()
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 action in category Geraet, got %d", len(items))
	}
	if got := items[0].(actionItem).a.ID; got != "geraet-1" {
		t.Fatalf("expected geraet-1, got %q", got)
	}
}

// TestShiftTabCyclesBackwardAndWrapsAround exercises the wrap-around in both
// directions: from "Alle" (index 0), shift+tab must wrap to the *last*
// category rather than going negative or doing nothing.
func TestShiftTabCyclesBackwardAndWrapsAround(t *testing.T) {
	m := newTestCategoryModel(t)

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)

	lastIdx := len(m.dashboard.categories) - 1
	if m.dashboard.categoryIdx != lastIdx {
		t.Fatalf("expected shift+tab from index 0 to wrap to the last category (%d), got %d", lastIdx, m.dashboard.categoryIdx)
	}
	if got, want := m.dashboard.categories[m.dashboard.categoryIdx], "Logs"; got != want {
		t.Fatalf("expected the wrapped-to category to be %q, got %q", want, got)
	}
}

// TestCyclingThroughAllCategoriesReturnsToAlle proves the forward cycle is a
// full loop: tabbing once per category eventually lands back on "Alle" with
// every action visible again.
func TestCyclingThroughAllCategoriesReturnsToAlle(t *testing.T) {
	m := newTestCategoryModel(t)
	n := len(m.dashboard.categories)

	for i := 0; i < n; i++ {
		updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
	}

	if m.dashboard.categoryIdx != 0 {
		t.Fatalf("expected a full cycle (%d tabs) to land back on Alle (index 0), got %d", n, m.dashboard.categoryIdx)
	}
	if got, want := len(m.dashboard.actionList.Items()), 3; got != want {
		t.Fatalf("expected all 3 actions visible again after a full cycle, got %d", got)
	}
}
