package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/actions"
)

// confirmScreen wraps a huh.Confirm dialog - a real, focus-navigable Yes/No
// dialog with buttons and keyboard hints - instead of a bare "press y or n"
// text prompt.
//
// result is a *bool (not a bool) deliberately: huh.Confirm.Value binds
// directly to the pointer's target, and confirmScreen is copied by value
// whenever it's assigned into Model.confirm (Go's usual bubbletea pattern).
// If result were a plain bool, the dialog's internal accessor would end up
// pointing at a field on a since-discarded copy of this struct rather than
// the one actually stored on the Model, and toggling Yes/No would silently
// stop updating anything we can see. A pointer's target survives being
// copied around just fine.
type confirmScreen struct {
	action actions.Action
	params map[string]string
	dialog huh.Field
	result *bool
}

func newConfirmScreen(a actions.Action, params map[string]string, t uiText, theme *huh.Theme, width int) confirmScreen {
	result := new(bool)

	dialog := huh.NewConfirm().
		Title(fmt.Sprintf(t.ConfirmTitleFmt, a.Name)).
		Description(a.Description).
		Affirmative(t.ConfirmYes).
		Negative(t.ConfirmNo).
		Value(result).
		// A standalone field (as opposed to one embedded in a huh.Form)
		// never gets its keymap populated on its own - without this, y/n/
		// enter/tab all silently do nothing inside huh's own Update, since
		// key.Matches has nothing bound to match against.
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()

	return confirmScreen{action: a, params: params, dialog: dialog, result: result}
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.current = screenDashboard
		return m, nil
	}

	updated, cmd := m.confirm.dialog.Update(msg)
	if field, ok := updated.(huh.Field); ok {
		m.confirm.dialog = field
	}

	// huh.Confirm's own keymap (y/n/enter/tab) both flips its internal
	// value and requests advancing to the "next field" - meaningless here
	// since we're running it standalone rather than inside a huh.Form, so
	// there's no exported message type to react to. Recognizing the same
	// keys ourselves and reading the now-updated *result is simpler than
	// fighting huh's form-navigation machinery for a single yes/no field.
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "tab", "y", "Y", "n", "N":
			if *m.confirm.result {
				return m.beginExecution(m.confirm.action, m.confirm.params)
			}
			m.current = screenDashboard
			return m, nil
		}
	}
	return m, cmd
}

// viewConfirm renders the RIGHT pane's content while confirming a
// destructive action. The left pane and header/footer are composed
// separately in app.go (see isDashboardCluster).
func (m Model) viewConfirm() string {
	return m.confirm.dialog.View()
}
