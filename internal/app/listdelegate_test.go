package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestWrapToLinesPadsShortText(t *testing.T) {
	lines := wrapToLines("kurz", 20, 2)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (padded), got %d: %+v", len(lines), lines)
	}
	if lines[0] != "kurz" || lines[1] != "" {
		t.Fatalf("unexpected lines: %+v", lines)
	}
}

func TestWrapToLinesWrapsInsteadOfTruncating(t *testing.T) {
	// This is the real, reported bug: a description this long used to be
	// hard-truncated to a single line with "...". It must now spread across
	// the available lines instead, keeping (most of) the text readable.
	text := "[Anzeige] Nutzt das Geraet als reines USB-HID Ziel ohne Bildspiegelung"
	lines := wrapToLines(text, 25, 2)
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 lines, got %d: %+v", len(lines), lines)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 25 {
			t.Fatalf("line %d is %d wide, want <= 25: %q", i, w, l)
		}
	}
	// The text is 3 lines' worth once wrapped, but we only show 2, so the
	// last visible line legitimately loses its final character or two to
	// make room for the "…" truncation marker - that's expected. What
	// matters is that far more of the original text survives than the old
	// single-line hard-truncate ever showed.
	joined := strings.Join(lines, " ")
	for _, word := range []string{"Anzeige", "Nutzt", "Geraet", "reines"} {
		if !strings.Contains(joined, word) {
			t.Fatalf("expected wrapped text to still contain %q, got: %+v", word, lines)
		}
	}
	if !strings.Contains(lines[1], "…") {
		t.Fatalf("expected the truncated last line to carry an ellipsis marker, got %q", lines[1])
	}
}

func TestWrapToLinesStillTruncatesWhenTooLongForAllLines(t *testing.T) {
	text := strings.Repeat("wort ", 30) // way more than fits in 2 lines of width 10
	lines := wrapToLines(text, 10, 2)
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[1], "…") {
		t.Fatalf("expected the last line to be marked as truncated with an ellipsis, got %q", lines[1])
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 10 {
			t.Fatalf("line %d is %d wide, want <= 10: %q", i, w, l)
		}
	}
}

func TestWrapToLinesZeroMaxLines(t *testing.T) {
	if got := wrapToLines("anything", 10, 0); got != nil {
		t.Fatalf("expected nil for maxLines=0, got %+v", got)
	}
}
