package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"android-toolbox/internal/apkinfo"
)

type apkInfoStage int

const (
	apkInfoPicking apkInfoStage = iota
	apkInfoResult
)

type apkInfoScreen struct {
	stage    apkInfoStage
	picker   filepicker.Model
	viewport viewport.Model

	// startDir is where the picker was rooted when this screen was
	// entered - esc only leaves the tool (see updateAPKInfoPicking) once
	// the user has navigated back up to this directory; while nested in a
	// subfolder, esc is left to the picker's own "go up one level"
	// handling instead (esc is part of its default Back keybinding,
	// alongside h/backspace/left).
	startDir string

	// status is a transient message shown under the file picker - e.g. "only
	// .apk files can be selected" when the user tries to pick a disallowed
	// file. Cleared on the next successful pick.
	status string

	result    apkinfo.Info
	resultErr error
}

// apkInfoPickerHeight budgets the file picker's visible row count from the
// terminal height, leaving room for viewAPKInfoPicking's own title/blank/
// status/footer lines around it (mirrors deviceListHeight's reasoning) -
// without this, the picker's default Height (0, since it only sizes itself
// from a live tea.WindowSizeMsg - see AutoHeight) leaves its internal
// pagination window collapsed to a single visible entry, which is exactly
// the "everything's squeezed onto one line" bug this fixes.
func apkInfoPickerHeight(totalHeight int) int {
	h := totalHeight - 6
	if h < 3 {
		return 3
	}
	return h
}

// apkInfoResultViewportHeight budgets the result viewport's visible row
// count the same way apkInfoPickerHeight does for the picker: title + blank
// + blank-before-footer + footer is 4 lines around it (see
// viewAPKInfoResult) - without a fixed height here, a report with many
// permissions/activities/features simply overflows the terminal with no way
// to scroll back to it, and the footer's key legend scrolls off the bottom
// of the screen along with it.
func apkInfoResultViewportHeight(totalHeight int) int {
	h := totalHeight - 4
	if h < 3 {
		return 3
	}
	return h
}

// newAPKInfoScreen builds a fresh file-picker rooted at the user's home
// directory (falling back to the working directory), restricted to .apk
// files - always rebuilt from scratch when entering this tool (see
// Model.enterTool), the same "start clean every time" convention
// enterDashboard/enterDeviceSelect already follow for their own screens.
func newAPKInfoScreen(m Model) apkInfoScreen {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".apk"}
	fp.DirAllowed = true
	fp.FileAllowed = true
	fp.Styles = androidFilePickerStyles()
	// Seeded from the already-known terminal size rather than waiting for
	// the next live resize event (AutoHeight's own WindowSizeMsg handling
	// still keeps this in sync afterward - see Model.Update's forwarding).
	fp.SetHeight(apkInfoPickerHeight(m.height))

	dir := ""
	if d, err := os.UserHomeDir(); err == nil {
		dir = d
	} else if d, err := os.Getwd(); err == nil {
		dir = d
	}
	fp.CurrentDirectory = dir

	vp := viewport.New(m.width, apkInfoResultViewportHeight(m.height))

	return apkInfoScreen{stage: apkInfoPicking, picker: fp, viewport: vp, startDir: dir}
}

func (m Model) updateAPKInfo(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.apkInfo.stage == apkInfoResult {
		return m.updateAPKInfoResult(msg)
	}
	return m.updateAPKInfoPicking(msg)
}

func (m Model) updateAPKInfoPicking(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.String() == "q":
			return m, tea.Quit
		case key.String() == "esc" && m.apkInfo.picker.CurrentDirectory == m.apkInfo.startDir:
			// Only leave the tool once already back at the directory it was
			// opened in - otherwise esc falls through to the picker's own
			// Back handling below (it's bound to esc too) and just goes up one
			// directory level, which is what a user backing out of the wrong
			// folder actually wants.
			return m.enterToolSelect()
		}
	}

	var cmd tea.Cmd
	m.apkInfo.picker, cmd = m.apkInfo.picker.Update(msg)

	if didSelect, path := m.apkInfo.picker.DidSelectFile(msg); didSelect {
		m.apkInfo.status = ""
		m.apkInfo.result, m.apkInfo.resultErr = apkinfo.Analyze(path)
		m.apkInfo.stage = apkInfoResult
		m.apkInfo.viewport.SetContent(m.apkInfoResultContent())
		m.apkInfo.viewport.GotoTop()
		return m, nil
	}
	if didSelect, _ := m.apkInfo.picker.DidSelectDisabledFile(msg); didSelect {
		m.apkInfo.status = m.text.APKInfoWrongFileType
	}

	return m, cmd
}

func (m Model) updateAPKInfoResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "esc":
			// Back to picking another file, within the same tool - ctrl+t
			// (handled globally) is the way to leave the tool entirely.
			m.apkInfo.stage = apkInfoPicking
			return m, nil
		}
	}

	// Everything else (arrows, j/k, pgup/pgdown, u/d, mouse wheel) scrolls
	// the report - it's rendered into a fixed-height viewport (see
	// apkInfoResultViewportHeight) specifically so a report with many
	// permissions/activities/certificates can be scrolled through instead
	// of overflowing the terminal with no way back to the footer.
	var cmd tea.Cmd
	m.apkInfo.viewport, cmd = m.apkInfo.viewport.Update(msg)
	return m, cmd
}

func (m Model) viewAPKInfo() string {
	if m.apkInfo.stage == apkInfoResult {
		return m.viewAPKInfoResult()
	}
	return m.viewAPKInfoPicking()
}

func (m Model) viewAPKInfoPicking() string {
	title := m.styles.Title.Render(m.text.APKInfoTitle)
	body := m.apkInfo.picker.View()

	var status string
	if m.apkInfo.status != "" {
		status = "\n" + m.styles.Warn.Render(m.apkInfo.status) + "\n"
	}

	return title + "\n\n" + body + status + "\n" + m.styles.Subtle.Render(m.text.APKInfoPickingFooter)
}

func (m Model) viewAPKInfoResult() string {
	title := m.styles.Title.Render(m.text.APKInfoTitle)
	return title + "\n\n" + m.apkInfo.viewport.View() + "\n" + m.styles.Subtle.Render(m.text.APKInfoResultFooter)
}

// apkInfoResultContent renders the analyzed report (or its error) as the
// text fed into the result viewport - wrapped to the viewport's own width
// up front, since viewport.Model doesn't wrap long lines itself (it only
// truncates/horizontally-scrolls them), and a report full of hard-to-wrap
// certificate fingerprints and package names should scroll vertically, not
// sideways.
func (m Model) apkInfoResultContent() string {
	var content string
	if m.apkInfo.resultErr != nil {
		content = m.styles.Error.Render(fmt.Sprintf(m.text.APKInfoAnalyzeErrorFmt, m.apkInfo.resultErr.Error()))
	} else {
		content = m.renderAPKInfoResult(m.apkInfo.result)
	}

	width := m.apkInfo.viewport.Width
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Width(width).Render(content)
}

// renderAPKInfoResult formats an apkinfo.Info report using this app's own
// styles/i18n text - the TUI counterpart of cmd/android-toolbox's
// formatAPKInfo, styled instead of plain-text since it's shown on screen
// rather than piped/redirected.
func (m Model) renderAPKInfoResult(info apkinfo.Info) string {
	t := m.text
	var b strings.Builder

	writeField := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", m.styles.Highlight.Render(label), value)
	}

	writeField(t.APKInfoFileLabel, info.Path)
	writeField(t.APKInfoSizeLabel, fmt.Sprintf("%s (%d bytes)", formatByteSizeTUI(info.SizeBytes), info.SizeBytes))
	writeField(t.APKInfoHashLabel, info.SHA256)
	writeField(t.APKInfoEntriesLabel, fmt.Sprintf("%d", info.EntryCount))
	b.WriteString("\n")

	mf := info.Manifest
	writeField(t.APKInfoPackageLabel, orDash(mf.PackageName))
	writeField(t.APKInfoVersionLabel, fmt.Sprintf("%s (code %d)", orDash(mf.VersionName), mf.VersionCode))
	writeField(t.APKInfoMinSDKLabel, fmt.Sprintf("%d", mf.MinSDK))
	writeField(t.APKInfoTargetSDKLabel, fmt.Sprintf("%d", mf.TargetSDK))
	writeField(t.APKInfoAppLabelLabel, orDash(mf.ApplicationLabel))
	if mf.MainActivity != "" {
		writeField(t.APKInfoMainActivityLabel, mf.MainActivity)
	}

	fmt.Fprintf(&b, "\n%s\n", m.styles.Highlight.Render(fmt.Sprintf(t.APKInfoPermissionsHeaderFmt, len(mf.Permissions))))
	for _, p := range mf.Permissions {
		fmt.Fprintf(&b, "  - %s\n", p)
	}
	if len(mf.Features) > 0 {
		fmt.Fprintf(&b, "\n%s\n", m.styles.Highlight.Render(fmt.Sprintf(t.APKInfoFeaturesHeaderFmt, len(mf.Features))))
		for _, f := range mf.Features {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	}
	if len(mf.Activities) > 0 {
		fmt.Fprintf(&b, "\n%s\n", m.styles.Highlight.Render(fmt.Sprintf(t.APKInfoActivitiesHeaderFmt, len(mf.Activities))))
		for _, a := range mf.Activities {
			fmt.Fprintf(&b, "  - %s\n", a)
		}
	}

	fmt.Fprintf(&b, "\n%s\n", m.styles.Highlight.Render(t.APKInfoSigningTitle))
	s := info.Signing
	switch {
	case s.SchemeV2 || s.SchemeV3:
		var schemes []string
		if s.SchemeV2 {
			schemes = append(schemes, "v2")
		}
		if s.SchemeV3 {
			schemes = append(schemes, "v3")
		}
		writeField(t.APKInfoSigningSchemeLabel, strings.Join(schemes, ", "))
		for i, c := range s.Certificates {
			fmt.Fprintf(&b, "%s\n", m.styles.Highlight.Render(fmt.Sprintf(t.APKInfoCertificateFmt, i+1)))
			fmt.Fprintf(&b, "  %s %s\n", m.styles.Subtle.Render(t.APKInfoCertSubjectLabel), c.Subject)
			fmt.Fprintf(&b, "  %s %s\n", m.styles.Subtle.Render(t.APKInfoCertIssuerLabel), c.Issuer)
			fmt.Fprintf(&b, "  %s %s\n", m.styles.Subtle.Render(t.APKInfoCertSerialLabel), c.SerialNumber)
			fmt.Fprintf(&b, "  %s %s - %s\n", m.styles.Subtle.Render(t.APKInfoCertValidLabel), c.NotBefore, c.NotAfter)
			fmt.Fprintf(&b, "  %s %s\n", m.styles.Subtle.Render(t.APKInfoCertSHA256Label), c.SHA256)
		}
	case s.SchemeV1Only:
		b.WriteString(m.styles.Subtle.Render(t.APKInfoSigningV1OnlyLabel) + "\n")
	default:
		b.WriteString(m.styles.Subtle.Render(t.APKInfoSigningNoneLabel) + "\n")
	}

	return b.String()
}

// formatByteSizeTUI mirrors cmd/android-toolbox's formatByteSize - kept as
// a separate copy rather than shared, since internal/app can't import a
// cmd/ package (and the reverse would pull TUI-only dependencies into the
// CLI binary for a two-line helper).
func formatByteSizeTUI(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
