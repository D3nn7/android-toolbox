package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/config"
	"android-toolbox/internal/toolsmanager"
)

func newTestSettingsModel(t *testing.T) Model {
	t.Helper()
	paths := config.Paths{SettingsFile: filepath.Join(t.TempDir(), "settings.yaml")}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.dashboard.serial = "SERIAL1"
	m.adbTool = toolsmanager.ResolvedTool{Path: "/tools/adb", Source: "bundled"}
	m.actionSet = actions.ActionSet{Actions: []actions.Action{{ID: "a1"}, {ID: "a2"}}}
	return m
}

func advanceSettings(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.updateSettings(msg)
	return updated.(Model)
}

// TestDashboardSKeyEntersSettingsScreen makes sure the settings screen is
// actually reachable, not just internally correct, and starts out browsing
// the list rather than mid-edit.
func TestDashboardSKeyEntersSettingsScreen(t *testing.T) {
	m := newTestSettingsModel(t)
	m.current = screenDashboard

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)

	if m.current != screenSettings {
		t.Fatalf("expected 's' to enter the settings screen, current = %v", m.current)
	}
	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected the settings screen to start out browsing, stage = %v", m.settingsScreen.stage)
	}
}

// TestSettingsEscFromBrowsingReturnsToDashboardWithoutWriting proves esc
// leaves the settings screen without ever having touched settings.yaml -
// there's nothing to "cancel" in bulk in this design (each field commits
// individually, see the confirm-flow tests below), so plain browsing must
// never write anything on its own.
func TestSettingsEscFromBrowsingReturnsToDashboardWithoutWriting(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.current != screenDashboard {
		t.Fatalf("expected esc to return to the dashboard, current = %v", m.current)
	}
	if _, err := os.Stat(m.paths.SettingsFile); err == nil {
		t.Fatal("expected esc to never write settings.yaml")
	}
}

// TestSettingsBrowsingCursorNavigation checks that up/down move the cursor
// among the rows and clamp at both ends rather than wrapping or going out
// of bounds.
func TestSettingsBrowsingCursorNavigation(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	if m.settingsScreen.cursor != 0 {
		t.Fatalf("expected the cursor to start at row 0, got %d", m.settingsScreen.cursor)
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.settingsScreen.cursor != 0 {
		t.Fatalf("expected up at the top row to stay clamped at 0, got %d", m.settingsScreen.cursor)
	}

	for i := 0; i < len(settingsFieldsOrder)+2; i++ {
		m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	want := len(settingsFieldsOrder) - 1
	if m.settingsScreen.cursor != want {
		t.Fatalf("expected down past the last row to clamp at %d, got %d", want, m.settingsScreen.cursor)
	}
}

// TestSettingsEnterOnRowStartsEditingJustThatRow is the core of "select
// first, then change": enter must not be able to change anything by itself
// - it only opens an editor for whichever row is highlighted.
func TestSettingsEnterOnRowStartsEditingJustThatRow(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 1 // AI provider row

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.settingsScreen.stage != settingsEditing {
		t.Fatalf("expected enter to start editing, stage = %v", m.settingsScreen.stage)
	}
	if m.settingsScreen.editingField != settingsFieldAIProvider {
		t.Fatalf("expected to be editing the AI provider row, got %v", m.settingsScreen.editingField)
	}
	if m.settingsScreen.editField == nil || m.settingsScreen.editValue == nil {
		t.Fatal("expected an editor field and bound value to be constructed")
	}
}

// TestSettingsEditEscDiscardsWithoutConfirming proves esc while editing
// throws away the in-progress change and never even reaches the confirm
// step, let alone settings.yaml.
func TestSettingsEditEscDiscardsWithoutConfirming(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing language
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyDown})  // highlight "Deutsch"
	if *m.settingsScreen.editValue != "de" {
		t.Fatalf("expected the arrow key to move the highlighted option to 'de', got %q", *m.settingsScreen.editValue)
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected esc to return to browsing, stage = %v", m.settingsScreen.stage)
	}
	if m.settingsScreen.language != "en" {
		t.Fatalf("expected esc to discard the edit, language = %q", m.settingsScreen.language)
	}
	if _, err := os.Stat(m.paths.SettingsFile); err == nil {
		t.Fatal("expected esc during editing to never write settings.yaml")
	}
}

// TestSettingsSubmittingUnchangedValueSkipsConfirm makes sure re-submitting
// a field without actually changing it is a plain no-op back to browsing -
// there's nothing to confirm.
func TestSettingsSubmittingUnchangedValueSkipsConfirm(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing language (already "en")
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // submit without changing it

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected an unchanged value to skip the confirm dialog, stage = %v", m.settingsScreen.stage)
	}
}

// TestSettingsInvalidTimeoutStaysInEditing proves a non-numeric timeout is
// rejected before ever reaching the confirm step.
func TestSettingsInvalidTimeoutStaysInEditing(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 3 // AI timeout row

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.settingsScreen.editingField != settingsFieldAITimeout {
		t.Fatalf("expected to be editing the timeout field, got %v", m.settingsScreen.editingField)
	}

	*m.settingsScreen.editValue = "not-a-number"
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.settingsScreen.stage != settingsEditing {
		t.Fatalf("expected an invalid number to stay in editing rather than proceed to confirm, stage = %v", m.settingsScreen.stage)
	}
}

// TestBoolDisplayWordRendersAPlainWord is a regression test for the
// explicit "no brackets, that's confusing" feedback: the two settings
// toggles must show a plain "Enabled"/"Disabled" word, not a "[x]"/"[ ]"
// checkbox glyph.
func TestBoolDisplayWordRendersAPlainWord(t *testing.T) {
	if got := boolDisplayWord("true", uiTextEN); got != "Enabled" {
		t.Fatalf("expected the plain word for true, got %q", got)
	}
	if got := boolDisplayWord("false", uiTextEN); got != "Disabled" {
		t.Fatalf("expected the plain word for false, got %q", got)
	}
	if got := boolDisplayWord("true", uiTextDE); got != "Aktiviert" {
		t.Fatalf("expected the German word too, got %q", got)
	}
}

// TestSettingsToggleShowStartupAnimationOffAndAccept exercises the second
// settings interaction pattern (a checkbox-style Select rather than a
// multi-option one) through the same select -> enter -> edit -> confirm
// flow as every other settings field, and proves it actually persists.
func TestSettingsToggleShowStartupAnimationOffAndAccept(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 4 // Startup animation row (Language, AIProvider, AICommand, AITimeout, ShowStartupAnimation, ShowHealthcheck)

	if m.settingsScreen.rawValue(settingsFieldShowStartupAnimation) != "false" {
		t.Fatalf("expected the zero-value test model to start with the animation off, got %q", m.settingsScreen.showStartupAnimation)
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing
	if m.settingsScreen.editingField != settingsFieldShowStartupAnimation {
		t.Fatalf("expected to be editing the startup-animation row, got %v", m.settingsScreen.editingField)
	}
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyUp}) // move off "false" to "true"
	if *m.settingsScreen.editValue != "true" {
		t.Fatalf("expected the arrow key to move the highlighted option to 'true', got %q", *m.settingsScreen.editValue)
	}
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // submit -> confirm dialog

	if m.settingsScreen.stage != settingsConfirming {
		t.Fatalf("expected submitting a real change to open a confirm dialog, stage = %v", m.settingsScreen.stage)
	}
	if m.settings.UI.ShowStartupAnimation {
		t.Fatal("expected the setting to remain unchanged until the dialog is actually confirmed")
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected confirming to return to browsing, stage = %v", m.settingsScreen.stage)
	}
	if !m.settings.UI.ShowStartupAnimation {
		t.Fatal("expected the setting to actually flip to true")
	}

	persisted, err := config.LoadSettings(m.paths)
	if err != nil {
		t.Fatalf("expected settings.yaml to be written: %v", err)
	}
	if !persisted.UI.ShowStartupAnimation {
		t.Fatal("expected the change to be persisted to settings.yaml")
	}
}

// TestSettingsToggleShowHealthcheckDeclineLeavesItUnchanged mirrors
// TestSettingsChangeLanguageDeclineKeepsEnglish for the other checkbox row.
func TestSettingsToggleShowHealthcheckDeclineLeavesItUnchanged(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 5 // Health check screen row

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected declining to return to browsing, stage = %v", m.settingsScreen.stage)
	}
	if m.settings.UI.ShowHealthcheck {
		t.Fatal("expected declining to leave the setting unchanged")
	}
	if _, err := os.Stat(m.paths.SettingsFile); err == nil {
		t.Fatal("expected declining to never write settings.yaml")
	}
}

// TestSettingsToggleAutoCheckToolUpdatesOffAndAccept mirrors the other two
// checkbox-style toggles, proving the third one wired up the same way.
func TestSettingsToggleAutoCheckToolUpdatesOffAndAccept(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settings.UI.AutoCheckToolUpdates = true
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 6 // Auto-check tool updates row (see settingsFieldsOrder)

	if m.settingsScreen.rawValue(settingsFieldAutoCheckToolUpdates) != "true" {
		t.Fatalf("expected the toggle to start on, got %q", m.settingsScreen.autoCheckToolUpdates)
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.settingsScreen.editingField != settingsFieldAutoCheckToolUpdates {
		t.Fatalf("expected to be editing the auto-check-tool-updates row, got %v", m.settingsScreen.editingField)
	}
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyDown}) // move off "true" to "false"
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.settings.UI.AutoCheckToolUpdates {
		t.Fatal("expected the setting to actually flip to false")
	}
	persisted, err := config.LoadSettings(m.paths)
	if err != nil {
		t.Fatalf("expected settings.yaml to be written: %v", err)
	}
	if persisted.UI.AutoCheckToolUpdates {
		t.Fatal("expected the change to be persisted")
	}
}

// TestSettingsCheckForUpdatesNowTriggersALiveCheck proves pressing enter on
// that row does NOT go through the usual edit/confirm flow - it stays in
// browsing, shows a "checking" status immediately, and returns a command.
func TestSettingsCheckForUpdatesNowTriggersALiveCheck(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 7 // Check for updates now row

	updated, cmd := m.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected the check-now action to stay in browsing, not open an editor, stage = %v", m.settingsScreen.stage)
	}
	if m.settingsScreen.checkNowStatus != m.text.SettingsCheckingForUpdates {
		t.Fatalf("expected an immediate 'checking' status, got %q", m.settingsScreen.checkNowStatus)
	}
	if cmd == nil {
		t.Fatal("expected a command that performs the live check")
	}
}

// TestSettingsCheckForUpdatesNowIgnoresRepeatPressWhileInFlight proves a
// second enter press while a check is already running doesn't kick off a
// second, redundant one.
func TestSettingsCheckForUpdatesNowIgnoresRepeatPressWhileInFlight(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings
	m.settingsScreen.cursor = 7
	m.settingsScreen.checkNowStatus = m.text.SettingsCheckingForUpdates

	_, cmd := m.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected a repeat press while checking to be a no-op")
	}
}

// TestApplySettingsUpdateCheckUpdatesModelAndPersistsCache proves a
// completed check result is folded into Model (so the dashboard banner and
// this screen's own "Updates" block both immediately reflect it) and
// persisted to state.json, same as the periodic background checks.
func TestApplySettingsUpdateCheckUpdatesModelAndPersistsCache(t *testing.T) {
	m := newTestSettingsModel(t)
	m.paths.StateFile = filepath.Join(t.TempDir(), "state.json")
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = m.applySettingsUpdateCheck(settingsUpdateCheckMsg{toolboxVersion: "9.9.9", adbLatest: "etag-1", scrcpyLatest: "v4.1"})

	if m.latestKnownVersion != "9.9.9" || m.latestKnownADB != "etag-1" || m.latestKnownScrcpy != "v4.1" {
		t.Fatalf("expected all three latest-known fields to be set, got %+v", m)
	}
	if m.settingsScreen.checkNowStatus != m.text.SettingsCheckCompleteStatus {
		t.Fatalf("expected a completion status, got %q", m.settingsScreen.checkNowStatus)
	}

	persisted, err := config.LoadState(m.paths)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.LatestKnownVersion != "9.9.9" || persisted.ADBLatestKnown != "etag-1" || persisted.ScrcpyLatestKnown != "v4.1" {
		t.Fatalf("expected the cache to be persisted to state.json, got %+v", persisted)
	}
	if persisted.LastUpdateCheckAt == "" || persisted.LastToolsUpdateCheckAt == "" {
		t.Fatal("expected both check timestamps to be stamped")
	}
}

// TestApplySettingsUpdateCheckKeepsExistingValuesOnPartialFailure proves a
// query that only partially succeeds (e.g. GitHub reachable, Google's
// platform-tools endpoint not) doesn't blow away whatever was already known
// for the piece that failed.
func TestApplySettingsUpdateCheckKeepsExistingValuesOnPartialFailure(t *testing.T) {
	m := newTestSettingsModel(t)
	m.paths.StateFile = filepath.Join(t.TempDir(), "state.json")
	m.latestKnownADB = "previously-known-etag"

	m = m.applySettingsUpdateCheck(settingsUpdateCheckMsg{toolboxVersion: "9.9.9", adbLatest: "", scrcpyLatest: "v4.1"})

	if m.latestKnownADB != "previously-known-etag" {
		t.Fatalf("expected a failed adb check to leave the previous value alone, got %q", m.latestKnownADB)
	}
}

// TestToolboxUpdateSummaryLine and TestToolUpdateSummaryLine cover the
// three possible states each summary line can be in.
func TestToolboxUpdateSummaryLine(t *testing.T) {
	if got := toolboxUpdateSummaryLine(uiTextEN, ""); !strings.Contains(got, "not checked yet") {
		t.Fatalf("expected 'not checked yet' when nothing is known, got %q", got)
	}
	if got := toolboxUpdateSummaryLine(uiTextEN, buildinfo.Version); !strings.Contains(got, "up to date") {
		t.Fatalf("expected 'up to date' when latestKnown equals the running version, got %q", got)
	}
	if got := toolboxUpdateSummaryLine(uiTextEN, "999.0.0"); !strings.Contains(got, "update available") {
		t.Fatalf("expected 'update available' for a genuinely newer version, got %q", got)
	}
}

func TestToolUpdateSummaryLine(t *testing.T) {
	if got := toolUpdateSummaryLine("adb", "", "", uiTextEN); !strings.Contains(got, "not checked yet") {
		t.Fatalf("expected 'not checked yet', got %q", got)
	}
	if got := toolUpdateSummaryLine("adb", "etag-1", "etag-1", uiTextEN); !strings.Contains(got, "up to date") {
		t.Fatalf("expected 'up to date' for matching versions, got %q", got)
	}
	if got := toolUpdateSummaryLine("adb", "etag-1", "etag-2", uiTextEN); !strings.Contains(got, "update available") {
		t.Fatalf("expected 'update available' for differing versions, got %q", got)
	}
	if got := toolUpdateSummaryLine("adb", "", "etag-2", uiTextEN); !strings.Contains(got, "-") {
		t.Fatalf("expected an unknown installed version to render as '-', got %q", got)
	}
}

// TestSettingsChangeLanguageToGermanAndAccept is the full happy path this
// screen exists for, and doubles as the explicit check that German is
// actually reachable end to end: select the language row, arrow to
// "Deutsch", submit, confirm - the change must only take effect after that
// final confirmation, not before.
func TestSettingsChangeLanguageToGermanAndAccept(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // start editing language
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyDown})  // highlight "Deutsch"
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // submit -> should ask for confirmation

	if m.settingsScreen.stage != settingsConfirming {
		t.Fatalf("expected submitting a real change to open a confirm dialog, stage = %v", m.settingsScreen.stage)
	}
	if m.settingsScreen.confirmNewValue != "de" {
		t.Fatalf("expected the pending change to be 'de', got %q", m.settingsScreen.confirmNewValue)
	}
	if m.settings.Language() != "en" {
		t.Fatal("expected the language to remain unchanged until the dialog is actually confirmed")
	}

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected confirming to return to browsing, stage = %v", m.settingsScreen.stage)
	}
	if m.settings.Language() != "de" {
		t.Fatalf("expected the language to actually change to German, got %q", m.settings.Language())
	}
	if m.text.RunHint != uiTextDE.RunHint {
		t.Fatal("expected m.text to switch to German after confirming")
	}
	if m.keys.Run.Help().Desc != uiTextDE.KeyRun {
		t.Fatalf("expected m.keys to be rebuilt with German labels, got %q", m.keys.Run.Help().Desc)
	}

	persisted, err := config.LoadSettings(m.paths)
	if err != nil {
		t.Fatalf("expected settings.yaml to be written: %v", err)
	}
	if persisted.UI.Language != "de" {
		t.Fatalf("persisted language = %q, want %q", persisted.UI.Language, "de")
	}

	// German must also survive a restart, not just the live session.
	reloaded := New(context.Background(), m.paths, persisted, config.State{}, nil)
	if reloaded.text.RunHint != uiTextDE.RunHint {
		t.Fatal("expected a fresh Model built from the persisted settings to also be German")
	}
}

// TestSettingsChangeLanguageDeclineKeepsEnglish is the same flow, but
// declining the confirm dialog - the language (and settings.yaml) must
// stay exactly as they were.
func TestSettingsChangeLanguageDeclineKeepsEnglish(t *testing.T) {
	m := newTestSettingsModel(t)
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.settingsScreen.stage != settingsBrowsing {
		t.Fatalf("expected declining to return to browsing, stage = %v", m.settingsScreen.stage)
	}
	if m.settings.Language() != "en" {
		t.Fatalf("expected declining to leave the language unchanged, got %q", m.settings.Language())
	}
	if _, err := os.Stat(m.paths.SettingsFile); err == nil {
		t.Fatal("expected declining to never write settings.yaml")
	}
}

// TestProviderOptionLabelReflectsInstalledStatus proves the select shows an
// installed/not-installed status per provider rather than always looking
// the same regardless of whether the configured CLI command actually
// resolves on this machine.
func TestProviderOptionLabelReflectsInstalledStatus(t *testing.T) {
	installedLabel := providerOptionLabel(uiTextEN, "claude", "go", 5, "")
	if !strings.Contains(installedLabel, "installed") || strings.Contains(installedLabel, "not installed") {
		t.Fatalf("expected an installed label for a command on PATH, got %q", installedLabel)
	}

	notInstalledLabel := providerOptionLabel(uiTextEN, "claude", "definitely-not-a-real-binary-xyz", 5, "")
	if !strings.Contains(notInstalledLabel, "not installed") {
		t.Fatalf("expected a not-installed label for a nonexistent command, got %q", notInstalledLabel)
	}
}

// TestSettingsToolInfoShowsResolvedTools proves the "tool info" block
// actually reflects what Model already resolved at startup, without doing
// any new I/O of its own.
func TestSettingsToolInfoShowsResolvedTools(t *testing.T) {
	m := newTestSettingsModel(t)

	info := settingsToolInfo(m)

	if !strings.Contains(info, "/tools/adb") || !strings.Contains(info, "bundled") {
		t.Fatalf("expected adb path/source in tool info, got:\n%s", info)
	}
	if !strings.Contains(info, m.text.SettingsToolNotResolved) {
		t.Fatalf("expected scrcpy to show as not resolved (nil Launcher), got:\n%s", info)
	}
	if !strings.Contains(info, "2") {
		t.Fatalf("expected the 2-action count to appear, got:\n%s", info)
	}
}

// TestSettingsViewPreservesBackslashesInPaths is a regression test for a
// real bug: the tool-info block used to be rendered as a huh.Note field's
// Description, which runs through huh's own tiny markdown-like parser
// (field_note.go's render()) that treats "\" as an escape character and
// silently drops it. On Windows, where every resolved tool/config path is
// backslash-separated, that mangled every path in the settings screen
// (e.g. "C:\tools\adb\adb.exe" lost every backslash). The tool-info block
// renders as plain text outside any huh field, so backslashes survive.
func TestSettingsViewPreservesBackslashesInPaths(t *testing.T) {
	m := newTestSettingsModel(t)
	m.paths.ConfigDir = `C:\Users\test\AppData\Roaming\android-toolbox`
	m.adbTool.Path = `C:\Users\test\AppData\Roaming\android-toolbox\tools\adb\adb.exe`
	m.settingsScreen = newSettingsScreen(m)
	m.current = screenSettings

	out := m.viewSettings()

	if !strings.Contains(out, m.adbTool.Path) {
		t.Fatalf("expected the adb path's backslashes to survive intact, got:\n%s", out)
	}
	if !strings.Contains(out, m.paths.ConfigDir) {
		t.Fatalf("expected the config dir's backslashes to survive intact, got:\n%s", out)
	}
}
