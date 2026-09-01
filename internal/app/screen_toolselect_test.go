package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/config"
)

func newTestToolSelectModel(t *testing.T) Model {
	t.Helper()
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.current = screenToolSelect
	return m
}

func advanceToolSelect(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.updateToolSelect(msg)
	return updated.(Model)
}

func TestToolSelectCursorNavigation(t *testing.T) {
	m := newTestToolSelectModel(t)

	if m.toolSelect.cursor != 0 {
		t.Fatalf("expected the cursor to start at 0, got %d", m.toolSelect.cursor)
	}
	m = advanceToolSelect(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.toolSelect.cursor != 0 {
		t.Fatalf("expected up at the top to stay clamped at 0, got %d", m.toolSelect.cursor)
	}
	for i := 0; i < len(toolOrder)+2; i++ {
		m = advanceToolSelect(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	want := len(toolOrder) - 1
	if m.toolSelect.cursor != want {
		t.Fatalf("expected down past the last row to clamp at %d, got %d", want, m.toolSelect.cursor)
	}
}

// TestToolSelectEnterOnAPKInfoOpensThatTool proves picking "APK Info" from
// the menu actually gets you there, with a ready-to-use file picker.
func TestToolSelectEnterOnAPKInfoOpensThatTool(t *testing.T) {
	m := newTestToolSelectModel(t)
	m.toolSelect.cursor = 1 // APK Info (see toolOrder)

	updated, cmd := m.updateToolSelect(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.current != screenAPKInfo {
		t.Fatalf("expected enter on APK Info to switch to screenAPKInfo, current = %v", m.current)
	}
	if m.apkInfo.stage != apkInfoPicking {
		t.Fatalf("expected the APK Info tool to start in the picking stage, got %v", m.apkInfo.stage)
	}
	if cmd == nil {
		t.Fatal("expected a command to initialize the file picker (its directory read)")
	}
}

// TestToolSelectEnterOnEmulatorsOpensThatTool proves the new "Emulators"
// entry lands on the emulator list screen, degrading gracefully (no crash)
// even though m.avdManager is nil in this bare test Model - setupTools()
// never ran, mirroring how a real user reaches this screen only after it
// has.
func TestToolSelectEnterOnEmulatorsOpensThatTool(t *testing.T) {
	m := newTestToolSelectModel(t)
	m.toolSelect.cursor = 2 // Emulators (see toolOrder)

	updated, _ := m.updateToolSelect(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.current != screenEmulatorList {
		t.Fatalf("expected enter on Emulators to switch to screenEmulatorList, current = %v", m.current)
	}
}

// TestToolSelectEnterOnDevicesEntersDeviceSelect proves the first entry
// still leads to the existing adb-based flow, unchanged.
func TestToolSelectEnterOnDevicesEntersDeviceSelect(t *testing.T) {
	m := newTestToolSelectModel(t)
	m.toolSelect.cursor = 0 // Devices

	updated, _ := m.updateToolSelect(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.current != screenDeviceSelect {
		t.Fatalf("expected enter on Devices to switch to screenDeviceSelect, current = %v", m.current)
	}
}

func TestCanSwitchTool(t *testing.T) {
	cases := []struct {
		screen screen
		want   bool
	}{
		{screenSplash, false},
		{screenHealthcheck, false},
		{screenHealthFailed, false},
		{screenToolSelect, false},
		{screenDeviceSelect, true},
		{screenDashboard, true},
		{screenAPKInfo, true},
		{screenSettings, true},
		{screenEmulatorList, true},
		{screenEmulatorCreate, true},
	}
	for _, c := range cases {
		if got := canSwitchTool(c.screen); got != c.want {
			t.Errorf("canSwitchTool(%v) = %v, want %v", c.screen, got, c.want)
		}
	}
}

// TestGlobalCtrlTSwitchesToToolSelect proves ctrl+t works from an arbitrary
// "already in a tool" screen (the whole point of "jederzeit wechselbar").
func TestGlobalCtrlTSwitchesToToolSelect(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.current = screenDeviceSelect

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)

	if m.current != screenToolSelect {
		t.Fatalf("expected ctrl+t to switch to the tool-select screen, current = %v", m.current)
	}
}

// TestGlobalCtrlTIsANoOpDuringHealthcheck proves the guard actually
// excludes the startup-only screens rather than firing everywhere
// unconditionally.
func TestGlobalCtrlTIsANoOpDuringHealthcheck(t *testing.T) {
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.current = screenHealthcheck

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)

	if m.current != screenHealthcheck {
		t.Fatalf("expected ctrl+t to be ignored on the healthcheck screen, current = %v", m.current)
	}
}
