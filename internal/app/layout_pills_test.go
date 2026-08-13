package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func newPillsTestModel(categories []string, selectedIdx int) Model {
	m := Model{styles: newStyles(), text: uiTextEN}
	m.dashboard.categories = categories
	m.dashboard.categoryIdx = selectedIdx
	return m
}

// TestRenderCategoryPillsFitsEverythingWhenThereIsRoom is the baseline: with
// enough width, every category pill (plus "Alle") shows, no "…" needed.
func TestRenderCategoryPillsFitsEverythingWhenThereIsRoom(t *testing.T) {
	m := newPillsTestModel([]string{"", "Geraet", "Logs", "WLAN"}, 0)

	row := m.renderCategoryPills(100)

	for _, want := range []string{m.text.CategoryAll, "Geraet", "Logs", "WLAN"} {
		if !strings.Contains(row, want) {
			t.Fatalf("expected pill row to contain %q, got %q", want, row)
		}
	}
	if strings.Contains(row, "…") {
		t.Fatalf("did not expect an ellipsis when everything fits, got %q", row)
	}
}

// TestRenderCategoryPillsNeverExceedsWidth is a regression test for the
// original bug report: with many categories and a narrow pane, the row must
// never render wider than the space it was given - lipgloss would otherwise
// let it overflow into (or past) the pane border.
func TestRenderCategoryPillsNeverExceedsWidth(t *testing.T) {
	categories := []string{"", "Geraet", "Logs", "WLAN", "Anzeige", "Dateien", "Entwicklung", "Diagnose", "Netzwerk"}
	const width = 30

	for selected := 0; selected < len(categories); selected++ {
		m := newPillsTestModel(categories, selected)
		row := m.renderCategoryPills(width)
		if w := lipgloss.Width(row); w > width {
			t.Fatalf("selected=%d: pill row is %d wide, want <= %d: %q", selected, w, width, row)
		}
	}
}

// TestRenderCategoryPillsAlwaysShowsTheSelectedCategory is the actual "not
// sinnvoll" bug: previously a plain trailing-truncate could cut the row off
// before ever reaching the selected pill once its index was scrolled past
// the visible window, making it look like the category vanished. Whichever
// category is selected must always appear in the rendered row, however
// narrow the pane.
func TestRenderCategoryPillsAlwaysShowsTheSelectedCategory(t *testing.T) {
	categories := []string{"", "Geraet", "Logs", "WLAN", "Anzeige", "Dateien", "Entwicklung", "Diagnose", "Netzwerk"}
	const width = 25

	for selected, cat := range categories {
		m := newPillsTestModel(categories, selected)
		label := categoryDisplayLabel(cat, m.text)
		row := m.renderCategoryPills(width)
		if !strings.Contains(row, label) {
			t.Fatalf("selected=%d (%q): expected the selected category to always be visible, got %q", selected, label, row)
		}
	}
}

// TestRenderCategoryPillsShowsEllipsisWhenCategoriesAreHidden checks the
// scroll indicators: an ellipsis on the right when later categories are cut
// off, and one on the left after scrolling far enough that earlier ones are.
func TestRenderCategoryPillsShowsEllipsisWhenCategoriesAreHidden(t *testing.T) {
	categories := []string{"", "Geraet", "Logs", "WLAN", "Anzeige", "Dateien", "Entwicklung", "Diagnose", "Netzwerk"}
	const width = 25

	atStart := newPillsTestModel(categories, 0)
	rowAtStart := atStart.renderCategoryPills(width)
	if strings.HasPrefix(strings.TrimSpace(rowAtStart), "…") {
		t.Fatalf("did not expect a left ellipsis when the first category is selected, got %q", rowAtStart)
	}
	if !strings.Contains(rowAtStart, "…") {
		t.Fatalf("expected a right ellipsis since later categories don't fit, got %q", rowAtStart)
	}

	atEnd := newPillsTestModel(categories, len(categories)-1)
	rowAtEnd := atEnd.renderCategoryPills(width)
	if strings.HasSuffix(strings.TrimSpace(stripANSI(rowAtEnd)), "…") {
		t.Fatalf("did not expect a right ellipsis when the last category is selected, got %q", rowAtEnd)
	}
	if !strings.Contains(rowAtEnd, "…") {
		t.Fatalf("expected a left ellipsis since earlier categories don't fit, got %q", rowAtEnd)
	}
}
