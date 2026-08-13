package app

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestAndroidListStylesOverrideStockCharmColors is a regression test for the
// reported "still very purple" bug: bubbles/list's own defaults
// (list.NewDefaultItemStyles / list.DefaultStyles) ship a pink/purple
// selected-item and title-bar color untouched by the rest of the Android
// theming work, since neither is routed through styles.go's own color vars.
// Pinning the overridden values here (rather than just "not the stock
// value") makes a future accidental revert to list.NewDefaultItemStyles()/
// list.DefaultStyles() fail loudly instead of silently reintroducing pink.
func TestAndroidListStylesOverrideStockCharmColors(t *testing.T) {
	itemStyles := androidListItemStyles()

	if fg := itemStyles.SelectedTitle.GetForeground(); fg == lipgloss.Color("#EE6FF8") {
		t.Fatal("expected SelectedTitle to no longer use stock Charm pink")
	}
	if got, want := itemStyles.SelectedTitle.GetForeground(), lipgloss.TerminalColor(colorAndroidGreen); got != want {
		t.Fatalf("SelectedTitle foreground = %v, want colorAndroidGreen (%v)", got, want)
	}
	if got, want := itemStyles.SelectedTitle.GetBorderLeftForeground(), lipgloss.TerminalColor(colorAndroidGreen); got != want {
		t.Fatalf("SelectedTitle border foreground = %v, want colorAndroidGreen (%v)", got, want)
	}

	listStyles := androidListStyles()
	if bg := listStyles.Title.GetBackground(); bg == lipgloss.Color("62") {
		t.Fatal("expected the list title bar to no longer use stock Charm color 62")
	}
}
