package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/adb"
	"android-toolbox/internal/config"
	"android-toolbox/internal/device"
)

// actionItem carries its already-resolved category label (a.Category, or
// the translated fallback when empty) rather than resolving the fallback
// lazily in Description(): that method comes from bubbles/list's Item
// interface, so it can't take a uiText parameter - resolving it once, when
// the item is built (see buildActionItems), is what lets it stay
// language-aware without a package-level "current language" global.
//
// editable marks actions the user can change via the action editor (see
// screen_action_editor.go) - anything NOT shipped in
// actions.DefaultActionsYAML, i.e. added by hand or via the AI feature.
// Built-in actions stay as shipped; badge is the already-translated marker
// shown next to an editable action's title.
type actionItem struct {
	a        actions.Action
	category string
	editable bool
	badge    string
}

func (i actionItem) Title() string {
	if i.editable {
		return i.a.Name + "  " + i.badge
	}
	return i.a.Name
}
func (i actionItem) Description() string {
	return fmt.Sprintf("[%s] %s", i.category, i.a.Description)
}
func (i actionItem) FilterValue() string {
	return i.a.ID + " " + i.a.Name + " " + i.a.Category + " " + i.a.Description
}

type dashboardScreen struct {
	serial     string
	info       device.Info
	infoErr    error
	actionList list.Model
	interval   time.Duration
	status     string
	statusErr  bool

	// Category filter pills shown above the action list. actionSet is kept
	// around so switching categories can rebuild the list's items without
	// needing the caller to pass the whole set back in. categories[0] is
	// always "" (rendered as "Alle" - no filter); categories[1:] are the
	// real Action.Category values in actions.yaml's file order, from
	// ActionSet.ByCategory().
	actionSet   actions.ActionSet
	categories  []string
	categoryIdx int

	// Live preview: actions.Action.LivePreviewEligible actions auto-run and
	// show their result just from being highlighted, no enter required.
	// livePreviewActionID/livePreviewRunID identify which action and run
	// livePreviewOutput/livePreviewErr/livePreviewLoading belong to, so a
	// result for an item the user already navigated away from is
	// recognizable as stale and dropped (see updateDashboard's
	// livePreviewMsg case).
	livePreviewActionID string
	livePreviewRunID    int
	livePreviewOutput   string
	livePreviewErr      error
	livePreviewLoading  bool
	livePreviewCancel   context.CancelFunc
}

// categoryDisplayLabel translates a raw ActionSet.ByCategory group name (or
// the "" sentinel meaning "no filter selected") into what should actually be
// shown: ByCategory groups uncategorized actions under the fixed internal
// string "General" regardless of UI language, "" only ever means the
// leading "show everything" pill, and the built-in categories (Files,
// Device, Network, Display) have their own German translation - all three
// need mapping to the current language's label rather than a category name
// passing straight through untranslated. A user-created action's category
// is never in builtinCategoryTranslationsDE, so it always passes through
// unchanged, in whatever language it was written in.
func categoryDisplayLabel(raw string, t uiText) string {
	switch raw {
	case "":
		return t.CategoryAll
	case "General":
		return t.CategoryFallback
	}
	if t.LanguageCode == "de" {
		if translated, ok := builtinCategoryTranslationsDE[raw]; ok {
			return translated
		}
	}
	return raw
}

// buildActionItems returns list items for set's actions, restricted to a
// single category when category is non-empty (category == "" means no
// filter - every action, grouped in ActionSet.ByCategory's order, same as
// before category pills existed). Each item's Name/Description is
// localized via localizeAction - a no-op for anything but a recognized
// built-in action in German mode.
func buildActionItems(set actions.ActionSet, category string, t uiText) []list.Item {
	var items []list.Item
	for _, group := range set.ByCategory() {
		if category != "" && group.Category != category {
			continue
		}
		label := categoryDisplayLabel(group.Category, t)
		for _, a := range group.Actions {
			items = append(items, actionItem{
				a:        localizeAction(a, t),
				category: label,
				editable: !actions.IsBuiltinID(a.ID),
				badge:    t.ActionEditableBadge,
			})
		}
	}
	return items
}

func newDashboardScreen(ctx context.Context, client *adb.Client, set actions.ActionSet, serial string, settings config.Settings, t uiText) (dashboardScreen, tea.Cmd) {
	categories := []string{""}
	for _, group := range set.ByCategory() {
		categories = append(categories, group.Category)
	}

	// 3 description lines instead of bubbles' default 1: real action
	// descriptions run up to ~75 chars (e.g. "[Anzeige] Nutzt das Gerät
	// als reines USB-HID Ziel ohne Bildspiegelung"), comfortably needing
	// 2-3 wrapped lines even at the narrowest pane width - the default
	// delegate hard-truncated a single line instead of wrapping at all.
	l := list.New(buildActionItems(set, "", t), newWrappingDelegate(3), 0, 0)
	l.Styles = androidListStyles()
	// The category pills row (see renderCategoryPills) takes over the list's
	// own title bar's job of naming what's shown, so the built-in title bar
	// is just redundant space here.
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	// The dashboard cluster renders its own footer via bubbles/help
	// (dashboardClusterFooter); the list's built-in one would otherwise
	// show bubbles' default bindings inside the pane box, redundant with
	// and inconsistent from ours.
	l.SetShowHelp(false)
	l.FilterInput.Placeholder = t.ActionFilterPlaceholder

	interval := time.Duration(settings.Devices.RefreshIntervalSeconds) * time.Second
	d := dashboardScreen{serial: serial, actionList: l, interval: interval, actionSet: set, categories: categories}

	// Note: the recurring tick chain is kicked off by the caller (see
	// app.go's enterDashboard) at most once per app lifetime.
	return d, refreshInfoCmd(ctx, client, serial)
}

func refreshInfoCmd(ctx context.Context, client *adb.Client, serial string) tea.Cmd {
	return func() tea.Msg {
		info, err := device.Collect(ctx, client, serial)
		return deviceInfoMsg{info: info, err: err}
	}
}

// startLivePreviewCmd runs a live-preview-eligible action to completion and
// returns its full output in one message. Unlike the runner screen this
// doesn't stream line-by-line - live-preview actions are required to be
// quick, side-effect-free reads (see LivePreviewEligible), so waiting for
// the whole result is simpler and plenty fast.
func startLivePreviewCmd(ctx context.Context, executor *actions.Executor, a actions.Action, serial string, runID int) tea.Cmd {
	return func() tea.Msg {
		ra, err := executor.Start(ctx, a, serial, nil)
		if err != nil {
			return livePreviewMsg{runID: runID, actionID: a.ID, err: err}
		}
		data, _ := io.ReadAll(ra.Output)
		waitErr := ra.Wait()
		return livePreviewMsg{runID: runID, actionID: a.ID, output: string(data), err: waitErr}
	}
}

// cancelLivePreview stops any in-flight live-preview run and clears its
// cached state - used both when the selection moves to an ineligible
// action and when leaving the dashboard entirely (device switch, opening
// the AI/backups screens, running an action, quitting).
func (m *Model) cancelLivePreview() {
	if m.dashboard.livePreviewCancel != nil {
		m.dashboard.livePreviewCancel()
	}
	m.dashboard.livePreviewActionID = ""
	m.dashboard.livePreviewOutput = ""
	m.dashboard.livePreviewErr = nil
	m.dashboard.livePreviewLoading = false
	m.dashboard.livePreviewCancel = nil
}

// syncLivePreview checks the currently highlighted action against whatever
// live-preview run is already in flight/cached, starting a new one (and
// canceling any stale one) if the selection moved to a different
// live-preview-eligible action, or clearing the preview state if it moved
// to an ineligible one. Called after every list navigation so the right
// pane always reflects whatever is currently highlighted.
func (m Model) syncLivePreview() (Model, tea.Cmd) {
	item, ok := m.dashboard.actionList.SelectedItem().(actionItem)
	if !ok || !item.a.LivePreviewEligible() {
		m.cancelLivePreview()
		return m, nil
	}

	if item.a.ID == m.dashboard.livePreviewActionID {
		return m, nil // already the one being shown/loaded - nothing to do
	}

	if m.dashboard.livePreviewCancel != nil {
		m.dashboard.livePreviewCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.livePreviewSeq++
	m.dashboard.livePreviewActionID = item.a.ID
	m.dashboard.livePreviewRunID = m.livePreviewSeq
	m.dashboard.livePreviewOutput = ""
	m.dashboard.livePreviewErr = nil
	m.dashboard.livePreviewLoading = true
	m.dashboard.livePreviewCancel = cancel
	return m, startLivePreviewCmd(ctx, m.executor, item.a, m.dashboard.serial, m.livePreviewSeq)
}

// cycleCategory moves the selected category pill by delta (wrapping around
// both ends) and rebuilds the action list to show only that category's
// actions ("" - the first entry, "Alle" - shows everything, unfiltered).
func (m Model) cycleCategory(delta int) (tea.Model, tea.Cmd) {
	m.cancelLivePreview()

	n := len(m.dashboard.categories)
	if n == 0 {
		return m, nil
	}
	idx := (m.dashboard.categoryIdx + delta) % n
	if idx < 0 {
		idx += n
	}
	m.dashboard.categoryIdx = idx

	items := buildActionItems(m.dashboard.actionSet, m.dashboard.categories[idx], m.text)
	cmd := m.dashboard.actionList.SetItems(items)
	m.dashboard.actionList.ResetSelected()

	m, previewCmd := m.syncLivePreview()
	return m, tea.Batch(cmd, previewCmd)
}

func (m Model) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deviceInfoMsg:
		m.dashboard.info = msg.info
		m.dashboard.infoErr = msg.err
		return m, nil

	case scrcpyStartedMsg:
		if msg.err != nil {
			m.dashboard.status = msg.err.Error()
			m.dashboard.statusErr = true
		} else {
			m.dashboard.status = fmt.Sprintf(m.text.ScrcpyStartedFmt, msg.pid)
			m.dashboard.statusErr = false
		}
		return m, nil

	case statusMsg:
		m.dashboard.status = msg.text
		m.dashboard.statusErr = msg.isErr
		return m, nil

	case actionDoneMsg:
		// Result of a tea.ExecProcess'd interactive action.
		if msg.err != nil {
			m.dashboard.status = fmt.Sprintf(m.text.ActionFinishedErrFmt, msg.err.Error())
			m.dashboard.statusErr = true
		} else {
			m.dashboard.status = m.text.ActionFinishedOK
			m.dashboard.statusErr = false
		}
		return m, nil

	case livePreviewMsg:
		if msg.runID != m.dashboard.livePreviewRunID || msg.actionID != m.dashboard.livePreviewActionID {
			return m, nil // stale - the user already moved on to a different action
		}
		m.dashboard.livePreviewLoading = false
		m.dashboard.livePreviewOutput = msg.output
		m.dashboard.livePreviewErr = msg.err
		return m, nil

	case tea.KeyMsg:
		if m.dashboard.actionList.FilterState() != list.Filtering {
			switch msg.String() {
			case "q":
				m.cancelLivePreview()
				return m, tea.Quit
			case "x":
				m.updateNoticeDismissed = true
				return m, nil
			case "ctrl+g":
				m.cancelLivePreview()
				return m.enterDeviceSelect()
			case "a":
				m.cancelLivePreview()
				m.ai = newAIScreen(m.text)
				m.current = screenAI
				return m, m.ai.textarea.Focus()
			case "b":
				m.cancelLivePreview()
				m.recover = newRecoverScreen(m.paths.BackupDir, m.text)
				m.recover.list.SetSize(m.width, deviceListHeight(m.height))
				m.current = screenRecover
				return m, nil
			case "s":
				m.cancelLivePreview()
				m.settingsScreen = newSettingsScreen(m)
				m.settingsReturnTo = screenDashboard
				m.current = screenSettings
				return m, nil
			case "e":
				if item, ok := m.dashboard.actionList.SelectedItem().(actionItem); ok {
					if !item.editable {
						m.dashboard.status = m.text.ActionNotEditableStatus
						m.dashboard.statusErr = true
						return m, nil
					}
					m.cancelLivePreview()
					m.actionEdit = newActionEditScreen(item.a)
					m.current = screenActionEdit
				}
				return m, nil
			case "tab":
				return m.cycleCategory(1)
			case "shift+tab":
				return m.cycleCategory(-1)
			case "enter":
				if item, ok := m.dashboard.actionList.SelectedItem().(actionItem); ok {
					m.cancelLivePreview()
					return m.dispatchAction(item.a)
				}
			}
		}
	}

	var cmd tea.Cmd
	m.dashboard.actionList, cmd = m.dashboard.actionList.Update(msg)

	m, previewCmd := m.syncLivePreview()
	return m, tea.Batch(cmd, previewCmd)
}

// dispatchAction routes a selected action to the right next screen: a
// parameter form if it needs input, a confirmation prompt if marked
// destructive, or straight to execution otherwise.
func (m Model) dispatchAction(a actions.Action) (tea.Model, tea.Cmd) {
	if len(a.Params) > 0 {
		m.paramForm = newParamFormScreen(a)
		m.current = screenParamForm
		return m, nil
	}
	if a.Confirm {
		m.confirm = newConfirmScreen(a, nil, m.text, m.huhTheme, m.rightPaneContentWidth())
		m.current = screenConfirm
		return m, nil
	}
	return m.beginExecution(a, nil)
}

// beginExecution starts a (params already resolved) action: scrcpy actions
// launch detached and stay on the dashboard, interactive actions hand the
// terminal over via tea.ExecProcess, and everything else moves to the
// streaming runner screen.
func (m Model) beginExecution(a actions.Action, params map[string]string) (tea.Model, tea.Cmd) {
	serial := m.dashboard.serial

	if a.Tool == actions.ToolScrcpy {
		m.current = screenDashboard
		executor := m.executor
		cmd := func() tea.Msg {
			proc, err := executor.StartScrcpy(a, serial, params)
			if err != nil {
				return scrcpyStartedMsg{err: err}
			}
			return scrcpyStartedMsg{pid: proc.Process.Pid}
		}
		return m, cmd
	}

	if a.Interactive {
		cmd, err := m.executor.Prepare(m.ctx, a, serial, params)
		if err != nil {
			m.dashboard.status = err.Error()
			m.dashboard.statusErr = true
			m.current = screenDashboard
			return m, nil
		}
		m.current = screenDashboard
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return actionDoneMsg{err: err}
		})
	}

	_, rightW := paneWidths(m.width)
	rightContentW, contentH := paneContentSize(rightW, bodyHeight(m.height))
	// viewRunner wraps the viewport with one title line above and one status
	// line below (see screen_runner.go), so the viewport gets 2 fewer lines
	// than the pane's full content height.
	runnerHeight := contentH - 2
	if runnerHeight < 1 {
		runnerHeight = 1
	}
	m.runnerSeq++
	runner, cmd := newRunnerScreen(m.ctx, m.executor, a, serial, params, rightContentW, runnerHeight, m.runnerSeq)
	m.runner = runner
	m.current = screenRunner
	return m, cmd
}

// viewDashboard renders the RIGHT pane's content while browsing (i.e. no
// action is being parameterized/confirmed/run yet): a preview of whatever
// is currently highlighted in the left action list, plus the most recent
// status message (e.g. "scrcpy gestartet", a background action's result).
// The left pane (the action list itself) and the header/footer chrome are
// composed separately in app.go, since they're shared across the whole
// dashboard cluster (see isDashboardCluster).
func (m Model) viewDashboard() string {
	var b strings.Builder

	item, ok := m.dashboard.actionList.SelectedItem().(actionItem)
	switch {
	case !ok:
		b.WriteString(m.styles.Subtle.Render(m.text.NoActions))
	case item.a.LivePreviewEligible() && m.dashboard.livePreviewActionID == item.a.ID:
		m.writeLivePreview(&b, item)
	default:
		m.writeActionPreview(&b, item)
	}

	if m.dashboard.status != "" {
		style := m.styles.OK
		if m.dashboard.statusErr {
			style = m.styles.Error
		}
		b.WriteString("\n\n" + style.Render(m.dashboard.status))
	}

	return b.String()
}

// writeActionPreview renders the static metadata view of a highlighted
// action: what it is, what it'll do, and that enter is needed to run it.
func (m Model) writeActionPreview(b *strings.Builder, item actionItem) {
	a := item.a
	cat := item.category

	fmt.Fprintf(b, "%s\n", m.styles.Highlight.Render(a.Name))
	if a.Description != "" {
		fmt.Fprintf(b, "%s\n", a.Description)
	}
	// Labels are padded to a fixed column width (rather than a hardcoded
	// number of trailing spaces per label) so values still line up whether
	// the current language's label text is "Kategorie:" or "Category:".
	const fieldColumnWidth = 12
	field := func(label string) string { return fmt.Sprintf("%-*s", fieldColumnWidth, label) }

	b.WriteString("\n")
	fmt.Fprintf(b, "%s%s\n", field(m.text.FieldCategory), cat)
	fmt.Fprintf(b, "%s%s\n", field(m.text.FieldTool), a.Tool)
	if a.Command != "" {
		fmt.Fprintf(b, "%s%s\n", field(m.text.FieldCommand), a.Command)
	}
	if len(a.Params) > 0 {
		names := make([]string, len(a.Params))
		for i, p := range a.Params {
			names[i] = p.Name
		}
		fmt.Fprintf(b, "%s%s\n", field(m.text.FieldParams), strings.Join(names, ", "))
	}

	var hints []string
	if a.Confirm {
		hints = append(hints, m.text.HintConfirmNeeded)
	}
	if a.Interactive {
		hints = append(hints, m.text.HintInteractive)
	}
	if len(hints) > 0 {
		fmt.Fprintf(b, "%s%s\n", field(m.text.FieldHint), strings.Join(hints, ", "))
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render(m.text.RunHint))
}

// writeLivePreview renders a live-preview-eligible action's auto-run
// result: its own output (highlighted per its Format, same as the runner
// screen) instead of the static metadata view - this is what makes
// highlighting "Akku-Status" or "Geräteinformationen" show the answer
// immediately, without pressing enter.
func (m Model) writeLivePreview(b *strings.Builder, item actionItem) {
	a := item.a
	fmt.Fprintf(b, "%s\n", m.styles.Highlight.Render(a.Name))
	b.WriteString(m.styles.Subtle.Render(m.text.LivePreviewLabel))
	b.WriteString("\n\n")

	switch {
	case m.dashboard.livePreviewLoading:
		b.WriteString(m.styles.Info.Render(m.text.LivePreviewLoading))
	case m.dashboard.livePreviewErr != nil:
		b.WriteString(m.styles.Error.Render(m.dashboard.livePreviewErr.Error()))
	default:
		output := strings.TrimRight(m.dashboard.livePreviewOutput, "\n")
		if output == "" {
			b.WriteString(m.styles.Subtle.Render(m.text.NoOutput))
			break
		}
		for _, line := range strings.Split(output, "\n") {
			b.WriteString(m.renderOutputLine(a.Format, line))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render(m.text.LivePreviewOpenHint))
}
