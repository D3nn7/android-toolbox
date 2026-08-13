package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/ai"
	"android-toolbox/internal/backup"
)

type aiStage int

const (
	aiStageInput aiStage = iota
	aiStageLoading
	aiStagePreview
	aiStageSaved
)

type aiScreen struct {
	stage    aiStage
	textarea textarea.Model
	spinner  spinner.Model
	err      error
	draft    actions.Action
	saveErr  error

	// editingAction is set when this screen was opened from the action
	// editor to revise an existing action (via "a") rather than create a
	// new one from the dashboard; its zero value (ID == "") means
	// "creating new". When set, it also doubles as "where to go back to" on
	// cancel/decline - the action editor for that same action, not the
	// dashboard.
	editingAction actions.Action

	// The "save this generated action?" prompt, aiStagePreview's only real
	// yes/no decision. saveAnswer is a *bool for the same reason
	// confirmScreen.result is (see its doc comment): huh.Confirm binds
	// directly to the pointer's target, which must survive aiScreen being
	// copied by value into Model.ai on every Update.
	saveDialog huh.Field
	saveAnswer *bool
}

func (s aiScreen) isEditing() bool { return s.editingAction.ID != "" }

func newAIScreen(t uiText) aiScreen {
	ta := textarea.New()
	ta.Placeholder = t.AIPlaceholder
	ta.CharLimit = 2000
	ta.SetWidth(76)
	ta.SetHeight(5)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return aiScreen{stage: aiStageInput, textarea: ta, spinner: s}
}

// newAIEditScreen is newAIScreen, but for revising an existing action from
// the action editor: it's the same input/loading/preview/saved flow, just
// with the current action passed along as context (see submitAIRequest)
// and persisted via actions.Update instead of actions.Append.
func newAIEditScreen(t uiText, existing actions.Action) aiScreen {
	s := newAIScreen(t)
	s.textarea.Placeholder = t.AIEditPlaceholder
	s.editingAction = existing
	return s
}

func (m Model) updateAI(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case aiDraftMsg:
		m.ai.stage = aiStagePreview
		m.ai.err = msg.err
		m.ai.saveDialog = nil
		m.ai.saveAnswer = nil
		if msg.err == nil {
			m.ai.draft = msg.action
			dialog, answer := newSaveActionDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), m.ai.isEditing())
			m.ai.saveDialog = dialog
			m.ai.saveAnswer = answer
		}
		return m, nil

	case actionSavedMsg:
		m.ai.saveErr = msg.err
		if msg.err == nil {
			m.ai.stage = aiStageSaved
			if set, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML); err == nil {
				m.actionSet = set
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.ai.stage == aiStageLoading {
			var cmd tea.Cmd
			m.ai.spinner, cmd = m.ai.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		switch m.ai.stage {
		case aiStageInput:
			switch msg.String() {
			case "esc":
				return m.aiExitDestination()
			case "ctrl+d":
				return m.submitAIRequest()
			}
			// Any other key (i.e. actual typing) falls through past this
			// switch and the outer one below, reaching the textarea update
			// at the bottom of the function - it must NOT hit a `return`
			// here, or every keystroke would be swallowed without ever
			// being forwarded to the textarea.

		case aiStagePreview:
			if m.ai.err != nil {
				switch msg.String() {
				case "n", "esc":
					return m.aiExitDestination()
				case "r":
					m.ai.stage = aiStageInput
					m.ai.err = nil
					return m, m.ai.textarea.Focus()
				}
				return m, nil
			}
			return m.updateAISaveDialog(msg)

		case aiStageSaved:
			switch msg.String() {
			case "enter", "esc":
				return m.aiExitDestination()
			}
			return m, nil
		}
		// Falls through to here only for aiStageInput with a key that
		// wasn't esc/ctrl+d (i.e. normal typing) - aiStagePreview and
		// aiStageSaved always return from within their own case above.
	}

	if m.ai.stage == aiStageInput {
		var cmd tea.Cmd
		m.ai.textarea, cmd = m.ai.textarea.Update(msg)
		return m, cmd
	}
	return m, nil
}

// aiExitDestination returns to wherever the AI screen was opened from:
// the action editor when regenerating an existing action (m.ai.editingAction
// is set), the dashboard for a freshly created one. Used for every way out
// of this screen - cancel, decline, and after a successful save - since the
// destination is the same in all three cases.
//
// Neither branch calls enterDashboard: the dashboard screen the user left
// behind is still fully intact (list state, scroll position, live preview),
// so rebuilding it from scratch would be both wasted work and a visible
// regression (it would also re-kick the adb info refresh unnecessarily).
func (m Model) aiExitDestination() (Model, tea.Cmd) {
	if m.ai.isEditing() {
		a := m.ai.editingAction
		if fresh, ok := m.actionSet.Find(a.ID); ok {
			a = fresh
		}
		m.actionEdit = newActionEditScreen(a)
		m.current = screenActionEdit
		return m, nil
	}
	m.current = screenDashboard
	return m, nil
}

// submitAIRequest kicks off a GenerateAction call for the current textarea
// content, guarding against an empty prompt or an unavailable provider
// before ever shelling out.
func (m Model) submitAIRequest() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.ai.textarea.Value())
	if prompt == "" {
		return m, nil
	}
	if m.aiProvider == nil {
		m.ai.stage = aiStagePreview
		m.ai.err = m.aiErr
		return m, nil
	}
	if err := m.aiProvider.Available(); err != nil {
		m.ai.stage = aiStagePreview
		m.ai.err = err
		return m, nil
	}

	existingIDs := make([]string, len(m.actionSet.Actions))
	for i, a := range m.actionSet.Actions {
		existingIDs[i] = a.ID
	}
	provider := m.aiProvider
	ctx := m.ctx
	editing := m.ai.editingAction

	m.ai.stage = aiStageLoading
	m.ai.err = nil

	generate := func() tea.Msg {
		req := ai.GenerateRequest{UserPrompt: prompt, ExistingIDs: existingIDs}
		if editing.ID != "" {
			existingDraft := ai.ActionDraftFromAction(editing)
			req.ExistingAction = &existingDraft
		}
		draft, err := provider.GenerateAction(ctx, req)
		if err != nil {
			return aiDraftMsg{err: err}
		}
		a := draft.ToAction()
		if editing.ID != "" {
			a.ID = editing.ID // never let the AI change the ID of the action being edited
		}
		return aiDraftMsg{action: a}
	}
	return m, tea.Batch(generate, m.ai.spinner.Tick)
}

// newSaveActionDialog builds the "save this generated action?" confirm
// dialog shown once a draft comes back successfully.
func newSaveActionDialog(t uiText, theme *huh.Theme, width int, editing bool) (huh.Field, *bool) {
	title := t.AISaveTitle
	if editing {
		title = t.AISaveEditTitle
	}
	answer := new(bool)
	dialog := huh.NewConfirm().
		Title(title).
		Affirmative(t.AISaveYes).
		Negative(t.AISaveNo).
		Value(answer).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()
	return dialog, answer
}

// updateAISaveDialog handles aiStagePreview once a draft is ready (i.e.
// m.ai.err == nil): "r" (reformulate) and "esc" (discard) are handled here
// as out-of-band shortcuts alongside the dialog rather than through it,
// since huh.Confirm only models a single yes/no choice and this screen
// needs a third way out.
func (m Model) updateAISaveDialog(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		return m.aiExitDestination()
	case "r":
		m.ai.stage = aiStageInput
		m.ai.err = nil
		return m, m.ai.textarea.Focus()
	}

	updated, cmd := m.ai.saveDialog.Update(key)
	if field, ok := updated.(huh.Field); ok {
		m.ai.saveDialog = field
	}

	switch key.String() {
	case "enter", "tab", "y", "Y", "n", "N":
		if !*m.ai.saveAnswer {
			return m.aiExitDestination()
		}
		action := m.ai.draft
		path := m.paths.ActionsFile
		backupDir := m.paths.BackupDir
		editing := m.ai.isEditing()
		return m, func() tea.Msg {
			err := backup.BeforeWrite(backupDir, path, func() error {
				if editing {
					return actions.Update(path, action)
				}
				return actions.Append(path, action)
			})
			return actionSavedMsg{err: err}
		}
	}
	return m, cmd
}

func (m Model) viewAI() string {
	titleText := m.text.AITitle
	if m.ai.isEditing() {
		titleText = m.text.AIEditTitle
	}
	title := m.styles.Title.Render(titleText)

	switch m.ai.stage {
	case aiStageInput:
		body := m.ai.textarea.View() + "\n\n" + m.styles.Subtle.Render(m.text.AIFooterInput)
		return title + "\n\n" + body

	case aiStageLoading:
		return title + "\n\n" + m.ai.spinner.View() + " " + m.text.AIGenerating

	case aiStagePreview:
		if m.ai.err != nil {
			return title + "\n\n" + m.styles.Error.Render(m.ai.err.Error()) + "\n\n" +
				m.styles.Subtle.Render(m.text.AIFooterErr)
		}
		a := m.ai.draft
		var b strings.Builder
		// Field names here match actions.yaml's own YAML keys (id, name,
		// description, category, tool, command, confirm, interactive,
		// params) rather than being translated - this is a preview of the
		// actual data about to be written to that file, not UI prose.
		fmt.Fprintf(&b, "id:           %s\nname:         %s\ndescription:  %s\ncategory:     %s\ntool:         %s\ncommand:      %s\nconfirm:      %v\ninteractive:  %v\n",
			a.ID, a.Name, a.Description, a.Category, a.Tool, a.Command, a.Confirm, a.Interactive)
		for _, p := range a.Params {
			fmt.Fprintf(&b, "param:        %s (%s), default=%q\n", p.Name, p.Label, p.Default)
		}
		if m.ai.saveErr != nil {
			b.WriteString("\n" + m.styles.Error.Render(m.ai.saveErr.Error()))
		}
		b.WriteString("\n\n" + m.ai.saveDialog.View())
		b.WriteString(m.styles.Subtle.Render(m.text.AIFooterReformulate))
		return title + "\n\n" + m.styles.Box.Render(b.String())

	case aiStageSaved:
		savedFmt, footer := m.text.AISavedFmt, m.text.AIFooterSaved
		if m.ai.isEditing() {
			savedFmt, footer = m.text.AIEditSavedFmt, m.text.AIFooterSavedEdit
		}
		return title + "\n\n" + m.styles.OK.Render(fmt.Sprintf(savedFmt, m.ai.draft.ID)) + "\n\n" +
			m.styles.Subtle.Render(footer)
	}
	return ""
}
