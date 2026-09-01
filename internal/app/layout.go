package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"android-toolbox/internal/adb"
)

const (
	leftPaneWidthFrac = 0.38
	minLeftPaneWidth  = 28
	maxLeftPaneWidth  = 46
	// paneGap is rendered as a single blank column between the two boxes.
	paneGap = 1
	// paneFrameWidth/paneFrameHeight are how much a pane box style (1-char
	// border + Padding(0,1) on each side) adds beyond the content
	// width/height passed to lipgloss's Width()/Height() - confirmed
	// empirically: Width(w) renders at w+2, Height(h) renders at h+2.
	paneFrameWidth  = 4 // 2 border + 2 padding (left+right)
	paneFrameHeight = 2 // 2 border (top+bottom); Padding(0,1) adds none vertically
	// categoryPillsHeight is the one line the category filter pills row
	// takes up above the action list, inside the left pane's content area.
	categoryPillsHeight = 1
)

// paneWidths splits the terminal width into a left (action list) and right
// (preview/operate) pane OUTER width - i.e. including each box's own border.
// The left pane is deliberately capped so it stays a scannable list even on
// very wide terminals, handing the extra room to the right pane instead.
func paneWidths(totalWidth int) (left, right int) {
	if totalWidth <= 0 {
		return 0, 0
	}
	left = int(float64(totalWidth) * leftPaneWidthFrac)
	if left < minLeftPaneWidth {
		left = minLeftPaneWidth
	}
	if left > maxLeftPaneWidth {
		left = maxLeftPaneWidth
	}
	if left+paneGap >= totalWidth {
		// Terminal narrower than our minimums: split evenly instead of
		// overflowing or handing the right pane a negative width.
		left = totalWidth / 2
	}
	right = totalWidth - left - paneGap
	if right < 0 {
		right = 0
	}
	return left, right
}

// bodyHeight is the space left for the two panes' outer height after the
// header (title + device badges) and footer (key hints) lines, plus a
// blank separator line above and below the pane row, plus a couple of
// spare lines of margin (the header can wrap to two lines when a device
// info error is shown, and some terminals report slightly generous
// dimensions) so real overflow is rare rather than routine.
func bodyHeight(totalHeight int) int {
	h := totalHeight - 8
	if h < 5 {
		h = 5
	}
	return h
}

// paneContentSize converts an outer pane box size into the content
// width/height to pass to lipgloss's Style.Width/Height (or to an embedded
// bubbles component's SetSize) so the rendered box, once its border and
// padding are added back, comes out to exactly the outer size callers
// computed via paneWidths/bodyHeight.
func paneContentSize(outerWidth, outerHeight int) (width, height int) {
	width = outerWidth - paneFrameWidth
	if width < 1 {
		width = 1
	}
	height = outerHeight - paneFrameHeight
	if height < 1 {
		height = 1
	}
	return width, height
}

// dashboardListHeight carves the category pills row's line out of the left
// pane's content height, so the pills row plus the properly-sized-around-it
// action list together still add up to exactly the pane's content height
// instead of the list assuming it owns a line it doesn't.
func dashboardListHeight(contentH int) int {
	h := contentH - categoryPillsHeight
	if h < 1 {
		h = 1
	}
	return h
}

// rightPaneContentWidth is the usable text width inside the right pane at
// the model's current terminal size - used to size embedded huh dialogs
// (confirm prompts, the first-run PATH-install question, restore
// confirmations) so they wrap and align like everything else in the pane
// instead of guessing a fixed width.
func (m Model) rightPaneContentWidth() int {
	_, rightW := paneWidths(m.width)
	contentW, _ := paneContentSize(rightW, bodyHeight(m.height))
	return contentW
}

// fullScreenDialogWidth sizes a huh dialog on a screen with no left pane to
// share space with (healthcheck's first-run PATH prompt, recover's restore
// confirmation) - a capped fraction of the full terminal width, rather than
// rightPaneContentWidth's pane-relative sizing.
func (m Model) fullScreenDialogWidth() int {
	w := m.width - 4
	if w > 70 {
		w = 70
	}
	if w < 20 {
		w = 20
	}
	return w
}

// clampToBox word-wraps content to width (lipgloss.Style.Width already does
// this) and then hard-caps the result to at most height lines.
//
// This matters because Width() *wraps* rather than truncates: a single long
// line (e.g. a "Befehl: <long command>" preview line) can turn into several
// wrapped lines, silently growing a pane taller than the height its caller
// budgeted for. That both misaligns the two panes against each other (only
// one of them grew) and, if it pushes the whole screen taller than the
// terminal, is exactly what causes a resized-narrower terminal to look
// "broken" or the top of the screen to scroll out of view. Truncating the
// *content* here - before it ever reaches the bordered/padded box style -
// avoids the alternative of capping height on the already-bordered box
// (lipgloss's MaxHeight does that by slicing the rendered block itself,
// which cuts the closing border line off entirely instead of just hiding
// excess text).
func clampToBox(content string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(content)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// renderTwoPane joins a left and right pane box side by side with a
// single-column gap. Both boxes get an explicit Width AND Height (not just
// Height) - lipgloss only pads a style's box up to a size you actually set;
// leaving Width unset would let each box shrink to fit whatever its own
// content's widest line happens to be, ignoring the pane widths the caller
// computed, and breaking alignment between the two boxes' borders.
func renderTwoPane(leftStyle, rightStyle lipgloss.Style, leftContent, rightContent string, leftOuterWidth, rightOuterWidth, outerHeight int) string {
	leftContentW, innerHeight := paneContentSize(leftOuterWidth, outerHeight)
	rightContentW, _ := paneContentSize(rightOuterWidth, outerHeight)
	left := leftStyle.Width(leftContentW).Height(innerHeight).Render(clampToBox(leftContent, leftContentW, innerHeight))
	right := rightStyle.Width(rightContentW).Height(innerHeight).Render(clampToBox(rightContent, rightContentW, innerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", paneGap), right)
}

// batteryBadgeStyle picks a color for the battery badge based on level and
// charging state, so the header doubles as an at-a-glance health indicator.
func (m Model) batteryBadgeStyle(b adb.Battery) lipgloss.Style {
	switch {
	case b.Charging() || b.Level > 50:
		return m.styles.BadgeGood
	case b.Level > 20:
		return m.styles.BadgeWarn
	default:
		return m.styles.BadgeBad
	}
}

// isDashboardCluster reports whether s is one of the four screens sharing
// the two-pane dashboard layout (action list left, operate/preview right).
func isDashboardCluster(s screen) bool {
	switch s {
	case screenDashboard, screenParamForm, screenConfirm, screenRunner:
		return true
	}
	return false
}

// renderHeader renders the app title plus a row of compact device "badges"
// (model, Android version, battery, IP) instead of the tall info box the
// dashboard used to show - a more modern, dashboard-like look that also
// frees up vertical space for the two panes below.
func (m Model) renderHeader() string {
	title := m.styles.Title.Render(m.text.AppTitle)

	if !isDashboardCluster(m.current) {
		if notice := m.renderUpdateNotice(); notice != "" {
			return title + "\n" + notice
		}
		return title
	}

	info := m.dashboard.info
	badges := []string{
		m.styles.Badge.Render(fmt.Sprintf("%s (%s)", orDash(info.Model), m.dashboard.serial)),
	}
	if info.IsEmulator {
		// info.Model for an emulator is usually a generic build name (e.g.
		// "sdk_gphone64_x86_64"), not the AVD it's actually running - shown
		// as its own badge so it's visible without switching to the
		// Emulators tool, using BadgeGood (not the neutral Badge style) so
		// it reads as "this is an AVD, here's which one" at a glance.
		badges = append(badges, m.styles.BadgeGood.Render(fmt.Sprintf(m.text.AVDBadgeFmt, orDash(info.AVDName))))
	}
	if info.AndroidVersion != "" {
		androidLabel := fmt.Sprintf("Android %s", info.AndroidVersion)
		if info.SDK != "" {
			androidLabel = fmt.Sprintf("%s (SDK %s)", androidLabel, info.SDK)
		}
		badges = append(badges, m.styles.Badge.Render(androidLabel))
	}
	badges = append(badges, m.batteryBadgeStyle(info.Battery).Render(fmt.Sprintf(m.text.BatteryLabelFmt, info.Battery.Level)))
	if info.IPAddress != "" {
		badges = append(badges, m.styles.Badge.Render(info.IPAddress))
	}

	row := []string{title, "  "}
	for i, b := range badges {
		if i > 0 {
			row = append(row, " ")
		}
		row = append(row, b)
	}
	header := lipgloss.JoinHorizontal(lipgloss.Center, row...)

	if m.dashboard.infoErr != nil {
		header += "\n" + m.styles.Warn.Render(fmt.Sprintf(m.text.DeviceInfoIncompleteFmt, m.dashboard.infoErr.Error()))
	}
	if notice := m.renderUpdateNotice(); notice != "" {
		header += "\n" + notice
	}
	return header
}

// renderUpdateNotice is "" unless there's something to show (a newer
// android-toolbox release and/or an outdated adb/scrcpy build known from
// the background checks - see selfupdate_check.go/toolsupdate_check.go)
// AND the user hasn't dismissed it (see the "x" key in
// screen_dashboard.go/screen_health.go).
func (m Model) renderUpdateNotice() string {
	if m.updateNoticeDismissed {
		return ""
	}
	lines := m.pendingUpdateNoticeLines()
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines, m.text.UpdateNoticeDismissHint)
	return m.styles.Subtle.Render(strings.Join(lines, "\n"))
}

// pendingUpdateNoticeLines is renderUpdateNotice without the dismissed
// check or the dismiss hint - also used by the Settings screen's "Updates"
// info block (see settingsUpdateInfo) to show the very same facts without
// duplicating how they're derived.
func (m Model) pendingUpdateNoticeLines() []string {
	var lines []string
	if text := updateNoticeText(m.text, m.latestKnownVersion); text != "" {
		lines = append(lines, text)
	}
	if text := m.toolUpdateNoticeText(); text != "" {
		lines = append(lines, text)
	}
	return lines
}

// toolUpdateNoticeText names whichever of adb/scrcpy are outdated, or ""
// if neither is (or nothing is known yet).
func (m Model) toolUpdateNoticeText() string {
	adbOutdated, scrcpyOutdated := outdatedTools(m.paths, m.latestKnownADB, m.latestKnownScrcpy)
	if !adbOutdated && !scrcpyOutdated {
		return ""
	}
	var names []string
	if adbOutdated {
		names = append(names, "adb")
	}
	if scrcpyOutdated {
		names = append(names, "scrcpy")
	}
	return fmt.Sprintf(m.text.ToolUpdateAvailableFmt, strings.Join(names, ", "))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// rightPaneStyle picks the right pane's border color for the current
// dashboard-cluster state, so what's happening is visible at a glance from
// the color alone: neutral while browsing, accent while entering
// parameters, warning while confirming something destructive, info while
// streaming, and green/red once a run finishes.
func (m Model) rightPaneStyle() lipgloss.Style {
	switch m.current {
	case screenParamForm:
		return m.styles.PaneParam
	case screenConfirm:
		return m.styles.PaneConfirm
	case screenRunner:
		if m.runner.finished {
			if m.runner.exitErr != nil {
				return m.styles.PaneFailed
			}
			return m.styles.PaneSuccess
		}
		return m.styles.PaneRunning
	default:
		return m.styles.PanePreview
	}
}

// dashboardClusterFooter renders the context-sensitive key hints for
// whichever dashboard-cluster state is active, via the previously-unused
// bubbles/help component instead of a hand-built string.
func (m Model) dashboardClusterFooter() string {
	var bindings []key.Binding
	switch m.current {
	case screenParamForm:
		bindings = []key.Binding{m.keys.Tab, m.keys.Select, m.keys.Back}
	case screenConfirm:
		bindings = []key.Binding{m.keys.Confirm, m.keys.Cancel}
	case screenRunner:
		bindings = []key.Binding{m.keys.Back}
	default:
		bindings = []key.Binding{m.keys.Run, m.keys.Filter, m.keys.NextCategory, m.keys.EditAction, m.keys.AIAction, m.keys.Backups, m.keys.Settings, m.keys.SwitchDevice, m.keys.SwitchTool, m.keys.Quit}
	}
	return m.help.render(helpBindings{bindings: bindings})
}

// viewDashboardCluster composes the shared two-pane screen for
// {screenDashboard, screenParamForm, screenConfirm, screenRunner}: a header
// (title + device badges), the action list as a permanent left pane, a
// right pane whose content and border color depend on m.current, and a
// context-sensitive footer.
func (m Model) viewDashboardCluster() string {
	leftW, rightW := paneWidths(m.width)
	leftContentW, _ := paneContentSize(leftW, bodyHeight(m.height))
	left := m.renderCategoryPills(leftContentW) + "\n" + m.dashboard.actionList.View()

	var right string
	switch m.current {
	case screenParamForm:
		right = m.viewParamForm()
	case screenConfirm:
		right = m.viewConfirm()
	case screenRunner:
		right = m.viewRunner()
	default:
		right = m.viewDashboard()
	}

	panes := renderTwoPane(m.styles.PaneLeft, m.rightPaneStyle(), left, right, leftW, rightW, bodyHeight(m.height))

	return m.renderHeader() + "\n\n" + panes + "\n\n" + m.dashboardClusterFooter()
}

// renderCategoryPills renders the category filter row shown above the
// action list: one pill per Action.Category value found in actions.yaml
// (plus a leading "Alle" for no filter), the currently selected one
// highlighted in Android green.
//
// When every pill doesn't fit in width, this scrolls the row instead of
// just hard-truncating the tail: a plain trailing "…" would permanently
// hide every category past whatever fit first, with no way to see (or
// even know about) the rest. Instead a window of pills that always
// includes the selected one is shown, with a "…" on whichever side(s)
// still have hidden categories - so tabbing further keeps scrolling the
// window into view rather than tabbing into invisible categories.
func (m Model) renderCategoryPills(width int) string {
	categories := m.dashboard.categories
	n := len(categories)
	if n == 0 {
		return ""
	}

	styled := make([]string, n)
	pillWidth := make([]int, n)
	totalWidth := 0
	for i, cat := range categories {
		label := categoryDisplayLabel(cat, m.text)
		style := m.styles.PillInactive
		if i == m.dashboard.categoryIdx {
			style = m.styles.PillActive
		}
		styled[i] = style.Render(label)
		pillWidth[i] = lipgloss.Width(styled[i])
		totalWidth += pillWidth[i]
	}
	totalWidth += n - 1 // gaps between pills

	if totalWidth <= width {
		return strings.Join(styled, " ")
	}

	// Reserve room for a "…" indicator plus its connecting gap on both
	// sides up front, even though at most one side might end up needing
	// it - overshooting this budget could make the final row (indicators
	// included) wider than width, undershooting it can only ever show one
	// fewer pill than strictly necessary.
	selected := m.dashboard.categoryIdx
	avail := width - 4
	if avail < pillWidth[selected] {
		avail = pillWidth[selected] // the selected pill itself is never hidden
	}

	start, end := selected, selected
	used := pillWidth[selected]
	for {
		grew := false
		if end+1 < n && used+1+pillWidth[end+1] <= avail {
			end++
			used += 1 + pillWidth[end]
			grew = true
		}
		if start-1 >= 0 && used+1+pillWidth[start-1] <= avail {
			start--
			used += 1 + pillWidth[start]
			grew = true
		}
		if !grew {
			break
		}
	}

	parts := make([]string, 0, end-start+3)
	if start > 0 {
		parts = append(parts, m.styles.Subtle.Render("…"))
	}
	parts = append(parts, styled[start:end+1]...)
	if end < n-1 {
		parts = append(parts, m.styles.Subtle.Render("…"))
	}
	row := strings.Join(parts, " ")
	return ansi.Truncate(row, width, "") // safety net; a no-op if it already fits
}
