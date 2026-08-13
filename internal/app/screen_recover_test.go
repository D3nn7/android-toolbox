package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/backup"
	"android-toolbox/internal/config"
)

// newTestRecoverModel sets up a recoverScreen with exactly one backup entry
// already "confirmed" (i.e. the restore dialog is showing), backed by real
// temp directories - unlike the install prompt, backup.Restore only ever
// touches paths under configDir/backupDir, so it's safe to exercise the
// accept path for real here.
func newTestRecoverModel(t *testing.T) (Model, backupItem) {
	t.Helper()
	configDir := t.TempDir()
	backupDir := t.TempDir()

	original := filepath.Join(configDir, "actions.yaml")
	if err := os.WriteFile(original, []byte("old-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := backup.Snapshot(backupDir, original); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("new-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := backup.List(backupDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one backup entry, got %d, err=%v", len(entries), err)
	}
	item := backupItem{e: entries[0]}

	paths := config.Paths{ConfigDir: configDir, BackupDir: backupDir, ActionsFile: original}
	m := New(context.Background(), paths, config.Settings{}, config.State{}, nil)
	m.width, m.height = 100, 30
	m.recover = newRecoverScreen(backupDir, m.text)
	m.recover.confirmed = &item
	dialog, answer := newRestoreConfirmDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), item)
	m.recover.confirmDialog = dialog
	m.recover.confirmAnswer = answer
	m.current = screenRecover
	return m, item
}

func TestRestoreConfirmDeclineDoesNotRestore(t *testing.T) {
	m, _ := newTestRecoverModel(t)

	updated, _ := m.updateRecover(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)

	if m.recover.confirmed != nil {
		t.Fatal("expected 'n' to dismiss the restore confirmation")
	}
	if m.recover.restored != "" {
		t.Fatal("expected declining to skip backup.Restore entirely")
	}
}

func TestRestoreConfirmEscCancels(t *testing.T) {
	m, _ := newTestRecoverModel(t)

	updated, _ := m.updateRecover(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.recover.confirmed != nil {
		t.Fatal("expected esc to dismiss the restore confirmation")
	}
	if m.recover.restored != "" {
		t.Fatal("expected esc to skip backup.Restore entirely")
	}
}

// TestRestoreConfirmArrowToggleThenEnterRestores is the same regression
// class as screen_confirm_test.go's TestConfirmDialogArrowToggleThenEnterAccepts:
// without WithKeyMap(huh.NewDefaultKeyMap()), a standalone huh.Confirm's
// toggle/submit keys silently do nothing.
func TestRestoreConfirmArrowToggleThenEnterRestores(t *testing.T) {
	m, item := newTestRecoverModel(t)

	updated, _ := m.updateRecover(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if !*m.recover.confirmAnswer {
		t.Fatal("expected the left/right toggle key to flip the highlighted button to Yes")
	}

	updated, _ = m.updateRecover(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.recover.confirmed != nil {
		t.Fatal("expected enter after toggling to Yes to resolve the confirmation")
	}
	if m.recover.err != nil {
		t.Fatalf("unexpected restore error: %v", m.recover.err)
	}
	if m.recover.restored == "" {
		t.Fatal("expected a successful restore to be recorded")
	}

	restoredPath := filepath.Join(m.paths.ConfigDir, item.e.OriginalName)
	data, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatalf("expected restored file to exist: %v", err)
	}
	if string(data) != "old-content" {
		t.Fatalf("expected restored content %q, got %q", "old-content", data)
	}
}

func TestRestoreConfirmYKeyRestores(t *testing.T) {
	m, _ := newTestRecoverModel(t)

	updated, _ := m.updateRecover(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)

	if m.recover.confirmed != nil {
		t.Fatal("expected 'y' to resolve the confirmation")
	}
	if m.recover.restored == "" {
		t.Fatal("expected 'y' to restore the backup")
	}
}
