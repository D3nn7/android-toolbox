package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
)

type paramFormScreen struct {
	action actions.Action
	inputs []textinput.Model
	focus  int
}

func newParamFormScreen(a actions.Action) paramFormScreen {
	inputs := make([]textinput.Model, len(a.Params))
	for i, p := range a.Params {
		ti := textinput.New()
		// No placeholder text here: the field's label is already shown as
		// its own caption line right above the input (see viewParamForm),
		// so echoing it again as a placeholder would just show the same
		// text twice whenever the field is empty.
		ti.SetValue(p.Default)
		ti.CursorEnd()
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}
	return paramFormScreen{action: a, inputs: inputs, focus: 0}
}

func (p paramFormScreen) values() map[string]string {
	values := make(map[string]string, len(p.inputs))
	for i, param := range p.action.Params {
		values[param.Name] = p.inputs[i].Value()
	}
	return values
}

func (m Model) updateParamForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.current = screenDashboard
			return m, nil
		case "tab", "shift+tab", "down", "up":
			f := &m.paramForm
			f.inputs[f.focus].Blur()
			if msg.String() == "shift+tab" || msg.String() == "up" {
				f.focus--
			} else {
				f.focus++
			}
			if f.focus >= len(f.inputs) {
				f.focus = 0
			}
			if f.focus < 0 {
				f.focus = len(f.inputs) - 1
			}
			f.inputs[f.focus].Focus()
			return m, nil
		case "enter":
			if m.paramForm.focus < len(m.paramForm.inputs)-1 {
				f := &m.paramForm
				f.inputs[f.focus].Blur()
				f.focus++
				f.inputs[f.focus].Focus()
				return m, nil
			}
			a := m.paramForm.action
			values := m.paramForm.values()
			if a.Confirm {
				m.confirm = newConfirmScreen(a, values, m.text, m.huhTheme, m.rightPaneContentWidth())
				m.current = screenConfirm
				return m, nil
			}
			return m.beginExecution(a, values)
		}
	}

	var cmd tea.Cmd
	m.paramForm.inputs[m.paramForm.focus], cmd = m.paramForm.inputs[m.paramForm.focus].Update(msg)
	return m, cmd
}

// viewParamForm renders the RIGHT pane's content while collecting an
// action's parameters. The left pane and header/footer are composed
// separately in app.go (see isDashboardCluster).
func (m Model) viewParamForm() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", m.styles.Highlight.Render(m.paramForm.action.Name))
	for i, p := range m.paramForm.action.Params {
		label := p.Label
		if label == "" {
			label = p.Name
		}
		marker := "  "
		if i == m.paramForm.focus {
			marker = m.styles.Highlight.Render("> ")
		}
		fmt.Fprintf(&b, "%s%s\n  %s\n", marker, label, m.paramForm.inputs[i].View())
	}
	return b.String()
}
