// Package app implements the interactive Bubbletea TUI: a small
// screen-stack of sub-models (healthcheck, device selection, dashboard,
// parameter form, confirmation, action runner) driven by one root Model.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"android-toolbox/internal/actions"
	"android-toolbox/internal/adb"
	"android-toolbox/internal/ai"
	"android-toolbox/internal/config"
	"android-toolbox/internal/logging"
	"android-toolbox/internal/scrcpy"
	"android-toolbox/internal/toolsmanager"
)

type screen int

const (
	screenSplash screen = iota
	screenHealthcheck
	screenHealthFailed
	screenToolSelect
	screenDeviceSelect
	screenDashboard
	screenParamForm
	screenConfirm
	screenRunner
	screenAI
	screenRecover
	screenSettings
	screenActionEdit
	screenAPKInfo
)

// Model is the root Bubbletea model. It owns everything shared across
// screens (resolved tools, the action set, the current device) and delegates
// per-screen state to small dedicated structs.
type Model struct {
	ctx      context.Context
	paths    config.Paths
	settings config.Settings
	state    config.State
	log      *logging.Logger

	styles   styles
	text     uiText
	keys     keyMap
	help     helpModel
	huhTheme *huh.Theme

	width, height int

	current screen
	err     error

	// settingsReturnTo is where "esc" on the settings screen goes back to -
	// settings is reachable from more than one screen (the dashboard and,
	// since it needs no device to be selected, the device-select screen
	// too), so it can't just always return to the dashboard.
	settingsReturnTo screen

	// deviceTicking/infoTicking track whether the respective self-rescheduling
	// tea.Tick chain has already been kicked off, so revisiting a screen
	// (e.g. switching devices repeatedly) never spawns a second, redundant
	// chain running alongside the first.
	deviceTicking bool
	infoTicking   bool

	// runnerSeq is incremented for every streamed action started, so stale
	// messages from a canceled/replaced run can be told apart from the
	// current one (see runnerScreen.runID).
	runnerSeq int
	// livePreviewSeq is the same idea for auto-run live-preview actions
	// (see dashboardScreen.livePreviewRunID).
	livePreviewSeq int

	adbClient      *adb.Client
	adbTool        toolsmanager.ResolvedTool
	scrcpyLauncher *scrcpy.Launcher
	executor       *actions.Executor
	actionSet      actions.ActionSet

	aiProvider ai.Provider
	aiErr      error

	// latestKnownVersion is the newest android-toolbox release known from
	// the background self-update check (see selfupdate_check.go), or ""
	// until a check has completed. Rendered via updateNoticeText wherever
	// the notice is shown (healthcheck screen, dashboard header).
	latestKnownVersion string
	// latestKnownADB/latestKnownScrcpy are the same idea for the
	// third-party tools (see toolsupdate_check.go) - compared against the
	// locally installed version (outdatedTools) at render time.
	latestKnownADB    string
	latestKnownScrcpy string
	// updateNoticeDismissed hides the banner built from the fields above
	// for the rest of this run, once the user dismisses it (see the "x"
	// key in screen_dashboard.go/screen_health.go).
	updateNoticeDismissed bool

	splash         splashScreen
	health         healthScreen
	toolSelect     toolSelectScreen
	deviceSelect   deviceSelectScreen
	dashboard      dashboardScreen
	paramForm      paramFormScreen
	confirm        confirmScreen
	runner         runnerScreen
	ai             aiScreen
	recover        recoverScreen
	settingsScreen settingsScreen
	actionEdit     actionEditScreen
	apkInfo        apkInfoScreen
}

// New builds the initial Model. Everything that can fail (resolving tool
// paths, loading actions) is deferred into the healthcheck screen's Init so
// failures render as a normal screen instead of crashing before the UI ever
// appears.
func New(ctx context.Context, paths config.Paths, settings config.Settings, state config.State, log *logging.Logger) Model {
	text := resolveUIText(settings.Language())
	m := Model{
		ctx:      ctx,
		paths:    paths,
		settings: settings,
		state:    state,
		log:      log,
		styles:   newStyles(),
		text:     text,
		keys:     newKeyMap(text),
		help:     newHelpModel(),
		huhTheme: androidHuhTheme(),
		current:  screenHealthcheck,
		health:   newHealthScreen(),
	}
	if settings.UI.ShowStartupAnimation {
		m.current = screenSplash
		m.splash = newSplashScreen()
	}

	// Bubbletea delivers an initial tea.WindowSizeMsg before the user ever
	// reaches a screen beyond the healthcheck, and the root Update handler
	// calls SetSize on every screen's list/viewport unconditionally (see
	// below). bubbles/list.Model is not safe to call SetSize on in its
	// zero-value state - internally it panics with a nil pointer
	// dereference in updatePagination - so every list/viewport must already
	// be a properly constructed instance here, even before its screen is
	// ever entered. They get fully replaced with real data once the user
	// actually navigates to each screen.
	m.deviceSelect.list = list.New(nil, newWrappingDelegate(2), 0, 0)
	m.dashboard.actionList = list.New(nil, newWrappingDelegate(3), 0, 0)
	m.recover.list = list.New(nil, newWrappingDelegate(2), 0, 0)
	m.runner.viewport = viewport.New(0, 0)

	return m
}

func (m Model) Init() tea.Cmd {
	startCmd := m.health.Init(m.ctx, m.paths, m.settings)
	if m.current == screenSplash {
		// The real healthcheck must run identically whether or not the
		// splash animation is shown - only kick off the decorative ramp
		// here instead of healthScreen's own (irrelevant while splash is on
		// screen) spinner-ticking Init.
		startCmd = tea.Batch(m.splash.Init(), runHealthcheckCmd(m.ctx, m.paths, m.settings))
	}
	cmds := []tea.Cmd{tea.SetWindowTitle(m.text.AppTitle), startCmd, checkForSelfUpdateCmd(m.ctx, m.paths, m.state)}
	if toolsCmd := checkForToolUpdatesCmd(m.ctx, m.paths, m.state, m.settings.UI.AutoCheckToolUpdates); toolsCmd != nil {
		cmds = append(cmds, toolsCmd)
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.width = msg.Width
		// bubbles/list and bubbles/viewport do not size themselves from
		// tea.WindowSizeMsg automatically - without this they would stay at
		// whatever (possibly zero) size they were constructed with and
		// render as empty/garbled regardless of the real terminal size.
		m.deviceSelect.list.SetSize(msg.Width, deviceListHeight(msg.Height))
		m.recover.list.SetSize(msg.Width, deviceListHeight(msg.Height))
		m.splash.progress.Width = splashProgressBarWidth(msg.Width)
		// filepicker.Model sizes itself from tea.WindowSizeMsg via its own
		// AutoHeight handling - forwarded unconditionally (like the lists
		// above) so it's already correctly sized if a resize happens while
		// the user is on a different screen.
		m.apkInfo.picker, _ = m.apkInfo.picker.Update(msg)
		m.apkInfo.viewport.Width = msg.Width
		m.apkInfo.viewport.Height = apkInfoResultViewportHeight(msg.Height)
		if m.apkInfo.stage == apkInfoResult {
			// Re-wrap the already-analyzed report at the new width rather
			// than leaving it wrapped at whatever width was current when
			// the file was selected - SetContent only resets scroll
			// position if it's now out of range, so this doesn't jump the
			// user back to the top on every resize.
			m.apkInfo.viewport.SetContent(m.apkInfoResultContent())
		}

		// The dashboard cluster (action list | operate window) splits the
		// width into two panes - both the list and the runner's viewport
		// must be sized to their own pane's *content* area (outer pane size
		// minus that pane's border+padding), not the full terminal width,
		// or they'd overflow their box.
		leftW, rightW := paneWidths(msg.Width)
		outerH := bodyHeight(msg.Height)
		leftContentW, contentH := paneContentSize(leftW, outerH)
		rightContentW, _ := paneContentSize(rightW, outerH)

		m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))

		m.runner.viewport.Width = rightContentW
		// viewRunner wraps the viewport with one title line above and one
		// status line below (see screen_runner.go), so the viewport itself
		// gets 2 fewer lines than the pane's full content height.
		runnerViewportHeight := contentH - 2
		if runnerViewportHeight < 1 {
			runnerViewportHeight = 1
		}
		m.runner.viewport.Height = runnerViewportHeight

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			// Quitting from any screen must not orphan a still-running
			// streamed action (e.g. logcat -f) or live-preview fetch:
			// cancel their contexts so the child processes are killed
			// instead of leaking past the TUI exit.
			if m.runner.cancel != nil {
				m.runner.cancel()
			}
			m.cancelLivePreview()
			return m, tea.Quit
		}
		if msg.String() == "ctrl+t" && canSwitchTool(m.current) {
			// Switching tools (Devices <-> APK Info) is meant to work at
			// any time, not just from a screen's own top level - so this is
			// handled globally, the same tier as ctrl+c, rather than only
			// wired into each individual screen's own key switch. The same
			// cleanup as quitting applies: a running streamed action or
			// live-preview fetch must not keep running unattended once its
			// screen is left behind.
			if m.runner.cancel != nil {
				m.runner.cancel()
			}
			m.cancelLivePreview()
			return m.enterToolSelect()
		}

	case deviceTickMsg:
		// Handled centrally (rather than inside updateDeviceSelect) so the
		// self-rescheduling tick keeps running even while the user is on a
		// different screen; the actual re-fetch only happens if the device
		// select screen is the one currently visible.
		cmds := []tea.Cmd{devicesTickCmd(m.deviceSelect.interval)}
		if m.current == screenDeviceSelect && m.adbClient != nil {
			cmds = append(cmds, refreshDevicesCmd(m.ctx, m.adbClient))
		}
		return m, tea.Batch(cmds...)

	case infoTickMsg:
		cmds := []tea.Cmd{infoTickCmd(m.dashboard.interval)}
		if m.current == screenDashboard && m.adbClient != nil {
			cmds = append(cmds, refreshInfoCmd(m.ctx, m.adbClient, m.dashboard.serial))
		}
		return m, tea.Batch(cmds...)

	case selfUpdateCheckMsg:
		// Handled centrally rather than per-screen since it can arrive
		// while the user is anywhere - splash, healthcheck, mid-dashboard-
		// use - and every screen that renders the notice just reads
		// m.latestKnownVersion directly (see updateNoticeText).
		m.latestKnownVersion = msg.version
		return m, nil

	case toolsUpdateCheckMsg:
		m.latestKnownADB = msg.adbLatest
		m.latestKnownScrcpy = msg.scrcpyLatest
		return m, nil
	}

	switch m.current {
	case screenSplash:
		return m.updateSplash(msg)
	case screenHealthcheck, screenHealthFailed:
		return m.updateHealthcheck(msg)
	case screenToolSelect:
		return m.updateToolSelect(msg)
	case screenDeviceSelect:
		return m.updateDeviceSelect(msg)
	case screenDashboard:
		return m.updateDashboard(msg)
	case screenParamForm:
		return m.updateParamForm(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenRunner:
		return m.updateRunner(msg)
	case screenAI:
		return m.updateAI(msg)
	case screenRecover:
		return m.updateRecover(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenActionEdit:
		return m.updateActionEdit(msg)
	case screenAPKInfo:
		return m.updateAPKInfo(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var out string
	switch m.current {
	case screenSplash:
		out = m.viewSplash()
	case screenHealthcheck, screenHealthFailed:
		out = m.viewHealthcheck()
	case screenToolSelect:
		out = m.viewToolSelect()
	case screenDeviceSelect:
		out = m.viewDeviceSelect()
	case screenDashboard, screenParamForm, screenConfirm, screenRunner:
		out = m.viewDashboardCluster()
	case screenAI:
		out = m.viewAI()
	case screenRecover:
		out = m.viewRecover()
	case screenSettings:
		out = m.viewSettings()
	case screenActionEdit:
		out = m.viewActionEdit()
	case screenAPKInfo:
		out = m.viewAPKInfo()
	}
	// Final safety net: no matter which screen rendered (or what content it
	// happened to contain - a long device path, a narrow terminal, ...), the
	// result must never exceed the actual terminal size. Anything that does
	// would make the terminal itself wrap/scroll, which desyncs bubbletea's
	// line-based repaint from what's really on screen - the exact "layout
	// breaks on resize" / "top (or bottom) of the screen isn't visible"
	// symptoms.
	//
	// terminalBottomMargin reserves the very last row rather than clamping
	// to the full reported height: many terminals auto-scroll as soon as a
	// character is written into the bottom-right cell (the classic
	// "autowrap at the last column of the last row" behavior). Filling the
	// height exactly - even though that's technically "within bounds" -
	// can still trigger that scroll and push our own top row out of view.
	// Never touching the last row at all sidesteps it entirely.
	const terminalBottomMargin = 1
	return clampToTerminal(out, m.width, m.height-terminalBottomMargin)
}

// clampToTerminal hard-truncates content that would otherwise overflow the
// terminal: excess trailing lines are dropped, and any individual line
// wider than width is cut short. It's a last-resort net, not the primary
// fix - screens are expected to size themselves correctly (see
// clampToBox/paneWidths/bodyHeight) - but guarantees correctness even if a
// screen's own budget is ever off by a line or two.
func clampToTerminal(content string, width, height int) string {
	if width < 1 || height < 1 {
		// Sizes aren't known yet (e.g. the very first frame before
		// bubbletea's initial WindowSizeMsg arrives) - leave content as-is
		// rather than mangling it based on a bogus size.
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, l := range lines {
		if lipgloss.Width(l) > width {
			lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

// setupTools resolves adb/scrcpy and loads the action set once the
// healthcheck has passed. Called after transitioning past screenHealthcheck.
func (m *Model) setupTools() error {
	mgr := toolsmanager.New(m.paths.ToolsDir)

	adbTool, err := mgr.ResolveADB()
	if err != nil {
		return err
	}
	m.adbTool = adbTool
	m.adbClient = adb.New(adbTool.Path)

	if scrcpyTool, err := mgr.ResolveScrcpy(); err == nil {
		m.scrcpyLauncher = scrcpy.New(scrcpyTool.Path, m.settings.Scrcpy.DefaultArgs, m.paths.LogsDir)
	}

	m.executor = actions.NewExecutor(m.adbTool.Path, m.scrcpyLauncher)

	set, err := actions.Load(m.paths.ActionsFile, actions.DefaultActionsYAML)
	if err != nil {
		return err
	}
	m.actionSet = set

	// AI provider construction failure (e.g. an unknown provider name in
	// settings.yaml) is not fatal to the rest of the app - it only surfaces
	// when the user actually opens the AI screen.
	provider, aiErr := ai.New(m.settings.AI.Provider, m.settings.AI.Claude.Command, m.settings.AI.Claude.TimeoutSeconds, m.paths.AIPromptFile)
	m.aiProvider = provider
	m.aiErr = aiErr

	return nil
}

// deviceListHeight/dashboardListHeight compute a list's body height from the
// full terminal height, leaving room for this app's title/footer/info-panel
// chrome around it. Shared between the resize handler and the screen-entry
// helpers below, since a freshly constructed list.Model starts at size 0x0
// and only bubbletea's *next* WindowSizeMsg would otherwise resize it - and
// on a real terminal that only fires on an actual resize, which most
// sessions never trigger, leaving the list permanently invisible.
func deviceListHeight(totalHeight int) int {
	h := totalHeight - 4
	if h < 0 {
		return 0
	}
	return h
}

// enterDeviceSelect switches to the device-select screen and, the first
// time only, kicks off its self-rescheduling refresh tick.
func (m Model) enterDeviceSelect() (Model, tea.Cmd) {
	ds, cmd := newDeviceSelectScreen(m.ctx, m.adbClient, m.settings)
	m.deviceSelect = ds
	m.deviceSelect.list.SetSize(m.width, deviceListHeight(m.height))
	m.current = screenDeviceSelect
	if !m.deviceTicking && m.adbClient != nil {
		m.deviceTicking = true
		cmd = tea.Batch(cmd, devicesTickCmd(m.deviceSelect.interval))
	}
	return m, cmd
}

// enterDashboard switches to the dashboard screen for serial and, the first
// time only, kicks off its self-rescheduling info refresh tick.
func (m Model) enterDashboard(serial string) (Model, tea.Cmd) {
	dash, cmd := newDashboardScreen(m.ctx, m.adbClient, m.actionSet, serial, m.settings, m.text)
	m.dashboard = dash
	leftW, _ := paneWidths(m.width)
	leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
	m.dashboard.actionList.SetSize(leftContentW, dashboardListHeight(contentH))
	m.current = screenDashboard
	if !m.infoTicking && m.adbClient != nil {
		m.infoTicking = true
		cmd = tea.Batch(cmd, infoTickCmd(m.dashboard.interval))
	}
	// If the first selected action happens to be live-preview-eligible
	// (e.g. it's the top of the list), show it immediately rather than
	// waiting for the user to navigate away and back.
	m, previewCmd := m.syncLivePreview()
	return m, tea.Batch(cmd, previewCmd)
}

func devicesTickCmd(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg { return deviceTickMsg{at: t} })
}

func infoTickCmd(interval time.Duration) tea.Cmd {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg { return infoTickMsg{at: t} })
}

func refreshDevicesCmd(ctx context.Context, client *adb.Client) tea.Cmd {
	return func() tea.Msg {
		devices, err := client.ListDevices(ctx)
		return devicesRefreshedMsg{devices: devices, err: err}
	}
}

func fmtErr(prefix string, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}
