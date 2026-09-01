package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"android-toolbox/internal/adb"
	"android-toolbox/internal/avd"
)

// emulatorBootTimeout/emulatorBootPollInterval bound the boot-wait polling
// toggleSelectedEmulator kicks off after a successful Launch: real-world
// cold boots (especially without hardware acceleration configured) can take
// a long time, but "forever with no feedback" is indistinguishable from
// "actually stuck" - a bounded wait with a periodic status update, plus
// early-crash detection via Launch's onExit hook, replaces that with a
// concrete outcome either way.
const (
	emulatorBootTimeout      = 90 * time.Second
	emulatorBootPollInterval = 2 * time.Second
)

// emulatorAction identifies which simulation/specs field wizard is currently
// active on the emulator list screen - emuActionNone means plain browsing.
type emulatorAction int

const (
	emuActionNone emulatorAction = iota
	emuActionGPS
	emuActionNetwork
	emuActionBattery
	emuActionSpecs
)

// emulatorItem is one AVD in the list, annotated with whether it's currently
// running and (if so) which adb serial backs it - cross-referenced against
// adb.ListDevices/EmuAVDName in refreshEmulatorsCmd, since avdmanager itself
// has no notion of "running".
type emulatorItem struct {
	a       avd.AVD
	running bool
	serial  string
}

func (i emulatorItem) Title() string {
	// A plain unicode marker rather than an inline style: the list
	// delegate re-colors the whole title string based on selection state,
	// so embedding our own ANSI codes here would just get overridden -
	// still a clear at-a-glance running/stopped/broken signal either way.
	switch {
	case i.a.Broken:
		return "⚠ " + i.a.Name
	case i.running:
		return "● " + i.a.Name + " (" + i.serial + ")"
	default:
		return "○ " + i.a.Name
	}
}
func (i emulatorItem) Description() string {
	if i.a.Broken {
		return i.a.Error
	}
	if i.a.ABI != "" {
		return i.a.Target + " - " + i.a.ABI
	}
	return i.a.Target
}
func (i emulatorItem) FilterValue() string { return i.a.Name }

type emulatorListScreen struct {
	list list.Model
	err  error

	status    string
	statusErr bool

	// Destructive delete confirm - same standalone-huh.Confirm shape as
	// recoverScreen's restore confirm.
	confirmDeleteName string
	confirmDialog     huh.Field
	confirmAnswer     *bool

	// Simulation/specs field wizard (GPS, network, battery, specs) - see
	// fieldwizard.go.
	action     emulatorAction
	actionName string
	wizard     fieldWizard
}

// startingEmulator tracks one AVD currently being waited on to finish
// booting (see toggleSelectedEmulator). Kept on Model (see
// Model.startingEmulators) rather than emulatorListScreen: that struct is
// fully rebuilt every time the Emulators screen is (re-)entered (see
// newEmulatorListScreen), which would otherwise silently forget an
// in-flight boot the moment the user switches tools or opens the create
// wizard and comes back - and then let them launch a second, colliding
// instance of the same AVD (the emulator refuses this outright: "Running
// multiple emulators with the same AVD is an experimental feature").
type startingEmulator struct {
	deadline time.Time
	logPath  string
}

func newEmulatorListScreen(m Model) (emulatorListScreen, tea.Cmd) {
	l := list.New(nil, newWrappingDelegate(2), 0, 0)
	l.Styles = androidListStyles()
	// The page title just above already names this screen - the list's own
	// title bar would just restate that a second time (same reasoning as
	// deviceSelect/recover).
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	s := emulatorListScreen{list: l}

	if m.avdManager == nil {
		return s, nil
	}
	return s, refreshEmulatorsCmd(m.ctx, m.avdManager, m.adbClient)
}

// emulatorsRefreshedMsg carries the result of re-listing AVDs plus which of
// them are currently running.
type emulatorsRefreshedMsg struct {
	avds    []avd.AVD
	running map[string]string // AVD name -> adb serial
	err     error
}

func refreshEmulatorsCmd(ctx context.Context, manager *avd.Manager, client *adb.Client) tea.Cmd {
	return func() tea.Msg {
		avds, err := manager.List(ctx)
		if err != nil {
			return emulatorsRefreshedMsg{err: err}
		}
		running := map[string]string{}
		if client != nil {
			if devices, err := client.ListDevices(ctx); err == nil {
				for _, d := range devices {
					if !adb.IsEmulatorSerial(d.Serial) || !d.Connected() {
						continue
					}
					if name, err := client.EmuAVDName(ctx, d.Serial); err == nil && name != "" {
						running[name] = d.Serial
					}
				}
			}
		}
		return emulatorsRefreshedMsg{avds: avds, running: running}
	}
}

func emulatorItemsFrom(avds []avd.AVD, running map[string]string) []list.Item {
	items := make([]list.Item, len(avds))
	for i, a := range avds {
		serial, ok := running[a.Name]
		items[i] = emulatorItem{a: a, running: ok, serial: serial}
	}
	return items
}

// emulatorActionAppliedMsg reports the outcome of a blocking action (adb
// emu ..., start, stop, delete) run via a tea.Cmd rather than inline, so
// the TUI's Update loop never blocks on a subprocess/network call.
type emulatorActionAppliedMsg struct {
	status  string
	err     error
	refresh bool // re-fetch the AVD/running list afterward
}

func runEmulatorActionCmd(status string, refresh bool, fn func() error) tea.Cmd {
	return func() tea.Msg {
		err := fn()
		return emulatorActionAppliedMsg{status: status, err: err, refresh: refresh}
	}
}

// emulatorBootPollMsg drives the periodic deadline check while any AVD is
// starting (see Model.startingEmulators) - handled globally in app.go's
// Update regardless of the current screen.
type emulatorBootPollMsg struct{}

func emulatorBootPollCmd() tea.Cmd {
	return tea.Tick(emulatorBootPollInterval, func(time.Time) tea.Msg { return emulatorBootPollMsg{} })
}

// emulatorBootCheckMsg carries which emulator-* serials are currently
// connected, keyed by their AVD name - a lighter-weight relative of
// emulatorsRefreshedMsg used purely to detect "has this AVD finished
// booting yet", without also needing an *avd.Manager (an AVD list) just to
// poll adb.
type emulatorBootCheckMsg struct {
	running map[string]string // AVD name -> adb serial
}

func checkEmulatorBootCmd(ctx context.Context, client *adb.Client) tea.Cmd {
	return func() tea.Msg {
		running := map[string]string{}
		if client != nil {
			if devices, err := client.ListDevices(ctx); err == nil {
				for _, d := range devices {
					if !adb.IsEmulatorSerial(d.Serial) || !d.Connected() {
						continue
					}
					if name, err := client.EmuAVDName(ctx, d.Serial); err == nil && name != "" {
						running[name] = d.Serial
					}
				}
			}
		}
		return emulatorBootCheckMsg{running: running}
	}
}

// emulatorExitedMsg reports that a launched emulator process has exited -
// delivered via avd.Launcher.Launch's onExit callback, forwarded into
// Bubbletea's event loop through a channel exactly like screen_runner.go's
// streamEvent/screen_progress.go's progressEvent do for their own
// background work.
type emulatorExitedMsg struct {
	name string
	err  error
}

func waitForEmulatorExit(ch chan emulatorExitedMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func newDeleteAVDConfirmDialog(t uiText, theme *huh.Theme, width int, name string) (huh.Field, *bool) {
	answer := new(bool)
	dialog := huh.NewConfirm().
		Title(fmt.Sprintf(t.EmulatorDeleteTitleFmt, name)).
		Affirmative(t.EmulatorDeleteYes).
		Negative(t.EmulatorDeleteNo).
		Value(answer).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(theme).
		WithWidth(width)
	dialog.Focus()
	return dialog, answer
}

func requiredValidator(msg string) func(string) error {
	return func(v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}

func numberValidator(msg string) func(string) error {
	return func(v string) error {
		if _, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}

// storageMBFromConfig reads config.ini's disk.dataPartition.size - a raw
// byte count on AVDs created by older avdmanager versions, or a "<N>M"/
// "<N>G"-suffixed value on newer ones - and normalizes it to a plain MB
// integer string for display in the specs editor.
func storageMBFromConfig(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return strconv.FormatInt(n/(1024*1024), 10) // bytes -> MB
	}
	n, err := strconv.ParseInt(raw[:len(raw)-1], 10, 64)
	if err != nil {
		return ""
	}
	switch raw[len(raw)-1] {
	case 'G', 'g':
		return strconv.FormatInt(n*1024, 10)
	case 'M', 'm':
		return strconv.FormatInt(n, 10)
	case 'K', 'k':
		return strconv.FormatInt(n/1024, 10)
	default:
		return ""
	}
}

// startEmulatorWizard configures m.emulatorList to begin one of the
// simulation/specs field wizards for the AVD named name.
func (m Model) startEmulatorWizard(action emulatorAction, name string) Model {
	width := m.fullScreenDialogWidth()
	t := m.text
	var steps []fieldWizardStep

	switch action {
	case emuActionGPS:
		steps = []fieldWizardStep{
			newInputStep(t.FieldLatitude, "0", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newInputStep(t.FieldLongitude, "0", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
		}
	case emuActionNetwork:
		steps = []fieldWizardStep{
			newSelectStep(t.FieldNetworkSpeed, stringOptions(avd.NetworkSpeedProfiles), false, m.huhTheme, width),
			newSelectStep(t.FieldNetworkDelay, stringOptions(avd.NetworkDelayProfiles), false, m.huhTheme, width),
		}
	case emuActionBattery:
		steps = []fieldWizardStep{
			newInputStep(t.FieldBatteryPercent, "100", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newSelectStep(t.FieldBatteryCharging, []huh.Option[string]{
				huh.NewOption(t.ChargingYes, "yes"),
				huh.NewOption(t.ChargingNo, "no"),
			}, false, m.huhTheme, width),
		}
	case emuActionSpecs:
		cfg, _ := avd.ReadConfig(avd.AvdHome(), name)
		steps = []fieldWizardStep{
			newInputStep(t.FieldRAM, cfg["hw.ramSize"], numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newInputStep(t.FieldHeap, cfg["vm.heapSize"], numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newInputStep(t.FieldCPUCores, cfg["hw.cpu.ncore"], numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newInputStep(t.FieldStorage, storageMBFromConfig(cfg["disk.dataPartition.size"]), numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
			newInputStep(t.FieldDensity, cfg["hw.lcd.density"], numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
		}
	}

	m.emulatorList.action = action
	m.emulatorList.actionName = name
	m.emulatorList.wizard = newFieldWizard(steps)
	return m
}

// applyEmulatorWizard runs the completed wizard's effect and returns to
// plain browsing.
func (m Model) applyEmulatorWizard() (tea.Model, tea.Cmd) {
	action := m.emulatorList.action
	name := m.emulatorList.actionName
	serial := m.emulatorList.list.SelectedItem().(emulatorItem).serial
	steps := m.emulatorList.wizard.Steps
	m.emulatorList.action = emuActionNone

	switch action {
	case emuActionGPS:
		lat, _ := strconv.ParseFloat(strings.TrimSpace(*steps[0].Value), 64)
		lon, _ := strconv.ParseFloat(strings.TrimSpace(*steps[1].Value), 64)
		return m, runEmulatorActionCmd(m.text.EmulatorGPSAppliedMsg, false, func() error {
			return avd.SetGPS(m.ctx, m.adbClient, serial, lat, lon)
		})
	case emuActionNetwork:
		speed, delay := *steps[0].Value, *steps[1].Value
		return m, runEmulatorActionCmd(m.text.EmulatorNetworkAppliedMsg, false, func() error {
			if err := avd.SetNetworkSpeed(m.ctx, m.adbClient, serial, speed); err != nil {
				return err
			}
			return avd.SetNetworkDelay(m.ctx, m.adbClient, serial, delay)
		})
	case emuActionBattery:
		percent, _ := strconv.Atoi(strings.TrimSpace(*steps[0].Value))
		charging := *steps[1].Value == "yes"
		return m, runEmulatorActionCmd(m.text.EmulatorBatteryAppliedMsg, false, func() error {
			return avd.SetBattery(m.ctx, m.adbClient, serial, percent, charging)
		})
	case emuActionSpecs:
		ram := strings.TrimSpace(*steps[0].Value)
		heap := strings.TrimSpace(*steps[1].Value)
		cpu := strings.TrimSpace(*steps[2].Value)
		storageMB := strings.TrimSpace(*steps[3].Value)
		density := strings.TrimSpace(*steps[4].Value)
		return m, runEmulatorActionCmd(m.text.EmulatorSpecsAppliedMsg, true, func() error {
			return avd.WriteConfig(avd.AvdHome(), name, map[string]string{
				"hw.ramSize":              ram,
				"vm.heapSize":             heap,
				"hw.cpu.ncore":            cpu,
				"disk.dataPartition.size": storageMB + "M",
				"hw.lcd.density":          density,
			})
		})
	}
	return m, nil
}

// updateEmulatorBootPoll/updateEmulatorBootCheck/updateEmulatorExited handle
// the three boot-wait message types - see app.go's Update, which routes
// them here regardless of m.current (not just while screenEmulatorList is
// active), and Model.startingEmulators' own doc comment for why that
// matters: without it, navigating away mid-boot would silently stop
// resolving (and, worse, forget) the in-flight start.

func (m Model) updateEmulatorBootPoll(msg emulatorBootPollMsg) (Model, tea.Cmd) {
	if len(m.startingEmulators) == 0 {
		return m, nil
	}
	now := time.Now()
	for name, s := range m.startingEmulators {
		if now.After(s.deadline) {
			delete(m.startingEmulators, name)
			if m.current == screenEmulatorList {
				m.emulatorList.statusErr = true
				m.emulatorList.status = fmt.Sprintf(m.text.EmulatorBootTimeoutFmt, name, s.logPath)
			}
		}
	}
	if len(m.startingEmulators) == 0 {
		return m, nil
	}
	return m, tea.Batch(emulatorBootPollCmd(), checkEmulatorBootCmd(m.ctx, m.adbClient))
}

func (m Model) updateEmulatorBootCheck(msg emulatorBootCheckMsg) (Model, tea.Cmd) {
	booted := false
	for name := range m.startingEmulators {
		if _, ok := msg.running[name]; ok {
			delete(m.startingEmulators, name)
			booted = true
			if m.current == screenEmulatorList {
				m.emulatorList.statusErr = false
				m.emulatorList.status = fmt.Sprintf(m.text.EmulatorBootedFmt, name)
			}
		}
	}
	// A boot detected while the user was elsewhere won't have updated the
	// list's own running-markers - refresh it now so returning to the
	// screen shows the new state immediately instead of waiting for its
	// next unrelated refresh.
	if booted && m.current == screenEmulatorList && m.avdManager != nil {
		return m, refreshEmulatorsCmd(m.ctx, m.avdManager, m.adbClient)
	}
	return m, nil
}

func (m Model) updateEmulatorExited(msg emulatorExitedMsg) (Model, tea.Cmd) {
	s, wasStarting := m.startingEmulators[msg.name]
	if wasStarting {
		delete(m.startingEmulators, msg.name)
		if m.current == screenEmulatorList {
			m.emulatorList.statusErr = true
			if msg.err != nil {
				m.emulatorList.status = fmt.Sprintf(m.text.EmulatorCrashedFmt, msg.name, msg.err.Error(), s.logPath)
			} else {
				m.emulatorList.status = fmt.Sprintf(m.text.EmulatorExitedEarlyFmt, msg.name, s.logPath)
			}
		}
	}
	if m.current == screenEmulatorList && m.avdManager != nil {
		return m, refreshEmulatorsCmd(m.ctx, m.avdManager, m.adbClient)
	}
	return m, nil
}

func (m Model) updateEmulatorList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case emulatorsRefreshedMsg:
		m.emulatorList.err = msg.err
		if msg.err == nil {
			m.emulatorList.list.SetItems(emulatorItemsFrom(msg.avds, msg.running))
		}
		return m, nil

	case emulatorActionAppliedMsg:
		m.emulatorList.statusErr = msg.err != nil
		if msg.err != nil {
			m.emulatorList.status = msg.err.Error()
		} else {
			m.emulatorList.status = msg.status
		}
		if msg.refresh && m.avdManager != nil {
			return m, refreshEmulatorsCmd(m.ctx, m.avdManager, m.adbClient)
		}
		return m, nil
	}

	if m.emulatorList.confirmDeleteName != "" {
		return m.updateEmulatorDeleteConfirm(msg)
	}
	if m.emulatorList.action != emuActionNone {
		return m.updateEmulatorWizard(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok && m.emulatorList.list.FilterState() != list.Filtering {
		switch key.String() {
		case "esc":
			return m.enterToolSelect()
		case "n":
			return m.enterEmulatorCreate()
		case "enter":
			return m.toggleSelectedEmulator()
		case "d":
			if item, ok := m.emulatorList.list.SelectedItem().(emulatorItem); ok {
				m.emulatorList.confirmDeleteName = item.a.Name
				dialog, answer := newDeleteAVDConfirmDialog(m.text, m.huhTheme, m.fullScreenDialogWidth(), item.a.Name)
				m.emulatorList.confirmDialog = dialog
				m.emulatorList.confirmAnswer = answer
			}
			return m, nil
		case "g", "w", "b":
			item, ok := m.emulatorList.list.SelectedItem().(emulatorItem)
			if !ok {
				return m, nil
			}
			if !item.running {
				m.emulatorList.statusErr = true
				m.emulatorList.status = m.text.EmulatorNotRunningMsg
				return m, nil
			}
			action := map[string]emulatorAction{"g": emuActionGPS, "w": emuActionNetwork, "b": emuActionBattery}[key.String()]
			m = m.startEmulatorWizard(action, item.a.Name)
			return m, nil
		case "e":
			if item, ok := m.emulatorList.list.SelectedItem().(emulatorItem); ok {
				m = m.startEmulatorWizard(emuActionSpecs, item.a.Name)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.emulatorList.list, cmd = m.emulatorList.list.Update(msg)
	return m, cmd
}

func (m Model) toggleSelectedEmulator() (Model, tea.Cmd) {
	item, ok := m.emulatorList.list.SelectedItem().(emulatorItem)
	if !ok {
		return m, nil
	}
	if item.running {
		return m, runEmulatorActionCmd(fmt.Sprintf(m.text.EmulatorStoppedFmt, item.a.Name), true, func() error {
			_, err := m.adbClient.Emu(m.ctx, item.serial, "kill")
			return err
		})
	}
	if item.a.Broken {
		// avdmanager itself never gets far enough to start these (see
		// AVD.Broken) - launching would just fail with a much less clear
		// error from the emulator binary than the one avdmanager already
		// gave us.
		m.emulatorList.statusErr = true
		m.emulatorList.status = fmt.Sprintf(m.text.EmulatorBrokenMsg, item.a.Name, item.a.Error)
		return m, nil
	}
	if _, already := m.startingEmulators[item.a.Name]; already {
		// Already launched and being waited on - re-running Launch here
		// would spawn a second emulator process for the same AVD, which the
		// emulator refuses outright ("Running multiple emulators with the
		// same AVD is an experimental feature"), not "start it faster".
		m.emulatorList.status = fmt.Sprintf(m.text.EmulatorAlreadyStartingFmt, item.a.Name)
		m.emulatorList.statusErr = false
		return m, nil
	}
	if m.avdLauncher == nil {
		m.emulatorList.statusErr = true
		m.emulatorList.status = m.text.EmulatorStartFailedNoToolMsg
		return m, nil
	}

	windowed := m.settings.Emulator.Windowed
	extraArgs := m.settings.Emulator.ExtraArgs
	name := item.a.Name
	exitCh := make(chan emulatorExitedMsg, 1)
	result, err := m.avdLauncher.Launch(name, windowed, extraArgs, func(waitErr error) {
		exitCh <- emulatorExitedMsg{name: name, err: waitErr}
	})
	if err != nil {
		m.emulatorList.statusErr = true
		m.emulatorList.status = err.Error()
		return m, nil
	}

	if m.startingEmulators == nil {
		m.startingEmulators = map[string]startingEmulator{}
	}
	m.startingEmulators[name] = startingEmulator{
		deadline: time.Now().Add(emulatorBootTimeout),
		logPath:  result.LogPath,
	}
	m.emulatorList.statusErr = false
	m.emulatorList.status = fmt.Sprintf(m.text.EmulatorBootWaitingFmt, name)
	return m, tea.Batch(emulatorBootPollCmd(), waitForEmulatorExit(exitCh))
}

func (m Model) updateEmulatorDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() == "esc" {
		m.emulatorList.confirmDeleteName = ""
		return m, nil
	}

	updated, cmd := m.emulatorList.confirmDialog.Update(key)
	if field, ok := updated.(huh.Field); ok {
		m.emulatorList.confirmDialog = field
	}

	switch key.String() {
	case "enter", "tab", "y", "Y", "n", "N":
		name := m.emulatorList.confirmDeleteName
		accepted := *m.emulatorList.confirmAnswer
		m.emulatorList.confirmDeleteName = ""
		if !accepted {
			return m, nil
		}
		manager := m.avdManager
		return m, runEmulatorActionCmd(fmt.Sprintf(m.text.EmulatorDeletedFmt, name), true, func() error {
			return manager.Delete(m.ctx, name)
		})
	}
	return m, cmd
}

func (m Model) updateEmulatorWizard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" {
			if sel, filtering := selectStepFiltering(m.emulatorList.wizard.Current()); filtering {
				sel.Filtering(false)
				return m, nil
			}
			m.emulatorList.action = emuActionNone
			return m, nil
		}
		if key.String() == "enter" {
			if err := m.emulatorList.wizard.Current().Field.Error(); err == nil {
				m.emulatorList.wizard.Advance()
				if m.emulatorList.wizard.Done() {
					return m.applyEmulatorWizard()
				}
			}
			return m, nil
		}
	}

	wizard, cmd := m.emulatorList.wizard.Update(msg)
	m.emulatorList.wizard = wizard
	return m, cmd
}

// emulatorRightPaneStyle mirrors rightPaneStyle's reasoning (dashboard
// cluster) for the emulator screen's own sub-states, so what's currently
// happening here is visible from the pane border color alone too: neutral
// while browsing, a warning border while confirming a delete, the same
// accent as entering action parameters while stepping through a
// simulation/specs field, and the "streaming" info color while waiting for
// a just-launched AVD to boot.
func (m Model) emulatorRightPaneStyle() lipgloss.Style {
	switch {
	case m.emulatorList.confirmDeleteName != "":
		return m.styles.PaneConfirm
	case m.emulatorList.action != emuActionNone:
		return m.styles.PaneParam
	default:
		if item, ok := m.emulatorList.list.SelectedItem().(emulatorItem); ok {
			if _, starting := m.startingEmulators[item.a.Name]; starting {
				return m.styles.PaneRunning
			}
		}
		return m.styles.PanePreview
	}
}

// viewEmulatorList composes the same two-pane look the device dashboard
// uses (AVD list left, detail/wizard right, border color reflecting state)
// via the shared renderTwoPane/paneWidths/bodyHeight helpers, rather than a
// single scrolling column of plain text.
func (m Model) viewEmulatorList() string {
	title := m.styles.Title.Render(m.text.EmulatorsTitle)

	leftW, rightW := paneWidths(m.width)
	outerH := bodyHeight(m.height)

	left := m.emulatorList.list.View()
	if len(m.emulatorList.list.Items()) == 0 {
		left = m.styles.Subtle.Render(m.text.EmulatorsNone)
	}

	var right string
	switch {
	case m.emulatorList.confirmDeleteName != "":
		right = m.emulatorList.confirmDialog.View()
	case m.emulatorList.action != emuActionNone:
		step := m.emulatorList.wizard.Current()
		right = m.styles.Highlight.Render(step.Label) + "\n\n" + step.Field.View()
	default:
		right = m.viewEmulatorRightPane()
	}

	panes := renderTwoPane(m.styles.PaneLeft, m.emulatorRightPaneStyle(), left, right, leftW, rightW, outerH)
	return title + "\n\n" + panes + "\n\n" + m.styles.Subtle.Render(m.text.EmulatorsFooter)
}

// viewEmulatorRightPane is the right pane's content while plain browsing:
// the highlighted AVD's detail panel, plus any error/status message.
func (m Model) viewEmulatorRightPane() string {
	var b strings.Builder
	if item, ok := m.emulatorList.list.SelectedItem().(emulatorItem); ok {
		b.WriteString(m.emulatorDetail(item))
	} else {
		b.WriteString(m.styles.Subtle.Render(m.text.EmulatorsNone))
	}
	if m.emulatorList.err != nil {
		b.WriteString("\n\n" + m.styles.Error.Render(m.emulatorList.err.Error()))
	}
	if m.emulatorList.status != "" {
		style := m.styles.OK
		if m.emulatorList.statusErr {
			style = m.styles.Error
		}
		b.WriteString("\n\n" + style.Render(m.emulatorList.status))
	}
	return b.String()
}

// emulatorDetail renders the richer info panel the user asked for: specs
// (parsed from config.ini) plus running status - more than a physical
// device's info ever shows, since config.ini simply doesn't exist for one.
func (m Model) emulatorDetail(item emulatorItem) string {
	var b strings.Builder
	const fieldColumnWidth = 16
	field := func(label string) string { return fmt.Sprintf("%-*s", fieldColumnWidth, label) }

	statusBadge := m.styles.Badge.Render(m.text.FieldStatusStopped)
	switch {
	case item.a.Broken:
		statusBadge = m.styles.BadgeWarn.Render(m.text.FieldStatusBroken)
	case item.running:
		statusBadge = m.styles.BadgeGood.Render(fmt.Sprintf("%s (%s)", m.text.FieldStatusRunning, item.serial))
	}
	fmt.Fprintf(&b, "%s  %s\n\n", m.styles.Highlight.Render(item.a.Name), statusBadge)

	if item.a.Broken {
		// avdmanager never got far enough to report Target/ABI/Device for
		// this one (see AVD.Broken) - its Error is the only useful thing
		// left to show, plus Path so it can be found/cleaned up by hand.
		fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldPath), item.a.Path)
		b.WriteString("\n" + m.styles.Warn.Render(item.a.Error) + "\n")
		return b.String()
	}

	fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldTarget), item.a.Target)
	if item.a.Device != "" {
		fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldDevice), item.a.Device)
	}
	fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldPath), item.a.Path)

	if cfg, err := avd.ReadConfig(avd.AvdHome(), item.a.Name); err == nil {
		b.WriteString("\n" + m.styles.Subtle.Render("── "+m.text.EmulatorSpecsTitle+" ──") + "\n")
		if v := cfg["hw.ramSize"]; v != "" {
			fmt.Fprintf(&b, "%s%s MB\n", field(m.text.FieldRAM), v)
		}
		if v := cfg["vm.heapSize"]; v != "" {
			fmt.Fprintf(&b, "%s%s MB\n", field(m.text.FieldHeap), v)
		}
		if v := cfg["hw.cpu.ncore"]; v != "" {
			fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldCPUCores), v)
		}
		if v := storageMBFromConfig(cfg["disk.dataPartition.size"]); v != "" {
			fmt.Fprintf(&b, "%s%s MB\n", field(m.text.FieldStorage), v)
		}
		if v := cfg["hw.lcd.density"]; v != "" {
			fmt.Fprintf(&b, "%s%s\n", field(m.text.FieldDensity), v)
		}
	}

	return b.String()
}
