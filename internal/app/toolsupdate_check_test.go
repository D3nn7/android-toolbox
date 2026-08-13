package app

import (
	"path/filepath"
	"testing"
	"time"

	"android-toolbox/internal/config"
)

func TestOutdatedToolsNothingKnownYet(t *testing.T) {
	paths := config.Paths{ToolsDir: t.TempDir()}
	adbOutdated, scrcpyOutdated := outdatedTools(paths, "", "")
	if adbOutdated || scrcpyOutdated {
		t.Fatalf("expected neither tool to be flagged outdated when nothing is known yet, got adb=%v scrcpy=%v", adbOutdated, scrcpyOutdated)
	}
}

// TestOutdatedToolsWithNoLocalMarkerButAKnownLatest proves a tool that was
// never fetched by this app (no local ".version" marker) still counts as
// outdated once *something* is known to exist - matching
// toolsmanager.ToolUpdateStatus's own "Installed empty -> Available still
// meaningful" rule.
func TestOutdatedToolsWithNoLocalMarkerButAKnownLatest(t *testing.T) {
	paths := config.Paths{ToolsDir: t.TempDir()}
	adbOutdated, scrcpyOutdated := outdatedTools(paths, "some-etag", "")
	if !adbOutdated {
		t.Fatal("expected adb to be flagged outdated - a latest version is known but nothing is installed")
	}
	if scrcpyOutdated {
		t.Fatal("expected scrcpy to stay unflagged - nothing is known about it yet")
	}
}

func TestCheckForToolUpdatesCmdNilWhenDisabled(t *testing.T) {
	if cmd := checkForToolUpdatesCmd(nil, config.Paths{}, config.State{}, false); cmd != nil {
		t.Fatal("expected a nil command when AutoCheckToolUpdates is off, so Init() has nothing extra to batch in")
	}
}

func TestCheckForToolUpdatesCmdUsesCacheWithinInterval(t *testing.T) {
	paths := config.Paths{StateFile: filepath.Join(t.TempDir(), "state.json")}
	state := config.State{
		LastToolsUpdateCheckAt: time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		ADBLatestKnown:         "cached-adb",
		ScrcpyLatestKnown:      "cached-scrcpy",
	}

	cmd := checkForToolUpdatesCmd(t.Context(), paths, state, true)
	if cmd == nil {
		t.Fatal("expected a non-nil command when enabled")
	}
	msg, ok := cmd().(toolsUpdateCheckMsg)
	if !ok {
		t.Fatalf("expected a toolsUpdateCheckMsg, got different type")
	}
	if msg.adbLatest != "cached-adb" || msg.scrcpyLatest != "cached-scrcpy" {
		t.Fatalf("expected the cached values to be returned as-is, got %+v", msg)
	}
}

// TestPendingUpdateNoticeLinesCombinesSelfAndToolNotices proves the banner
// mentions both kinds of pending updates when both are known, not just
// whichever was checked first.
func TestPendingUpdateNoticeLinesCombinesSelfAndToolNotices(t *testing.T) {
	m := Model{text: uiTextEN}
	m.paths.ToolsDir = t.TempDir()
	m.latestKnownVersion = "999.0.0"
	m.latestKnownADB = "some-etag"

	lines := m.pendingUpdateNoticeLines()
	if len(lines) != 2 {
		t.Fatalf("expected two notice lines (toolbox + tools), got %d: %v", len(lines), lines)
	}
}

func TestPendingUpdateNoticeLinesEmptyWhenNothingIsOutdated(t *testing.T) {
	m := Model{text: uiTextEN}
	m.paths.ToolsDir = t.TempDir()

	if lines := m.pendingUpdateNoticeLines(); len(lines) != 0 {
		t.Fatalf("expected no notice lines, got %v", lines)
	}
}

// TestRenderUpdateNoticeRespectsDismissal proves dismissing hides the
// banner even though the underlying facts (a newer version is known)
// haven't changed.
func TestRenderUpdateNoticeRespectsDismissal(t *testing.T) {
	m := Model{text: uiTextEN, styles: newStyles()}
	m.paths.ToolsDir = t.TempDir()
	m.latestKnownVersion = "999.0.0"

	if m.renderUpdateNotice() == "" {
		t.Fatal("expected a notice before dismissal")
	}
	m.updateNoticeDismissed = true
	if got := m.renderUpdateNotice(); got != "" {
		t.Fatalf("expected dismissal to hide the notice, got %q", got)
	}
}
