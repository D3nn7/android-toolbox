package app

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/config"
)

func newTestDeviceSelectModel(t *testing.T) Model {
	t.Helper()
	m := New(context.Background(), config.Paths{}, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	ds, _ := newDeviceSelectScreen(context.Background(), nil, config.Settings{})
	ds.list.SetSize(m.width, deviceListHeight(m.height))
	m.deviceSelect = ds
	m.current = screenDeviceSelect
	return m
}

func advanceDeviceSelect(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.updateDeviceSelect(msg)
	return updated.(Model)
}

// TestDeviceSelectSKeyEntersSettings is the point of this whole change: the
// user must be able to reach Settings before a device has ever been
// selected, not just from the dashboard.
func TestDeviceSelectSKeyEntersSettings(t *testing.T) {
	m := newTestDeviceSelectModel(t)

	m = advanceDeviceSelect(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if m.current != screenSettings {
		t.Fatalf("expected 's' to enter the settings screen, current = %v", m.current)
	}
	if m.settingsReturnTo != screenDeviceSelect {
		t.Fatalf("expected settingsReturnTo to be recorded as screenDeviceSelect, got %v", m.settingsReturnTo)
	}
}

// TestSettingsEscReturnsToDeviceSelectWhenEnteredFromThere proves the round
// trip: esc must go back to wherever settings was actually opened from,
// not unconditionally to the dashboard.
func TestSettingsEscReturnsToDeviceSelectWhenEnteredFromThere(t *testing.T) {
	m := newTestDeviceSelectModel(t)
	m = advanceDeviceSelect(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	m = advanceSettings(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.current != screenDeviceSelect {
		t.Fatalf("expected esc to return to the device select screen, current = %v", m.current)
	}
}

// TestDeviceSelectSKeyDoesNotInterceptFilterTyping proves the new binding
// respects the same "don't steal keys while filtering" rule the dashboard
// and recover screens already follow - typing an "s" while searching for a
// device must reach the filter box, not open Settings.
func TestDeviceSelectSKeyDoesNotInterceptFilterTyping(t *testing.T) {
	m := newTestDeviceSelectModel(t)
	m.deviceSelect.list.SetItems([]list.Item{deviceItem{}})

	var cmd tea.Cmd
	m.deviceSelect.list, cmd = m.deviceSelect.list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	_ = cmd

	m = advanceDeviceSelect(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if m.current == screenSettings {
		t.Fatal("expected 's' typed into the filter box to not open Settings")
	}
	if m.deviceSelect.list.FilterInput.Value() != "s" {
		t.Fatalf("expected 's' to reach the filter input instead, got %q", m.deviceSelect.list.FilterInput.Value())
	}
}

func TestDeviceSelectQKeyQuits(t *testing.T) {
	m := newTestDeviceSelectModel(t)

	_, cmd := m.updateDeviceSelect(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected 'q' to return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected a tea.QuitMsg, got %T", cmd())
	}
}
