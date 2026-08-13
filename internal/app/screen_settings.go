package app

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/ai"
	"android-toolbox/internal/buildinfo"
	"android-toolbox/internal/config"
	"android-toolbox/internal/selfupdate"
	"android-toolbox/internal/toolsmanager"
)

// settingsFieldKind identifies one of the settings screen's editable rows.
type settingsFieldKind int

const (
	settingsFieldLanguage settingsFieldKind = iota
	settingsFieldAIProvider
	settingsFieldAICommand
	settingsFieldAITimeout
	settingsFieldShowStartupAnimation
	settingsFieldShowHealthcheck
	settingsFieldAutoCheckToolUpdates
	// settingsFieldCheckForUpdatesNow is not a persisted value like the
	// others - pressing enter on it triggers a live update check instead of
	// opening the usual edit/confirm flow (see updateSettingsBrowsing and
	// runSettingsUpdateCheckCmd).
	settingsFieldCheckForUpdatesNow
)

var settingsFieldsOrder = [...]settingsFieldKind{
	settingsFieldLanguage,
	settingsFieldAIProvider,
	settingsFieldAICommand,
	settingsFieldAITimeout,
	settingsFieldShowStartupAnimation,
	settingsFieldShowHealthcheck,
	settingsFieldAutoCheckToolUpdates,
	settingsFieldCheckForUpdatesNow,
}

// settingsStage is where the settings screen currently is: browsing the
// list of settings, editing whichever one was selected, or confirming that
// edit before it's actually applied. Changing a setting always goes
// browsing -> editing -> confirming -> browsing (or back to browsing early
// via esc/no-op), one field at a time - never a multi-field form the user
// has to tab all the way through, and never a second pane/screen the way
// the dashboard's action list vs. operate window is split.
type settingsStage int

const (
	settingsBrowsing settingsStage = iota
	settingsEditing
	settingsConfirming
)

type settingsScreen struct {
	stage  settingsStage
	cursor int // index into settingsFieldsOrder; valid while stage == settingsBrowsing

	// Current, already-committed values - refreshed after every successful
	// commit (see Model.commitSettingsField) so the browsing list always
	// reflects what's actually saved to settings.yaml, not an in-progress
	// edit that might still be discarded.
	language   string
	aiProvider string
	aiCommand  string
	aiTimeout  string // kept as a string; parsed to int only on commit

	showStartupAnimation string // "true"/"false", same string-encoded-bool scheme as actionEditScreen's boolRawValue
	showHealthcheck      string
	autoCheckToolUpdates string

	// checkNowStatus is settingsFieldCheckForUpdatesNow's display value -
	// "" before the first check, then a transient "checking" message, then
	// a short result summary. Not persisted; reset each time the screen is
	// (re)constructed.
	checkNowStatus string

	// stage == settingsEditing. editValue is a *string (not a string)
	// deliberately, for the same reason confirmScreen.result is a *bool
	// (see its doc comment): huh's Value(&x) accessor binds directly to the
	// pointer's target, and settingsScreen is copied by value every time
	// it's reassigned into Model.settingsScreen (Go's usual bubbletea
	// pattern) - a plain string field would leave the bound accessor
	// pointing at a field on a since-discarded copy, and typing/selecting
	// would silently stop updating anything visible.
	editingField settingsFieldKind
	editValue    *string
	editField    huh.Field

	// stage == settingsConfirming.
	confirmField    settingsFieldKind
	confirmNewValue string
	confirmDialog   huh.Field
	confirmAnswer   *bool

	status    string
	statusErr bool
}

func newSettingsScreen(m Model) settingsScreen {
	return settingsScreen{
		language:             m.settings.Language(),
		aiProvider:           m.settings.AI.Provider,
		aiCommand:            m.settings.AI.Claude.Command,
		aiTimeout:            strconv.Itoa(m.settings.AI.Claude.TimeoutSeconds),
		showStartupAnimation: boolRawValue(m.settings.UI.ShowStartupAnimation),
		showHealthcheck:      boolRawValue(m.settings.UI.ShowHealthcheck),
		autoCheckToolUpdates: boolRawValue(m.settings.UI.AutoCheckToolUpdates),
	}
}

func (s settingsScreen) rawValue(f settingsFieldKind) string {
	switch f {
	case settingsFieldLanguage:
		return s.language
	case settingsFieldAIProvider:
		return s.aiProvider
	case settingsFieldAICommand:
		return s.aiCommand
	case settingsFieldAITimeout:
		return s.aiTimeout
	case settingsFieldShowStartupAnimation:
		return s.showStartupAnimation
	case settingsFieldShowHealthcheck:
		return s.showHealthcheck
	case settingsFieldAutoCheckToolUpdates:
		return s.autoCheckToolUpdates
	}
	return ""
}

// displayValue is rawValue, but human-readable: Language shows its full name
// rather than the "en"/"de" code stored in settings.yaml, and the two on/off
// toggles show a plain "Enabled"/"Disabled" word (see boolDisplayWord)
// rather than their raw "true"/"false" storage value.
func (s settingsScreen) displayValue(f settingsFieldKind, t uiText) string {
	switch f {
	case settingsFieldLanguage:
		return languageDisplayName(s.language)
	case settingsFieldShowStartupAnimation, settingsFieldShowHealthcheck, settingsFieldAutoCheckToolUpdates:
		return boolDisplayWord(s.rawValue(f), t)
	case settingsFieldCheckForUpdatesNow:
		if s.checkNowStatus != "" {
			return s.checkNowStatus
		}
		return t.SettingsCheckForUpdatesIdleHint
	}
	return s.rawValue(f)
}

func languageDisplayName(code string) string {
	if code == "de" {
		return "Deutsch"
	}
	return "English"
}

// boolDisplayWord renders a bool field's "true"/"false" raw value as a
// plain word - used for the browsing row, the edit-stage Select's two
// options, and the "change X to Y?" confirm dialog's sentence alike.
func boolDisplayWord(raw string, t uiText) string {
	if raw == "true" {
		return t.SettingsEnabledLabel
	}
	return t.SettingsDisabledLabel
}

func (m Model) settingsFieldLabel(f settingsFieldKind) string {
	switch f {
	case settingsFieldLanguage:
		return m.text.SettingsLanguageLabel
	case settingsFieldAIProvider:
		return m.text.SettingsAIProviderLabel
	case settingsFieldAICommand:
		return m.text.SettingsAICommandLabel
	case settingsFieldAITimeout:
		return m.text.SettingsAITimeoutLabel
	case settingsFieldShowStartupAnimation:
		return m.text.SettingsShowStartupAnimationLabel
	case settingsFieldShowHealthcheck:
		return m.text.SettingsShowHealthcheckLabel
	case settingsFieldAutoCheckToolUpdates:
		return m.text.SettingsAutoCheckToolUpdatesLabel
	case settingsFieldCheckForUpdatesNow:
		return m.text.SettingsCheckForUpdatesLabel
	}
	return ""
}

// settingsToolInfo renders the read-only "what did we resolve" block: adb/
// scrcpy paths already resolved at startup (see Model.setupTools) plus the
// config paths in use - reusing that already-resolved state instead of
// re-running healthcheck.Run's I/O just to redisplay the same information.
func settingsToolInfo(m Model) string {
	adbLine := fmt.Sprintf(m.text.SettingsToolADBFmt, m.adbTool.Path, m.adbTool.Source)

	scrcpyPath := m.text.SettingsToolNotResolved
	if m.scrcpyLauncher != nil {
		scrcpyPath = m.scrcpyLauncher.BinPath
	}
	scrcpyLine := fmt.Sprintf(m.text.SettingsToolScrcpyFmt, scrcpyPath)

	configDirLine := fmt.Sprintf(m.text.SettingsToolConfigDirFmt, m.paths.ConfigDir)
	actionsLine := fmt.Sprintf(m.text.SettingsToolActionsFmt, m.paths.ActionsFile, len(m.actionSet.Actions))

	return adbLine + "\n" + scrcpyLine + "\n" + configDirLine + "\n" + actionsLine
}

// settingsUpdateInfo renders the read-only "what do we know about updates"
// block: android-toolbox's own version plus adb/scrcpy, each compared
// against whatever the periodic background checks (or a manual "check for
// updates now") last found. Purely a rendering of already-known state - no
// I/O of its own, unlike runSettingsUpdateCheckCmd.
func settingsUpdateInfo(m Model) string {
	mgr := toolsmanager.New(m.paths.ToolsDir)
	lines := []string{
		toolboxUpdateSummaryLine(m.text, m.latestKnownVersion),
		toolUpdateSummaryLine("adb", mgr.InstalledADBVersion(runtime.GOOS), m.latestKnownADB, m.text),
		toolUpdateSummaryLine("scrcpy", mgr.InstalledScrcpyVersion(runtime.GOOS, runtime.GOARCH), m.latestKnownScrcpy, m.text),
	}
	return strings.Join(lines, "\n")
}

func toolboxUpdateSummaryLine(t uiText, latestKnown string) string {
	status := t.SettingsUpdateNotCheckedYetLabel
	if latestKnown != "" {
		status = t.SettingsUpToDateLabel
		if selfupdate.IsNewer(buildinfo.Version, latestKnown) {
			status = t.SettingsUpdateAvailableLabel
		}
	}
	return fmt.Sprintf("android-toolbox: %s (%s)", buildinfo.Version, status)
}

// toolUpdateSummaryLine is toolboxUpdateSummaryLine for adb/scrcpy: their
// "latest" values aren't semantic versions (an ETag or a git tag), so a
// plain inequality is the comparison - same rule toolsmanager.ToolUpdateStatus
// itself uses.
func toolUpdateSummaryLine(name, installed, latestKnown string, t uiText) string {
	status := t.SettingsUpdateNotCheckedYetLabel
	if latestKnown != "" {
		status = t.SettingsUpToDateLabel
		if installed != latestKnown {
			status = t.SettingsUpdateAvailableLabel
		}
	}
	return fmt.Sprintf("%s: %s (%s)", name, orDash(installed), status)
}

// appInfoText renders the version/repo block shown on both the healthcheck
// and settings screens.
func appInfoText(t uiText) string {
	return fmt.Sprintf(t.AppInfoVersionFmt, buildinfo.Version, buildinfo.Commit) + "\n" +
		fmt.Sprintf(t.AppInfoRepoFmt, buildinfo.RepoURL)
}

// providerOptionLabel appends a live "installed"/"not installed" status to
// a provider's option label - checked against the command currently in the
// settings (i.e. what would actually be invoked), so switching to a
// provider whose CLI isn't on this machine is visible before saving,
// instead of only failing later when the AI screen is opened.
func providerOptionLabel(t uiText, name, command string, timeoutSeconds int, promptPath string) string {
	installed := false
	if provider, err := ai.New(name, command, timeoutSeconds, promptPath); err == nil {
		installed = provider.Available() == nil
	}
	if installed {
		return fmt.Sprintf(t.SettingsProviderInstalledFmt, name)
	}
	return fmt.Sprintf(t.SettingsProviderNotInstalledFmt, name)
}

// newSettingsEditField builds a standalone Select or Input field for the
// given field, pre-filled with its current value. The returned *string is
// where huh writes live as the user navigates/types (see settingsScreen's
// editValue doc comment for why it must be a freshly allocated pointer
// rather than the address of an existing struct field).
//
// WithKeyMap(huh.NewDefaultKeyMap()) is required on both: a standalone
// huh.Select/huh.Input (as opposed to one embedded in a huh.Form, which
// wires this up itself) never gets its keymap populated on its own - without
// it, arrow-key navigation on a Select silently does nothing at all. This is
// the exact same bug class documented in newConfirmScreen; see its comment
// for the fuller explanation.
func newSettingsEditField(m Model, field settingsFieldKind) (huh.Field, *string) {
	width := m.fullScreenDialogWidth()
	current := new(string)
	*current = m.settingsScreen.rawValue(field)

	switch field {
	case settingsFieldLanguage:
		sel := huh.NewSelect[string]().
			Options(huh.NewOption("English", "en"), huh.NewOption("Deutsch", "de")).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		sel.Focus()
		return sel, current

	case settingsFieldAIProvider:
		timeoutSeconds, _ := strconv.Atoi(m.settingsScreen.aiTimeout)
		names := ai.Names()
		opts := make([]huh.Option[string], 0, len(names))
		for _, name := range names {
			label := providerOptionLabel(m.text, name, m.settingsScreen.aiCommand, timeoutSeconds, m.paths.AIPromptFile)
			opts = append(opts, huh.NewOption(label, name))
		}
		sel := huh.NewSelect[string]().
			Options(opts...).
			Description(m.text.SettingsAIProviderDescription).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		sel.Focus()
		return sel, current

	case settingsFieldAICommand:
		in := huh.NewInput().
			Description(m.text.SettingsAICommandDescription).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		in.Focus()
		return in, current

	case settingsFieldAITimeout:
		in := huh.NewInput().
			Value(current).
			Validate(func(v string) error {
				if _, err := strconv.Atoi(v); err != nil {
					return fmt.Errorf("%s", m.text.SettingsInvalidNumber)
				}
				return nil
			}).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		in.Focus()
		return in, current

	default: // settingsFieldShowStartupAnimation, settingsFieldShowHealthcheck, settingsFieldAutoCheckToolUpdates
		var description string
		switch field {
		case settingsFieldShowHealthcheck:
			description = m.text.SettingsShowHealthcheckDescription
		case settingsFieldAutoCheckToolUpdates:
			description = m.text.SettingsAutoCheckToolUpdatesDescription
		default:
			description = m.text.SettingsShowStartupAnimationDescription
		}
		sel := huh.NewSelect[string]().
			Options(
				huh.NewOption(boolDisplayWord("true", m.text), "true"),
				huh.NewOption(boolDisplayWord("false", m.text), "false"),
			).
			Description(description).
			Value(current).
			WithKeyMap(huh.NewDefaultKeyMap()).
			WithTheme(m.huhTheme).
			WithWidth(width)
		sel.Focus()
		return sel, current
	}
}

// newSettingsConfirmDialog builds the "really change X to Y?" confirm shown
// before a single field's edit takes effect.
func newSettingsConfirmDialog(t uiText, theme *huh.Theme, width int, label, newValue string) (huh.Field, *bool) {
	answer := new(bool)
	dialog := huh.NewConfirm().
		Title(fmt.Sprintf(t.SettingsConfirmTitleFmt, label, newValue)).
		Affirmative(t.SettingsConfirmYes).
		Negative(t.SettingsConfirmNo).
		Value(answer).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()
	return dialog, answer
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.settingsScreen.stage {
	case settingsEditing:
		return m.updateSettingsEditing(msg)
	case settingsConfirming:
		return m.updateSettingsConfirming(msg)
	default:
		return m.updateSettingsBrowsing(msg)
	}
}

func (m Model) updateSettingsBrowsing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if checkMsg, ok := msg.(settingsUpdateCheckMsg); ok {
		return m.applySettingsUpdateCheck(checkMsg), nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		if m.settingsReturnTo == screenDeviceSelect {
			return m.enterDeviceSelect()
		}
		return m.enterDashboard(m.dashboard.serial)
	case "up", "k":
		if m.settingsScreen.cursor > 0 {
			m.settingsScreen.cursor--
		}
	case "down", "j":
		if m.settingsScreen.cursor < len(settingsFieldsOrder)-1 {
			m.settingsScreen.cursor++
		}
	case "enter":
		field := settingsFieldsOrder[m.settingsScreen.cursor]
		if field == settingsFieldCheckForUpdatesNow {
			if m.settingsScreen.checkNowStatus == m.text.SettingsCheckingForUpdates {
				return m, nil // already in flight; ignore a repeat press
			}
			m.settingsScreen.checkNowStatus = m.text.SettingsCheckingForUpdates
			return m, runSettingsUpdateCheckCmd(m.ctx, m.paths)
		}
		m.settingsScreen.status = ""
		m.settingsScreen.stage = settingsEditing
		m.settingsScreen.editingField = field
		editField, editValue := newSettingsEditField(m, field)
		m.settingsScreen.editField = editField
		m.settingsScreen.editValue = editValue
	}
	return m, nil
}

// settingsUpdateCheckMsg carries the result of the Settings screen's manual
// "check for updates now" action - unlike the periodic background checks
// (selfUpdateCheckMsg/toolsUpdateCheckMsg), this always performs a live
// query regardless of the cache, since forcing a fresh check is the whole
// point of the button.
type settingsUpdateCheckMsg struct {
	toolboxVersion string // "" if the query failed
	adbLatest      string
	scrcpyLatest   string
}

// runSettingsUpdateCheckCmd queries both android-toolbox's own latest
// release and the third-party tools' in one go, reusing
// liveCheckToolVersions (toolsupdate_check.go) for the latter so the
// "what counts as outdated" logic never drifts between the manual and
// automatic checks.
func runSettingsUpdateCheckCmd(ctx context.Context, paths config.Paths) tea.Cmd {
	return func() tea.Msg {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var toolboxVersion string
		if rel, err := selfupdate.LatestRelease(checkCtx); err == nil {
			toolboxVersion = rel.Version
		}
		adbLatest, scrcpyLatest := liveCheckToolVersions(checkCtx, paths)

		return settingsUpdateCheckMsg{toolboxVersion: toolboxVersion, adbLatest: adbLatest, scrcpyLatest: scrcpyLatest}
	}
}

// applySettingsUpdateCheck folds a manual check's result into Model exactly
// like the periodic background checks do (same state.json cache fields, so
// a manual check also postpones the next automatic one) and leaves a short
// status message on the "check for updates now" row.
func (m Model) applySettingsUpdateCheck(msg settingsUpdateCheckMsg) Model {
	now := time.Now().UTC().Format(time.RFC3339)
	m.state.LastUpdateCheckAt = now
	m.state.LastToolsUpdateCheckAt = now
	if msg.toolboxVersion != "" {
		m.latestKnownVersion = msg.toolboxVersion
		m.state.LatestKnownVersion = msg.toolboxVersion
	}
	if msg.adbLatest != "" {
		m.latestKnownADB = msg.adbLatest
		m.state.ADBLatestKnown = msg.adbLatest
	}
	if msg.scrcpyLatest != "" {
		m.latestKnownScrcpy = msg.scrcpyLatest
		m.state.ScrcpyLatestKnown = msg.scrcpyLatest
	}
	_ = config.SaveState(m.paths, m.state) // best-effort, same as the background checks

	m.settingsScreen.checkNowStatus = m.text.SettingsCheckCompleteStatus
	return m
}

func (m Model) updateSettingsEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.settingsScreen.stage = settingsBrowsing
			return m, nil
		case "enter":
			return m.submitSettingsEdit()
		}
	}

	updated, cmd := m.settingsScreen.editField.Update(msg)
	if f, ok := updated.(huh.Field); ok {
		m.settingsScreen.editField = f
	}
	return m, cmd
}

// submitSettingsEdit closes out the editing stage: a value equal to what's
// already saved is a no-op back to browsing (nothing to confirm), otherwise
// it opens the confirm dialog for the change. An invalid AI timeout is
// rejected here directly (rather than relying solely on the field's own
// Validate) so the flow never proceeds to "confirm" a value that would fail
// to parse on commit.
func (m Model) submitSettingsEdit() (tea.Model, tea.Cmd) {
	s := m.settingsScreen
	newValue := strings.TrimSpace(*s.editValue)

	if s.editingField == settingsFieldAITimeout {
		if _, err := strconv.Atoi(newValue); err != nil {
			return m, nil // stay in editing; the field already shows the error
		}
	}

	if newValue == s.rawValue(s.editingField) {
		m.settingsScreen.stage = settingsBrowsing
		return m, nil
	}

	displayNew := newValue
	switch s.editingField {
	case settingsFieldLanguage:
		displayNew = languageDisplayName(newValue)
	case settingsFieldShowStartupAnimation, settingsFieldShowHealthcheck, settingsFieldAutoCheckToolUpdates:
		displayNew = boolDisplayWord(newValue, m.text)
	}

	dialog, answer := newSettingsConfirmDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), m.settingsFieldLabel(s.editingField), displayNew)
	m.settingsScreen.stage = settingsConfirming
	m.settingsScreen.confirmField = s.editingField
	m.settingsScreen.confirmNewValue = newValue
	m.settingsScreen.confirmDialog = dialog
	m.settingsScreen.confirmAnswer = answer
	return m, nil
}

func (m Model) updateSettingsConfirming(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.settingsScreen.stage = settingsBrowsing
		return m, nil
	}

	updated, cmd := m.settingsScreen.confirmDialog.Update(msg)
	if f, ok := updated.(huh.Field); ok {
		m.settingsScreen.confirmDialog = f
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "tab", "y", "Y", "n", "N":
			accepted := *m.settingsScreen.confirmAnswer
			field := m.settingsScreen.confirmField
			value := m.settingsScreen.confirmNewValue
			label := m.settingsFieldLabel(field)

			m.settingsScreen.stage = settingsBrowsing
			if !accepted {
				return m, nil
			}

			if err := m.commitSettingsField(field, value); err != nil {
				m.settingsScreen.statusErr = true
				m.settingsScreen.status = fmt.Sprintf(m.text.SettingsSaveErrorFmt, err.Error())
				return m, nil
			}
			m.settingsScreen.statusErr = false
			m.settingsScreen.status = fmt.Sprintf(m.text.SettingsChangeSavedFmt, label)
			return m, nil
		}
	}
	return m, cmd
}

// commitSettingsField persists exactly one field's new value: updates
// m.settings, re-resolves m.text/m.keys when the language changed, rebuilds
// the AI provider when any AI-related field changed, and writes
// settings.yaml. settingsScreen's own cached display fields are updated too
// so the browsing list immediately reflects the change.
func (m *Model) commitSettingsField(field settingsFieldKind, value string) error {
	switch field {
	case settingsFieldLanguage:
		m.settings.UI.Language = value
		m.text = resolveUIText(m.settings.Language())
		m.keys = newKeyMap(m.text)
		m.settingsScreen.language = value

	case settingsFieldAIProvider:
		m.settings.AI.Provider = value
		m.settingsScreen.aiProvider = value

	case settingsFieldAICommand:
		m.settings.AI.Claude.Command = value
		m.settingsScreen.aiCommand = value

	case settingsFieldAITimeout:
		timeout, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		m.settings.AI.Claude.TimeoutSeconds = timeout
		m.settingsScreen.aiTimeout = value

	case settingsFieldShowStartupAnimation:
		m.settings.UI.ShowStartupAnimation = value == "true"
		m.settingsScreen.showStartupAnimation = value

	case settingsFieldShowHealthcheck:
		m.settings.UI.ShowHealthcheck = value == "true"
		m.settingsScreen.showHealthcheck = value

	case settingsFieldAutoCheckToolUpdates:
		m.settings.UI.AutoCheckToolUpdates = value == "true"
		m.settingsScreen.autoCheckToolUpdates = value
	}

	if field == settingsFieldAIProvider || field == settingsFieldAICommand || field == settingsFieldAITimeout {
		provider, aiErr := ai.New(m.settings.AI.Provider, m.settings.AI.Claude.Command, m.settings.AI.Claude.TimeoutSeconds, m.paths.AIPromptFile)
		m.aiProvider = provider
		m.aiErr = aiErr
	}

	return config.SaveSettings(m.paths, m.settings)
}

func (m Model) viewSettings() string {
	switch m.settingsScreen.stage {
	case settingsEditing:
		return m.viewSettingsEditing()
	case settingsConfirming:
		return m.viewSettingsConfirming()
	default:
		return m.viewSettingsBrowsing()
	}
}

func (m Model) viewSettingsBrowsing() string {
	title := m.styles.Title.Render(m.text.SettingsTitle)
	s := m.settingsScreen

	var rows strings.Builder
	for i, f := range settingsFieldsOrder {
		marker := "  "
		if i == s.cursor {
			marker = m.styles.Highlight.Render("> ")
		}
		fmt.Fprintf(&rows, "%s%s: %s\n", marker, m.settingsFieldLabel(f), s.displayValue(f, m.text))
	}

	var status string
	if s.status != "" {
		style := m.styles.OK
		if s.statusErr {
			style = m.styles.Error
		}
		status = "\n" + style.Render(s.status) + "\n"
	}

	toolInfo := m.styles.Highlight.Render(m.text.SettingsToolInfoTitle) + "\n" +
		m.styles.Subtle.Render(settingsToolInfo(m)) + "\n\n" +
		m.styles.Subtle.Render(appInfoText(m.text))

	updateInfo := m.styles.Highlight.Render(m.text.SettingsUpdatesTitle) + "\n" +
		m.styles.Subtle.Render(settingsUpdateInfo(m))

	return title + "\n\n" + rows.String() + status + "\n" + toolInfo + "\n\n" + updateInfo + "\n\n" + m.styles.Subtle.Render(m.text.SettingsBrowsingFooter)
}

func (m Model) viewSettingsEditing() string {
	title := m.styles.Title.Render(m.text.SettingsTitle)
	label := m.settingsFieldLabel(m.settingsScreen.editingField)
	return title + "\n\n" + m.styles.Highlight.Render(label) + "\n\n" +
		m.settingsScreen.editField.View() + "\n" + m.styles.Subtle.Render(m.text.SettingsEditingFooter)
}

func (m Model) viewSettingsConfirming() string {
	title := m.styles.Title.Render(m.text.SettingsTitle)
	return title + "\n\n" + m.settingsScreen.confirmDialog.View()
}
