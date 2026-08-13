package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/backup"
)

// actionEditFieldKind identifies one of the action editor's editable rows.
// Params/Format/LivePreview are deliberately not covered here - Params is a
// list of objects, not a single value, and the other two are advanced/rare
// enough that hand-editing actions.yaml (or the AI feature) covers them
// fine for now.
type actionEditFieldKind int

const (
	actionEditFieldName actionEditFieldKind = iota
	actionEditFieldDescription
	actionEditFieldCategory
	actionEditFieldTool
	actionEditFieldCommand
	actionEditFieldConfirm
	actionEditFieldInteractive
)

var actionEditFieldsOrder = [...]actionEditFieldKind{
	actionEditFieldName,
	actionEditFieldDescription,
	actionEditFieldCategory,
	actionEditFieldTool,
	actionEditFieldCommand,
	actionEditFieldConfirm,
	actionEditFieldInteractive,
}

// actionEditStage mirrors settingsStage exactly: browse the field list,
// edit whichever one was selected, confirm before it actually takes
// effect. See settingsStage's doc comment - the same reasoning applies
// here, just for one action's fields instead of an app setting.
type actionEditStage int

const (
	actionEditBrowsing actionEditStage = iota
	actionEditEditing
	actionEditConfirming
)

type actionEditScreen struct {
	stage  actionEditStage
	cursor int

	// action is the current, already-persisted state of the action being
	// edited - refreshed after every successful commit (see
	// commitActionEditField) so the browsing list always reflects what's
	// actually saved to actions.yaml.
	action actions.Action

	// stage == actionEditEditing. editValue is a *string for the same
	// reason confirmScreen.result and settingsScreen.editValue are (see
	// their doc comments): huh's Value(&x) binds directly to the pointer's
	// target, which must survive this struct being copied by value.
	editingField actionEditFieldKind
	editValue    *string
	editField    huh.Field

	// stage == actionEditConfirming.
	confirmField    actionEditFieldKind
	confirmNewValue string
	confirmDialog   huh.Field
	confirmAnswer   *bool

	status    string
	statusErr bool
}

func newActionEditScreen(a actions.Action) actionEditScreen {
	return actionEditScreen{action: a}
}

func (s actionEditScreen) rawValue(f actionEditFieldKind) string {
	switch f {
	case actionEditFieldName:
		return s.action.Name
	case actionEditFieldDescription:
		return s.action.Description
	case actionEditFieldCategory:
		return s.action.Category
	case actionEditFieldTool:
		return string(s.action.Tool)
	case actionEditFieldCommand:
		return s.action.Command
	case actionEditFieldConfirm:
		return boolRawValue(s.action.Confirm)
	case actionEditFieldInteractive:
		return boolRawValue(s.action.Interactive)
	}
	return ""
}

func boolRawValue(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// yesNoDisplay renders a bool field's raw "true"/"false" value as a
// human-readable Yes/No in the current UI language.
func yesNoDisplay(raw string, languageCode string) string {
	affirmative := raw == "true"
	if languageCode == "de" {
		if affirmative {
			return "Ja"
		}
		return "Nein"
	}
	if affirmative {
		return "Yes"
	}
	return "No"
}

// displayValue is rawValue, but human-readable - only the two boolean
// fields differ (their raw value is the literal "true"/"false" stored via
// boolRawValue), and an empty value shows as "-" rather than blank so the
// row doesn't look broken.
func (m Model) actionEditDisplayValue(f actionEditFieldKind) string {
	raw := m.actionEdit.rawValue(f)
	switch f {
	case actionEditFieldConfirm, actionEditFieldInteractive:
		return yesNoDisplay(raw, m.text.LanguageCode)
	}
	return orDash(raw)
}

func (m Model) actionEditFieldLabel(f actionEditFieldKind) string {
	switch f {
	case actionEditFieldName:
		return m.text.ActionEditFieldName
	case actionEditFieldDescription:
		return m.text.ActionEditFieldDescription
	case actionEditFieldCategory:
		return m.text.FieldCategory
	case actionEditFieldTool:
		return m.text.FieldTool
	case actionEditFieldCommand:
		return m.text.FieldCommand
	case actionEditFieldConfirm:
		return m.text.ActionEditFieldConfirm
	case actionEditFieldInteractive:
		return m.text.ActionEditFieldInteractive
	}
	return ""
}

// newActionEditField builds a standalone Select or Input field for the
// given field, pre-filled with its current value. See settingsScreen's
// newSettingsEditField (same pattern) for why WithKeyMap(huh.NewDefaultKeyMap())
// is required on both: without it, a standalone huh.Select/huh.Input never
// gets its keymap populated, so arrow-key navigation silently does nothing.
func newActionEditField(m Model, field actionEditFieldKind) (huh.Field, *string) {
	width := m.fullScreenDialogWidth()
	current := new(string)
	*current = m.actionEdit.rawValue(field)

	switch field {
	case actionEditFieldTool:
		sel := huh.NewSelect[string]().
			Options(
				huh.NewOption(string(actions.ToolADB), string(actions.ToolADB)),
				huh.NewOption(string(actions.ToolScrcpy), string(actions.ToolScrcpy)),
				huh.NewOption(string(actions.ToolShell), string(actions.ToolShell)),
			).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		sel.Focus()
		return sel, current

	case actionEditFieldConfirm, actionEditFieldInteractive:
		sel := huh.NewSelect[string]().
			Options(
				huh.NewOption(yesNoDisplay("true", m.text.LanguageCode), "true"),
				huh.NewOption(yesNoDisplay("false", m.text.LanguageCode), "false"),
			).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		sel.Focus()
		return sel, current

	default: // Name, Description, Category, Command - free text
		input := huh.NewInput().Value(current)
		if field == actionEditFieldName {
			input = input.Validate(func(v string) error {
				if strings.TrimSpace(v) == "" {
					return fmt.Errorf("%s", m.text.ActionEditFieldRequiredMsg)
				}
				return nil
			})
		}
		fld := input.WithKeyMap(huh.NewDefaultKeyMap()).WithTheme(m.huhTheme).WithWidth(width)
		fld.Focus()
		return fld, current
	}
}

func (m Model) updateActionEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.actionEdit.stage {
	case actionEditEditing:
		return m.updateActionEditEditing(msg)
	case actionEditConfirming:
		return m.updateActionEditConfirming(msg)
	default:
		return m.updateActionEditBrowsing(msg)
	}
}

func (m Model) updateActionEditBrowsing(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		return m.enterDashboard(m.dashboard.serial)
	case "up", "k":
		if m.actionEdit.cursor > 0 {
			m.actionEdit.cursor--
		}
	case "down", "j":
		if m.actionEdit.cursor < len(actionEditFieldsOrder)-1 {
			m.actionEdit.cursor++
		}
	case "enter":
		field := actionEditFieldsOrder[m.actionEdit.cursor]
		m.actionEdit.status = ""
		m.actionEdit.stage = actionEditEditing
		m.actionEdit.editingField = field
		editField, editValue := newActionEditField(m, field)
		m.actionEdit.editField = editField
		m.actionEdit.editValue = editValue
	case "a":
		m.ai = newAIEditScreen(m.text, m.actionEdit.action)
		m.current = screenAI
		return m, m.ai.textarea.Focus()
	}
	return m, nil
}

func (m Model) updateActionEditEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.actionEdit.stage = actionEditBrowsing
			return m, nil
		case "enter":
			return m.submitActionEdit()
		}
	}

	updated, cmd := m.actionEdit.editField.Update(msg)
	if f, ok := updated.(huh.Field); ok {
		m.actionEdit.editField = f
	}
	return m, cmd
}

// submitActionEdit closes out the editing stage: a value equal to what's
// already saved is a no-op back to browsing, otherwise it opens the confirm
// dialog for the change - reusing newSettingsConfirmDialog as-is, since the
// "really change X to Y?" shape is identical regardless of whether X is an
// app setting or one of an action's fields.
func (m Model) submitActionEdit() (tea.Model, tea.Cmd) {
	s := m.actionEdit
	newValue := strings.TrimSpace(*s.editValue)

	if s.editingField == actionEditFieldName && newValue == "" {
		return m, nil // stay in editing; the field's own Validate already shows the error
	}

	if newValue == s.rawValue(s.editingField) {
		m.actionEdit.stage = actionEditBrowsing
		return m, nil
	}

	displayNew := newValue
	if s.editingField == actionEditFieldConfirm || s.editingField == actionEditFieldInteractive {
		displayNew = yesNoDisplay(newValue, m.text.LanguageCode)
	}

	dialog, answer := newSettingsConfirmDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), m.actionEditFieldLabel(s.editingField), displayNew)
	m.actionEdit.stage = actionEditConfirming
	m.actionEdit.confirmField = s.editingField
	m.actionEdit.confirmNewValue = newValue
	m.actionEdit.confirmDialog = dialog
	m.actionEdit.confirmAnswer = answer
	return m, nil
}

func (m Model) updateActionEditConfirming(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.actionEdit.stage = actionEditBrowsing
		return m, nil
	}

	updated, cmd := m.actionEdit.confirmDialog.Update(msg)
	if f, ok := updated.(huh.Field); ok {
		m.actionEdit.confirmDialog = f
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "tab", "y", "Y", "n", "N":
			accepted := *m.actionEdit.confirmAnswer
			field := m.actionEdit.confirmField
			value := m.actionEdit.confirmNewValue
			label := m.actionEditFieldLabel(field)

			m.actionEdit.stage = actionEditBrowsing
			if !accepted {
				return m, nil
			}

			if err := m.commitActionEditField(field, value); err != nil {
				m.actionEdit.statusErr = true
				m.actionEdit.status = fmt.Sprintf(m.text.ActionEditSaveErrorFmt, err.Error())
				return m, nil
			}
			m.actionEdit.statusErr = false
			m.actionEdit.status = fmt.Sprintf(m.text.SettingsChangeSavedFmt, label)
			return m, nil
		}
	}
	return m, cmd
}

// commitActionEditField persists exactly one field's new value: applies it
// to a copy of the current action, snapshots actions.yaml before writing
// (same backup.BeforeWrite convention used everywhere else this app
// modifies actions.yaml), and reloads Model.actionSet so the dashboard's
// list reflects the change once the user returns to it.
func (m *Model) commitActionEditField(field actionEditFieldKind, value string) error {
	a := m.actionEdit.action
	switch field {
	case actionEditFieldName:
		a.Name = value
	case actionEditFieldDescription:
		a.Description = value
	case actionEditFieldCategory:
		a.Category = value
	case actionEditFieldTool:
		a.Tool = actions.Tool(value)
	case actionEditFieldCommand:
		a.Command = value
	case actionEditFieldConfirm:
		a.Confirm = value == "true"
	case actionEditFieldInteractive:
		a.Interactive = value == "true"
	}

	err := backup.BeforeWrite(m.paths.BackupDir, m.paths.ActionsFile, func() error {
		return actions.Update(m.paths.ActionsFile, a)
	})
	if err != nil {
		return err
	}

	m.actionEdit.action = a
	if set, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML); err == nil {
		m.actionSet = set
	}
	return nil
}

func (m Model) viewActionEdit() string {
	switch m.actionEdit.stage {
	case actionEditEditing:
		return m.viewActionEditEditing()
	case actionEditConfirming:
		return m.viewActionEditConfirming()
	default:
		return m.viewActionEditBrowsing()
	}
}

func (m Model) viewActionEditBrowsing() string {
	title := m.styles.Title.Render(m.text.ActionEditTitle)
	s := m.actionEdit

	var rows strings.Builder
	for i, f := range actionEditFieldsOrder {
		marker := "  "
		if i == s.cursor {
			marker = m.styles.Highlight.Render("> ")
		}
		fmt.Fprintf(&rows, "%s%s %s\n", marker, m.actionEditFieldLabel(f), m.actionEditDisplayValue(f))
	}

	var status string
	if s.status != "" {
		style := m.styles.OK
		if s.statusErr {
			style = m.styles.Error
		}
		status = "\n" + style.Render(s.status) + "\n"
	}

	return title + "\n\n" + m.styles.Highlight.Render(s.action.ID) + "\n\n" + rows.String() + status + "\n" + m.styles.Subtle.Render(m.text.ActionEditBrowsingFooter)
}

func (m Model) viewActionEditEditing() string {
	title := m.styles.Title.Render(m.text.ActionEditTitle)
	label := m.actionEditFieldLabel(m.actionEdit.editingField)
	return title + "\n\n" + m.styles.Highlight.Render(label) + "\n\n" +
		m.actionEdit.editField.View() + "\n" + m.styles.Subtle.Render(m.text.SettingsEditingFooter)
}

func (m Model) viewActionEditConfirming() string {
	title := m.styles.Title.Render(m.text.ActionEditTitle)
	return title + "\n\n" + m.actionEdit.confirmDialog.View()
}
