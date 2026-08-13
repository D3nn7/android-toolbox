package app

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/config"
)

func TestPaneWidthsRegularTerminal(t *testing.T) {
	left, right := paneWidths(120)
	if left < minLeftPaneWidth || left > maxLeftPaneWidth {
		t.Fatalf("left = %d, want within [%d, %d]", left, minLeftPaneWidth, maxLeftPaneWidth)
	}
	if left+right+paneGap != 120 {
		t.Fatalf("left(%d)+right(%d)+gap(%d) = %d, want 120", left, right, paneGap, left+right+paneGap)
	}
}

func TestPaneWidthsNarrowTerminalSplitsEvenlyInstead(t *testing.T) {
	left, right := paneWidths(20)
	if left != 10 {
		t.Fatalf("expected an even split (10) for a terminal narrower than the minimums, got left=%d", left)
	}
	if right < 0 {
		t.Fatalf("right must never be negative, got %d", right)
	}
}

func TestPaneWidthsZeroOrNegative(t *testing.T) {
	for _, w := range []int{0, -5} {
		left, right := paneWidths(w)
		if left != 0 || right != 0 {
			t.Fatalf("paneWidths(%d) = (%d, %d), want (0, 0)", w, left, right)
		}
	}
}

func TestBodyHeightHasAFloor(t *testing.T) {
	if got := bodyHeight(0); got < 5 {
		t.Fatalf("bodyHeight(0) = %d, want >= 5 (floor)", got)
	}
	if got := bodyHeight(-10); got < 5 {
		t.Fatalf("bodyHeight(-10) = %d, want >= 5 (floor)", got)
	}
}

func TestPaneContentSizeSubtractsFrameAndHasFloor(t *testing.T) {
	w, h := paneContentSize(40, 20)
	if w != 40-paneFrameWidth || h != 20-paneFrameHeight {
		t.Fatalf("paneContentSize(40, 20) = (%d, %d), want (%d, %d)", w, h, 40-paneFrameWidth, 20-paneFrameHeight)
	}

	w, h = paneContentSize(1, 1)
	if w < 1 || h < 1 {
		t.Fatalf("paneContentSize(1, 1) = (%d, %d), want both >= 1 (floor)", w, h)
	}
}

func TestRightPaneStyleDiffersByState(t *testing.T) {
	styleFor := func(current screen, finished, hasErr bool) lipgloss.TerminalColor {
		m := Model{styles: newStyles(), current: current}
		if current == screenRunner {
			m.runner.finished = finished
			if hasErr {
				m.runner.exitErr = errProbe{}
			}
		}
		return m.rightPaneStyle().GetBorderTopForeground()
	}

	all := map[string]lipgloss.TerminalColor{
		"preview": styleFor(screenDashboard, false, false),
		"param":   styleFor(screenParamForm, false, false),
		"confirm": styleFor(screenConfirm, false, false),
		"running": styleFor(screenRunner, false, false),
		"success": styleFor(screenRunner, true, false),
		"failed":  styleFor(screenRunner, true, true),
	}
	seen := map[lipgloss.TerminalColor]string{}
	for name, color := range all {
		if other, ok := seen[color]; ok {
			t.Fatalf("expected distinct pane colors per state, but %q and %q share one", name, other)
		}
		seen[color] = name
	}
}

type errProbe struct{}

func (errProbe) Error() string { return "boom" }

func TestViewDashboardClusterRendersActionPreview(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 120, 32

	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "a1", Name: "Test Aktion", Category: "Logs", Tool: actions.ToolADB, Command: "shell echo hi"},
	}}
	dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
	m.dashboard = dash
	leftW, _ := paneWidths(m.width)
	leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
	m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))
	m.current = screenDashboard

	out := m.View()

	if !strings.Contains(out, "Test Aktion") {
		t.Fatalf("expected the left pane's action list to show the action name, got:\n%s", out)
	}
	if !strings.Contains(out, m.text.RunHint) {
		t.Fatalf("expected the right pane's preview hint, got:\n%s", out)
	}
}

func TestViewDashboardClusterRendersParamFormAndConfirmAndRunner(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 120, 32
	m.dashboard.serial = "SERIAL1"

	action := actions.Action{ID: "a1", Name: "Param Aktion", Tool: actions.ToolADB, Command: "shell echo {x}",
		Params: []actions.Param{{Name: "x", Label: "X-Wert"}}}

	m.current = screenParamForm
	m.paramForm = newParamFormScreen(action)
	if out := m.View(); !strings.Contains(out, "X-Wert") {
		t.Fatalf("expected param form content in cluster view, got:\n%s", out)
	}

	m.current = screenConfirm
	m.confirm = newConfirmScreen(action, nil, m.text, m.huhTheme, m.rightPaneContentWidth())
	if out := m.View(); !strings.Contains(out, "Param Aktion") {
		t.Fatalf("expected confirm content in cluster view, got:\n%s", out)
	}

	m.current = screenRunner
	m.runner.action = action
	if out := m.View(); !strings.Contains(out, "Param Aktion") {
		t.Fatalf("expected runner content in cluster view, got:\n%s", out)
	}
}

func TestClampToBoxCapsLineCountAfterWrapping(t *testing.T) {
	// A single long line, once word-wrapped to a narrow width, naturally
	// turns into many lines - clampToBox must hard-cap that back down to
	// the requested height instead of letting the box grow taller than
	// its caller budgeted for.
	long := strings.Repeat("Befehl aeoeue ", 30)
	out := clampToBox(long, 15, 4)
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected exactly 4 lines after clamping, got %d:\n%s", len(lines), out)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 15 {
			t.Fatalf("line %d is %d wide, want <= 15: %q", i, w, l)
		}
	}
}

// TestRenderTwoPaneKeepsBothBoxesTheSameHeightEvenWithLongContent isolates
// renderTwoPane itself (no outer Model.View() safety net involved) to prove
// the pane-level fix actually does its job: a right pane whose content
// wraps to many lines must still render at exactly the same outer height as
// the left pane, borders intact - not just "fits the terminal somehow"
// (which the outer clampToTerminal alone could already guarantee, even if
// the two boxes ended up mismatched or a border got cut off).
func TestRenderTwoPaneKeepsBothBoxesTheSameHeightEvenWithLongContent(t *testing.T) {
	shortLeft := "Aktion 1\nAktion 2"
	longRight := "Befehl: " + strings.Repeat("sehrlangeswort ", 40)

	styles := newStyles()
	out := renderTwoPane(styles.PaneLeft, styles.PanePreview, shortLeft, longRight, 20, 30, 8)

	lines := strings.Split(out, "\n")
	wantLines := 8 // the outerHeight passed in
	if len(lines) != wantLines {
		t.Fatalf("expected exactly %d lines (both boxes same height), got %d:\n%s", wantLines, len(lines), out)
	}

	first, last := stripANSI(lines[0]), stripANSI(lines[len(lines)-1])
	if !strings.HasSuffix(strings.TrimRight(first, " "), "╮") {
		t.Fatalf("left/right top border missing or box heights mismatched, first line: %q", first)
	}
	if !strings.HasSuffix(strings.TrimRight(last, " "), "╯") {
		t.Fatalf("bottom border got cut off (this is exactly what lipgloss's MaxHeight would do) - last line: %q", last)
	}
}

// TestViewDashboardClusterNeverOverflowsOnLongContentOrNarrowResize is a
// regression test for the reported bug: a long "Befehl:" preview line, once
// word-wrapped to a narrower pane (e.g. after resizing the terminal
// smaller), used to grow the right pane taller than the left one and/or
// taller than the terminal itself - exactly the "layout breaks on resize"
// and "top of the screen isn't visible" symptoms. The full rendered screen
// must never exceed the terminal's reported width/height, at any size.
func TestViewDashboardClusterNeverOverflowsOnLongContentOrNarrowResize(t *testing.T) {
	longCommand := `shell "echo Modell: $(getprop ro.product.model); echo Hersteller: $(getprop ro.product.manufacturer); echo Android: $(getprop ro.build.version.release); echo SDK: $(getprop ro.build.version.sdk)"`

	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "long", Name: "Sehr lange Befehlsvorschau", Category: "Geraet", Tool: actions.ToolADB, Command: longCommand},
	}}

	for _, size := range []struct{ w, h int }{
		{120, 32}, // normal
		{60, 20},  // resized narrower
		{40, 15},  // quite narrow
		{25, 10},  // extreme/minimal
	} {
		m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
		m.width, m.height = size.w, size.h
		dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
		m.dashboard = dash
		leftW, _ := paneWidths(m.width)
		leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
		m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))
		m.current = screenDashboard

		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) > size.h {
			t.Fatalf("at %dx%d: rendered %d lines, want <= %d (height budget broke: %d)", size.w, size.h, len(lines), size.h, contentH)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > size.w {
				t.Fatalf("at %dx%d: line %d is %d wide, want <= %d: %q", size.w, size.h, i, w, size.w, l)
			}
		}
	}
}

// TestViewNeverWritesToTheVeryLastRow is a regression test for a follow-up
// bug report: even when content fit within the reported terminal height
// exactly, many terminals (including Windows Terminal) still scroll as
// soon as something is written into the bottom-right cell (the classic
// "autowrap at the last row/column" behavior), which pushes the top of an
// otherwise correctly-sized frame out of view. Model.View() must therefore
// never use the very last row at all, leaving a one-line margin.
func TestViewNeverWritesToTheVeryLastRow(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 80, 24

	set := actions.ActionSet{Actions: []actions.Action{
		{ID: "a1", Name: "Test", Tool: actions.ToolADB, Command: "shell echo hi"},
	}}
	dash, _ := newDashboardScreen(context.Background(), nil, set, "SERIAL1", config.Settings{}, m.text)
	m.dashboard = dash
	m.current = screenDashboard

	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) >= m.height {
		t.Fatalf("rendered %d lines for a %d-row terminal - must be < height to leave the last row untouched", len(lines), m.height)
	}
}
