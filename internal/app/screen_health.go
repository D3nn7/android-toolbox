package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/config"
	"android-toolbox/internal/healthcheck"
	"android-toolbox/internal/install"
)

type healthScreen struct {
	spinner spinner.Model
	report  healthcheck.Report
	done    bool
	err     error

	// First-run-only PATH install prompt. installAnswer is a *bool for the
	// same reason confirmScreen.result is (see its doc comment): huh.Confirm
	// binds directly to the pointer's target, which must survive
	// healthScreen being copied around as Model.health is.
	askInstall     bool
	installDialog  huh.Field
	installAnswer  *bool
	installOutcome *install.Result
	installErr     error
}

func newHealthScreen() healthScreen {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return healthScreen{spinner: s}
}

// newInstallPromptDialog builds the first-run "add to PATH?" confirm dialog.
func newInstallPromptDialog(t uiText, theme *huh.Theme, width int, alias string) (huh.Field, *bool) {
	answer := new(bool)
	dialog := huh.NewConfirm().
		Title(t.InstallPromptTitle).
		Description(fmt.Sprintf(t.InstallPromptDescFmt, alias)).
		Affirmative(t.InstallPromptYes).
		Negative(t.InstallPromptNo).
		Value(answer).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()
	return dialog, answer
}

// runHealthcheckCmd kicks off the actual startup checks in the background.
// Split out from healthScreen.Init so the splash screen (screen_splash.go)
// can start the same real check without also pulling in healthScreen's own
// spinner-ticking, which is irrelevant while the splash animation is what's
// actually on screen.
func runHealthcheckCmd(ctx context.Context, paths config.Paths, settings config.Settings) tea.Cmd {
	return func() tea.Msg {
		report := healthcheck.Run(ctx, paths, settings)
		return healthDoneMsg{report: report}
	}
}

func (h healthScreen) Init(ctx context.Context, paths config.Paths, settings config.Settings) tea.Cmd {
	return tea.Batch(h.spinner.Tick, runHealthcheckCmd(ctx, paths, settings))
}

// finalizeHealthcheck runs once the real healthcheck result is known
// (m.health.report/done already set by the caller - see updateHealthcheck
// and updateSplash): resolves adb/scrcpy, decides whether the results
// screen is shown at all (see UISettings.ShowHealthcheck), and asks the
// first-run "add to PATH?" question when applicable.
func (m Model) finalizeHealthcheck() (Model, tea.Cmd) {
	if m.health.report.HasFailures() {
		m.current = screenHealthFailed
		return m, nil
	}

	if err := m.setupTools(); err != nil {
		m.health.err = err
		m.current = screenHealthFailed
		return m, nil
	}

	m.current = screenHealthcheck
	if m.state.IsFirstRun() {
		alias := m.settings.Install.AliasName
		if alias == "" {
			alias = "atbx"
		}
		dialog, answer := newInstallPromptDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), alias)
		m.health.askInstall = true
		m.health.installDialog = dialog
		m.health.installAnswer = answer
		return m, nil
	}

	if !m.settings.UI.ShowHealthcheck {
		return m.enterToolSelect()
	}
	return m, nil
}

func (m Model) updateHealthcheck(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case healthDoneMsg:
		m.health.report = msg.report
		m.health.err = msg.err
		m.health.done = true
		return m.finalizeHealthcheck()

	case spinner.TickMsg:
		if !m.health.done {
			var cmd tea.Cmd
			m.health.spinner, cmd = m.health.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if !m.health.done {
			return m, nil
		}
		if m.health.askInstall {
			return m.updateInstallPrompt(msg)
		}
		switch msg.String() {
		case "r":
			m.health = newHealthScreen()
			m.current = screenHealthcheck
			return m, m.health.Init(m.ctx, m.paths, m.settings)
		case "q":
			return m, tea.Quit
		case "x":
			m.updateNoticeDismissed = true
			return m, nil
		case "enter", " ":
			if m.current == screenHealthFailed {
				// Only reachable here when checks passed but setupTools()
				// itself failed; nothing more to do than let the user quit.
				return m, nil
			}
			return m.enterToolSelect()
		}
	}
	return m, nil
}

// updateInstallPrompt handles the first-run-only "install to PATH?" prompt.
// Whichever way the user answers, first-run is marked complete so the
// question is never asked again.
func (m Model) updateInstallPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m.resolveInstallPrompt(false)
	}

	updated, cmd := m.health.installDialog.Update(msg)
	if field, ok := updated.(huh.Field); ok {
		m.health.installDialog = field
	}

	switch msg.String() {
	case "enter", "tab", "y", "Y", "n", "N":
		return m.resolveInstallPrompt(*m.health.installAnswer)
	}
	return m, cmd
}

// resolveInstallPrompt runs (or skips) the PATH installation per the
// dialog's answer and marks first-run complete either way.
func (m Model) resolveInstallPrompt(install bool) (tea.Model, tea.Cmd) {
	m.health.askInstall = false
	if install {
		m.runInstall()
	}
	if newState, err := config.MarkFirstRunComplete(m.paths, m.state); err == nil {
		m.state = newState
	}
	return m, nil
}

func (m *Model) runInstall() {
	exePath, err := os.Executable()
	if err != nil {
		m.health.installErr = err
		return
	}
	alias := m.settings.Install.AliasName
	if alias == "" {
		alias = "atbx"
	}
	res, err := install.Install(exePath, "android-toolbox", alias)
	m.health.installOutcome = &res
	m.health.installErr = err
}

func (m Model) viewHealthcheck() string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Render(m.text.HealthTitle))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render(appInfoText(m.text)))
	if notice := m.renderUpdateNotice(); notice != "" {
		b.WriteString("\n")
		b.WriteString(notice)
	}
	b.WriteString("\n\n")

	if !m.health.done {
		fmt.Fprintf(&b, "%s %s\n", m.health.spinner.View(), m.text.HealthChecking)
		return b.String()
	}

	for _, r := range m.health.report.Results {
		var mark string
		switch r.Severity {
		case healthcheck.OK:
			mark = m.styles.OK.Render("[ OK ]")
		case healthcheck.Warn:
			mark = m.styles.Warn.Render("[WARN]")
		default:
			mark = m.styles.Error.Render("[FAIL]")
		}
		fmt.Fprintf(&b, "%s %-28s %s\n", mark, r.Name, r.Detail)
		if r.Remediation != "" {
			fmt.Fprintf(&b, "        -> %s\n", m.styles.Subtle.Render(r.Remediation))
		}
	}

	if m.health.err != nil {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render(fmt.Sprintf(m.text.HealthInitFailedFmt, m.health.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.health.askInstall {
		b.WriteString(m.health.installDialog.View())
		return b.String()
	}

	if m.health.installOutcome != nil {
		b.WriteString(m.styles.OK.Render(fmt.Sprintf(m.text.InstallOutcomeFmt, m.health.installOutcome.InstallDir)))
		b.WriteString("\n")
		if m.health.installOutcome.Note != "" {
			b.WriteString(m.styles.Subtle.Render(m.health.installOutcome.Note))
			b.WriteString("\n")
		}
	}
	if m.health.installErr != nil {
		b.WriteString(m.styles.Warn.Render(fmt.Sprintf(m.text.InstallFailedFmt, m.health.installErr.Error())))
		b.WriteString("\n")
	}
	if m.health.installOutcome != nil || m.health.installErr != nil {
		b.WriteString("\n")
	}

	if m.current == screenHealthFailed {
		b.WriteString(m.styles.Subtle.Render(m.text.HealthFooterFailed))
	} else {
		b.WriteString(m.styles.Subtle.Render(m.text.HealthFooterDone))
	}
	return b.String()
}
