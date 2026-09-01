package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"android-toolbox/internal/avd"
	"android-toolbox/internal/toolsmanager"
)

// emulatorCreateStage is where the create wizard currently is: fetching the
// device-profile/system-image lists, stepping through the form fields,
// running the (possibly multi-gigabyte) system-image download plus AVD
// creation with a live progress bar, or done.
type emulatorCreateStage int

const (
	emuCreateLoading emulatorCreateStage = iota
	emuCreateForm
	emuCreateProgress
	emuCreateDone
)

type emulatorCreateScreen struct {
	stage emulatorCreateStage
	err   error

	installedImages map[string]bool

	wizard fieldWizard

	progress progressRunner
	runID    int

	result    string
	resultErr bool
}

// emulatorCreateDataMsg carries the device-profile/system-image lists the
// wizard's select fields are built from.
type emulatorCreateDataMsg struct {
	deviceProfiles []string
	installed      []string
	available      []string
	err            error
}

func loadEmulatorCreateDataCmd(ctx context.Context, manager *avd.Manager) tea.Cmd {
	return func() tea.Msg {
		profiles, err := manager.ListDeviceProfiles(ctx)
		if err != nil {
			return emulatorCreateDataMsg{err: err}
		}
		installed, available, err := toolsmanager.ListSdkPackages(ctx, manager.SdkManagerPath, manager.SdkRoot)
		if err != nil {
			return emulatorCreateDataMsg{err: err}
		}
		return emulatorCreateDataMsg{deviceProfiles: profiles, installed: installed, available: available}
	}
}

// enterEmulatorCreate switches to the create wizard, kicking off the
// device-profile/system-image fetch its form fields need.
func (m Model) enterEmulatorCreate() (Model, tea.Cmd) {
	m.emulatorCreate = emulatorCreateScreen{stage: emuCreateLoading}
	m.current = screenEmulatorCreate
	if m.avdManager == nil {
		m.emulatorCreate.err = errNoAvdTools(m.text)
		return m, nil
	}
	return m, loadEmulatorCreateDataCmd(m.ctx, m.avdManager)
}

// enterEmulatorList switches to (or returns to) the emulator list, kicking
// off a fresh AVD/running-status fetch.
func (m Model) enterEmulatorList() (Model, tea.Cmd) {
	el, cmd := newEmulatorListScreen(m)
	m.emulatorList = el
	// A freshly constructed list.Model starts at size 0x0 and only resizes
	// on the next tea.WindowSizeMsg (see app.go's New() for the same
	// concern on every other list) - which, on a real terminal, only fires
	// on an actual resize event, not on navigating to this screen.
	leftW, _ := paneWidths(m.width)
	leftContentW, contentH := paneContentSize(leftW, bodyHeight(m.height))
	m.emulatorList.list.SetSize(leftContentW, contentH)
	m.current = screenEmulatorList
	return m, cmd
}

func errNoAvdTools(t uiText) error {
	return errors.New(t.EmulatorNoToolsMsg)
}

func withPrefix(system []string, prefix string) []string {
	out := make([]string, 0, len(system))
	for _, p := range system {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out
}

// apiLevelToAndroidVersion maps an SDK API level - as it appears in a
// system-images package path, e.g. "system-images;android-34;..." - to its
// marketing Android version name. API levels and marketing version numbers
// have never lined up (API 34 is Android 14, not "Android 34"); without
// this, "android-17" in the picker reads as "Android 17" at a glance when
// it's actually Android 4.2 Jelly Bean - an easy, silent mis-pick.
var apiLevelToAndroidVersion = map[string]string{
	"10": "2.3.3 (Gingerbread)",
	"15": "4.0.3 (Ice Cream Sandwich)",
	"16": "4.1 (Jelly Bean)",
	"17": "4.2 (Jelly Bean)",
	"18": "4.3 (Jelly Bean)",
	"19": "4.4 (KitKat)",
	"20": "4.4W (KitKat Wear)",
	"21": "5.0 (Lollipop)",
	"22": "5.1 (Lollipop)",
	"23": "6.0 (Marshmallow)",
	"24": "7.0 (Nougat)",
	"25": "7.1 (Nougat)",
	"26": "8.0 (Oreo)",
	"27": "8.1 (Oreo)",
	"28": "9 (Pie)",
	"29": "10",
	"30": "11",
	"31": "12",
	"32": "12L",
	"33": "13",
	"34": "14",
	"35": "15",
	"36": "16",
}

var apiLevelRe = regexp.MustCompile(`android-(\d+)`)

// androidVersionLabel returns "Android <version>" for a system-images
// package path whose API level is recognized, or "" otherwise.
func androidVersionLabel(pkg string) string {
	m := apiLevelRe.FindStringSubmatch(pkg)
	if m == nil {
		return ""
	}
	v, ok := apiLevelToAndroidVersion[m[1]]
	if !ok {
		return ""
	}
	return "Android " + v
}

// systemImageMatchesHost reports whether a system-images package's ABI can
// actually run on this host - the emulator binary refuses the mismatch
// outright rather than emulating slowly ("Avd's CPU Architecture 'arm64' is
// not supported by the QEMU2 emulator on x86_64 host. System image must
// match the host architecture."), so an incompatible entry left in the
// picker just lets someone create an AVD that can never start. Only the
// x86-on-x86_64 direction is actually blocked by the emulator; an arm64
// host can run both arm64 (native) and x86_64 (translated) images, so no
// filtering is applied there.
func systemImageMatchesHost(pkg string) bool {
	switch runtime.GOARCH {
	case "amd64", "386":
		return !strings.Contains(pkg, "arm64-v8a") && !strings.Contains(pkg, "armeabi")
	default:
		return true
	}
}

// buildSystemImageOptions merges installed and available "system-images;..."
// packages into one select list, filtered to this host's runnable ABIs (see
// systemImageMatchesHost), each entry labeled with its actual Android
// version (see androidVersionLabel) and whether it's already installed or
// would need to be downloaded first.
func buildSystemImageOptions(installed, available []string) []huh.Option[string] {
	installedSet := map[string]bool{}
	for _, p := range withPrefix(installed, "system-images;") {
		installedSet[p] = true
	}
	var all []string
	for _, p := range withPrefix(installed, "system-images;") {
		if systemImageMatchesHost(p) {
			all = append(all, p)
		}
	}
	for _, p := range withPrefix(available, "system-images;") {
		if !installedSet[p] && systemImageMatchesHost(p) {
			all = append(all, p)
		}
	}

	opts := make([]huh.Option[string], 0, len(all))
	for _, p := range all {
		label := p
		if v := androidVersionLabel(p); v != "" {
			label = v + "  —  " + p
		}
		if installedSet[p] {
			label += " (installed)"
		} else {
			label += " (download required)"
		}
		opts = append(opts, huh.NewOption(label, p))
	}
	return opts
}

func (m Model) buildEmulatorCreateWizard(data emulatorCreateDataMsg) fieldWizard {
	width := m.fullScreenDialogWidth()
	t := m.text

	deviceOpts := append([]huh.Option[string]{huh.NewOption(t.EmulatorCreateDefaultDevice, "")}, stringOptions(data.deviceProfiles)...)
	imageOpts := buildSystemImageOptions(data.installed, data.available)

	steps := []fieldWizardStep{
		newInputStep(t.FieldName, "", requiredValidator(t.WizardFieldRequiredMsg), m.huhTheme, width),
		newSelectStep(t.FieldDevice, deviceOpts, true, m.huhTheme, width),
		newSelectStep(t.FieldSystemImage, imageOpts, true, m.huhTheme, width),
		// RAM/CPU/storage default to avdmanager's own usual defaults - all
		// three are applied as a config.ini patch right after "avdmanager
		// create avd" succeeds (see beginEmulatorCreateProgress), since
		// avdmanager itself has no create-time flags for them.
		newInputStep(t.FieldRAM, "2048", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
		newInputStep(t.FieldCPUCores, "4", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
		newInputStep(t.FieldStorage, "6144", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
		newInputStep(t.FieldSDCard, "0", numberValidator(t.WizardFieldNumberMsg), m.huhTheme, width),
	}
	return newFieldWizard(steps)
}

func (m Model) updateEmulatorCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.emulatorCreate.stage {
	case emuCreateLoading:
		return m.updateEmulatorCreateLoading(msg)
	case emuCreateForm:
		return m.updateEmulatorCreateForm(msg)
	case emuCreateProgress:
		return m.updateEmulatorCreateProgress(msg)
	default: // emuCreateDone
		if key, ok := msg.(tea.KeyMsg); ok && (key.String() == "esc" || key.String() == "enter") {
			return m.enterEmulatorList()
		}
		return m, nil
	}
}

func (m Model) updateEmulatorCreateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if data, ok := msg.(emulatorCreateDataMsg); ok {
		if data.err != nil {
			m.emulatorCreate.err = data.err
			return m, nil
		}
		installedSet := map[string]bool{}
		for _, p := range data.installed {
			installedSet[p] = true
		}
		m.emulatorCreate.installedImages = installedSet
		m.emulatorCreate.wizard = m.buildEmulatorCreateWizard(data)
		m.emulatorCreate.stage = emuCreateForm
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m.enterToolSelect()
	}
	return m, nil
}

func (m Model) updateEmulatorCreateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if sel, filtering := selectStepFiltering(m.emulatorCreate.wizard.Current()); filtering {
				sel.Filtering(false)
				return m, nil
			}
			return m.enterToolSelect()
		case "enter":
			if err := m.emulatorCreate.wizard.Current().Field.Error(); err == nil {
				m.emulatorCreate.wizard.Advance()
				if m.emulatorCreate.wizard.Done() {
					return m.beginEmulatorCreateProgress()
				}
			}
			return m, nil
		}
	}
	wizard, cmd := m.emulatorCreate.wizard.Update(msg)
	m.emulatorCreate.wizard = wizard
	return m, cmd
}

// beginEmulatorCreateProgress starts the download-if-needed + create task
// once every wizard field has been filled in.
func (m Model) beginEmulatorCreateProgress() (tea.Model, tea.Cmd) {
	steps := m.emulatorCreate.wizard.Steps
	name := strings.ReplaceAll(strings.TrimSpace(*steps[0].Value), " ", "_")
	device := *steps[1].Value
	image := *steps[2].Value
	ram := strings.TrimSpace(*steps[3].Value)
	cpu := strings.TrimSpace(*steps[4].Value)
	storageMB := strings.TrimSpace(*steps[5].Value)
	sdcardMB, _ := strconv.Atoi(strings.TrimSpace(*steps[6].Value))
	haveImage := m.emulatorCreate.installedImages[image]

	manager := m.avdManager
	ctx := m.ctx
	task := func(report func(string)) error {
		if !haveImage {
			if err := toolsmanager.InstallSdkPackage(ctx, manager.SdkManagerPath, manager.SdkRoot, image, report); err != nil {
				return err
			}
		}
		report(m.text.EmulatorCreateCreatingMsg)
		if err := manager.Create(ctx, avd.CreateOptions{Name: name, SystemImage: image, Device: device, SDCardMB: sdcardMB}); err != nil {
			return err
		}
		// avdmanager itself has no create-time flags for RAM/CPU/internal
		// storage - applied as a config.ini patch right after creation,
		// same mechanism the post-creation specs editor uses.
		return avd.WriteConfig(avd.AvdHome(), name, map[string]string{
			"hw.ramSize":              ram,
			"hw.cpu.ncore":            cpu,
			"disk.dataPartition.size": storageMB + "M",
		})
	}

	m.emulatorRunSeq++
	m.emulatorCreate.runID = m.emulatorRunSeq
	m.emulatorCreate.stage = emuCreateProgress
	pr, cmd := startProgressRunner(m.emulatorCreate.runID, m.fullScreenDialogWidth(), task)
	m.emulatorCreate.progress = pr
	m.emulatorCreate.result = name
	return m, cmd
}

func (m Model) updateEmulatorCreateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" && m.emulatorCreate.progress.finished {
		return m.enterEmulatorList()
	}

	pr, cmd, justFinished := updateProgressRunner(m.emulatorCreate.progress, msg)
	m.emulatorCreate.progress = pr
	if justFinished && pr.err == nil {
		m.emulatorCreate.stage = emuCreateDone
	}
	return m, cmd
}

func (m Model) viewEmulatorCreate() string {
	title := m.styles.Title.Render(m.text.EmulatorCreateTitle)

	switch m.emulatorCreate.stage {
	case emuCreateLoading:
		if m.emulatorCreate.err != nil {
			return title + "\n\n" + m.styles.Box.Render(m.styles.Error.Render(m.emulatorCreate.err.Error()))
		}
		return title + "\n\n" + m.styles.Box.Render(m.styles.Subtle.Render(m.text.EmulatorCreateLoadingMsg))
	case emuCreateForm:
		step := m.emulatorCreate.wizard.Current()
		stepCount := fmt.Sprintf(m.text.EmulatorCreateStepFmt, m.emulatorCreate.wizard.Index+1, len(m.emulatorCreate.wizard.Steps))
		content := m.styles.Subtle.Render(stepCount) + "\n" + m.styles.Highlight.Render(step.Label) + "\n\n" + step.Field.View()
		if step.Filterable {
			content += "\n" + m.styles.Subtle.Render(m.text.WizardSearchHint)
		}
		return title + "\n\n" + m.styles.Box.Render(content)
	case emuCreateProgress:
		return title + "\n\n" + m.styles.Box.Render(viewProgressRunner(m.emulatorCreate.progress, m.styles))
	default: // emuCreateDone
		content := m.styles.OK.Render(fmt.Sprintf(m.text.EmulatorCreateDoneFmt, m.emulatorCreate.result)) +
			"\n\n" + m.styles.Subtle.Render(m.text.EmulatorCreateFooter)
		return title + "\n\n" + m.styles.Box.Render(content)
	}
}
