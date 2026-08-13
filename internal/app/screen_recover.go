package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/backup"
)

type backupItem struct{ e backup.Entry }

func (i backupItem) Title() string { return i.e.OriginalName }
func (i backupItem) Description() string {
	return i.e.Timestamp.Format("2006-01-02 15:04:05")
}
func (i backupItem) FilterValue() string { return i.e.OriginalName }

type recoverScreen struct {
	list     list.Model
	err      error
	restored string

	confirmed     *backupItem
	confirmDialog huh.Field
	confirmAnswer *bool
}

func newRecoverScreen(backupDir string, t uiText) recoverScreen {
	entries, err := backup.List(backupDir)
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = backupItem{e: e}
	}
	l := list.New(items, newWrappingDelegate(2), 0, 0)
	l.Styles = androidListStyles()
	// The page title just above already names this screen - the list's
	// own title bar would just restate that a second time.
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // this screen renders its own footer text instead
	l.FilterInput.Placeholder = t.BackupFilterPlaceholder
	return recoverScreen{list: l, err: err}
}

// newRestoreConfirmDialog builds the "really restore this backup?" confirm
// dialog. See confirmScreen's doc comment for why the bound value is a
// *bool: huh.Confirm binds to the pointer's target, which must survive
// recoverScreen being copied by value into Model.recover on every Update.
func newRestoreConfirmDialog(t uiText, theme *huh.Theme, width int, item backupItem) (huh.Field, *bool) {
	answer := new(bool)
	dialog := huh.NewConfirm().
		Title(fmt.Sprintf(t.RestoreTitleFmt, item.e.OriginalName, item.e.Timestamp.Format("2006-01-02 15:04:05"))).
		Description(t.RestoreDescription).
		Affirmative(t.RestoreYes).
		Negative(t.RestoreNo).
		Value(answer).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()
	return dialog, answer
}

func (m Model) updateRecover(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.recover.confirmed != nil {
			return m.updateRestoreConfirm(key)
		}

		if m.recover.list.FilterState() != list.Filtering {
			switch key.String() {
			case "esc", "q":
				return m.enterDashboard(m.dashboard.serial)
			case "enter":
				if item, ok := m.recover.list.SelectedItem().(backupItem); ok {
					m.recover.confirmed = &item
					dialog, answer := newRestoreConfirmDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), item)
					m.recover.confirmDialog = dialog
					m.recover.confirmAnswer = answer
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.recover.list, cmd = m.recover.list.Update(msg)
	return m, cmd
}

func (m Model) updateRestoreConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "esc" {
		m.recover.confirmed = nil
		return m, nil
	}

	updated, cmd := m.recover.confirmDialog.Update(key)
	if field, ok := updated.(huh.Field); ok {
		m.recover.confirmDialog = field
	}

	switch key.String() {
	case "enter", "tab", "y", "Y", "n", "N":
		if !*m.recover.confirmAnswer {
			m.recover.confirmed = nil
			return m, nil
		}
		item := *m.recover.confirmed
		m.recover.confirmed = nil
		err := backup.Restore(item.e, m.paths.ConfigDir, m.paths.BackupDir)
		if err != nil {
			m.recover.err = err
			return m, nil
		}
		m.recover.restored = fmt.Sprintf(m.text.RestoredEntryFmt, item.e.OriginalName, item.e.Timestamp.Format("2006-01-02 15:04:05"))
		if set, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML); err == nil {
			m.actionSet = set
		}
		return m, nil
	}
	return m, cmd
}

func (m Model) viewRecover() string {
	title := m.styles.Title.Render(m.text.RecoverTitle)

	if m.recover.confirmed != nil {
		return title + "\n\n" + m.recover.confirmDialog.View()
	}

	body := m.recover.list.View()
	if len(m.recover.list.Items()) == 0 {
		body = m.styles.Subtle.Render(m.text.RecoverNone)
	}
	if m.recover.err != nil {
		body = m.styles.Error.Render(m.recover.err.Error()) + "\n" + body
	}
	if m.recover.restored != "" {
		body = m.styles.OK.Render(fmt.Sprintf(m.text.RecoveredFmt, m.recover.restored)) + "\n\n" + body
	}
	footer := m.styles.Subtle.Render(m.text.RecoverFooter)
	return title + "\n\n" + body + "\n" + footer
}
