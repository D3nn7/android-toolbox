package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// wrappingDelegate is a bubbles/list.ItemDelegate for list.DefaultItem
// values that word-wraps the description across multiple lines instead of
// bubbles' own DefaultDelegate, which hard-truncates it to a single line
// with an ellipsis - real action descriptions (e.g. "[Anzeige] Nutzt das
// Gerät als reines USB-HID Ziel ohne Bildspiegelung") routinely exceed the
// action list's pane width, so nearly every entry lost its ending.
//
// Every item still occupies a *fixed* number of lines (bubbles/list expects
// one uniform Height() across all items for its pagination math), so
// descriptions that wrap to fewer lines than that get blank-padded, and
// ones that still don't fit even after wrapping fall back to truncating
// their last visible line with "..." - same as before, just as a rarer
// last resort instead of the routine case.
type wrappingDelegate struct {
	styles    list.DefaultItemStyles
	height    int
	spacing   int
	descLines int
}

// newWrappingDelegate creates a delegate showing a single-line (truncated)
// title followed by up to descLines word-wrapped description lines.
func newWrappingDelegate(descLines int) wrappingDelegate {
	if descLines < 1 {
		descLines = 1
	}
	return wrappingDelegate{
		styles:    androidListItemStyles(),
		height:    1 + descLines,
		spacing:   1,
		descLines: descLines,
	}
}

func (d wrappingDelegate) Height() int                               { return d.height }
func (d wrappingDelegate) Spacing() int                              { return d.spacing }
func (d wrappingDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d wrappingDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(list.DefaultItem)
	if !ok || m.Width() <= 0 {
		return
	}

	s := d.styles
	textWidth := m.Width() - s.NormalTitle.GetPaddingLeft() - s.NormalTitle.GetPaddingRight()
	if textWidth < 1 {
		textWidth = 1
	}

	title := ansi.Truncate(item.Title(), textWidth, "…")
	descLines := wrapToLines(item.Description(), textWidth, d.descLines)

	isSelected := index == m.Index()
	filtering := m.FilterState() == list.Filtering
	emptyFilter := filtering && m.FilterValue() == ""

	titleStyle, descStyle := s.NormalTitle, s.NormalDesc
	switch {
	case emptyFilter:
		titleStyle, descStyle = s.DimmedTitle, s.DimmedDesc
	case isSelected && !filtering:
		titleStyle, descStyle = s.SelectedTitle, s.SelectedDesc
	}

	fmt.Fprint(w, titleStyle.Render(title))
	for _, line := range descLines {
		fmt.Fprint(w, "\n"+descStyle.Render(line))
	}
}

// wrapToLines word-wraps text to width (via lipgloss, which is ANSI/wide-
// rune aware) and returns exactly maxLines lines: shorter results are
// blank-padded, longer ones are cut with the last visible line truncated
// and marked with an ellipsis.
func wrapToLines(text string, width, maxLines int) []string {
	if width < 1 {
		width = 1
	}
	if maxLines < 1 {
		return nil
	}

	wrapped := lipgloss.NewStyle().Width(width).Render(text)
	lines := strings.Split(wrapped, "\n")
	// lipgloss pads each wrapped line with trailing spaces out to width;
	// harmless once styled, but pointless noise for shorter lines/tests.
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}

	if len(lines) > maxLines {
		// The overall text continues past what we're showing, so the last
		// visible line always needs the "…" marker - but ansi.Truncate only
		// *adds* it when the line itself doesn't already fit within width,
		// which usually isn't the case here (word-wrap already broke it at
		// a width boundary). Reserve room for the marker explicitly instead
		// of relying on Truncate to add it as a side effect.
		const ellipsis = "…"
		maxContentWidth := width - lipgloss.Width(ellipsis)
		if maxContentWidth < 0 {
			maxContentWidth = 0
		}
		lines = lines[:maxLines]
		lines[maxLines-1] = ansi.Truncate(lines[maxLines-1], maxContentWidth, "") + ellipsis
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	return lines
}
