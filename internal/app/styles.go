package app

import (
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// colorAndroidGreen/colorAndroidDark are Android's own brand colors (see
// developer.android.com/distribute/marketing-tools/brand-guidelines): the
// signature green from the current Android logo, and the deep teal/navy
// used alongside it. Used throughout below so the toolbox visibly reads as
// "an Android tool" rather than a generic TUI - light-mode gets a slightly
// deeper shade of the same green for contrast against a white background.
var (
	colorAndroidGreen = lipgloss.AdaptiveColor{Light: "#0F9D58", Dark: "#3DDC84"}
	colorAndroidDark  = lipgloss.AdaptiveColor{Light: "#0B4F44", Dark: "#5B8C7E"}
)

// Text colors adapt to the terminal's light/dark background (lipgloss
// detects this automatically) so the UI reads well either way, instead of
// the fixed ANSI-256 values used previously. Semantic status colors (error/
// warn/info) stay conventional red/amber/blue for usability - branding
// lives in the neutral/positive chrome (title, emphasis, success, badges)
// instead of overriding colors people rely on to mean "something's wrong".
var (
	colorTitle     = colorAndroidGreen
	colorSubtle    = lipgloss.AdaptiveColor{Light: "240", Dark: "246"}
	colorError     = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorWarn      = lipgloss.AdaptiveColor{Light: "130", Dark: "214"}
	colorOK        = colorAndroidGreen
	colorInfo      = lipgloss.AdaptiveColor{Light: "25", Dark: "39"}
	colorHighlight = colorAndroidGreen
	colorBorderDim = lipgloss.AdaptiveColor{Light: "252", Dark: "238"}
	colorPreview   = colorAndroidDark
	// colorAccent is deliberately NOT one of the brand colors above: it's
	// used only for the "entering parameters" pane border, which must stay
	// visually distinct from both the idle (colorPreview) and finished
	// (colorOK) pane states - reusing a brand color for a third state would
	// collide with one of those and defeat the point of color-coding them.
	colorAccent = lipgloss.AdaptiveColor{Light: "29", Dark: "86"}
)

// styles centralises the app's look, so every screen renders consistently.
type styles struct {
	Title     lipgloss.Style
	Subtle    lipgloss.Style
	Error     lipgloss.Style
	Warn      lipgloss.Style
	OK        lipgloss.Style
	Info      lipgloss.Style
	Box       lipgloss.Style
	Highlight lipgloss.Style
	StatusBar lipgloss.Style

	// Pane styles: the dashboard cluster (action list | operate window) uses
	// one border color per right-pane state, so what's currently happening
	// is visible at a glance even before reading any text - not just "two
	// boxes side by side" but "this box's color tells you its state".
	PaneLeft    lipgloss.Style // action list (always neutral)
	PanePreview lipgloss.Style // right pane: browsing, nothing started yet
	PaneParam   lipgloss.Style // right pane: entering parameters
	PaneConfirm lipgloss.Style // right pane: confirming a destructive action
	PaneRunning lipgloss.Style // right pane: streaming output
	PaneSuccess lipgloss.Style // right pane: finished without error
	PaneFailed  lipgloss.Style // right pane: finished with an error

	// Header badges (device model/version/battery/IP chips).
	Badge     lipgloss.Style
	BadgeGood lipgloss.Style
	BadgeWarn lipgloss.Style
	BadgeBad  lipgloss.Style

	// Category filter pills above the action list.
	PillActive   lipgloss.Style // the currently selected category
	PillInactive lipgloss.Style // every other category
}

func newStyles() styles {
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)

	return styles{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Padding(0, 1),
		Subtle:    lipgloss.NewStyle().Foreground(colorSubtle),
		Error:     lipgloss.NewStyle().Bold(true).Foreground(colorError),
		Warn:      lipgloss.NewStyle().Foreground(colorWarn),
		OK:        lipgloss.NewStyle().Foreground(colorOK),
		Info:      lipgloss.NewStyle().Foreground(colorInfo),
		Box:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorderDim).Padding(1, 2),
		Highlight: lipgloss.NewStyle().Bold(true).Foreground(colorHighlight),
		StatusBar: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "238", Dark: "250"}).Padding(0, 1),

		PaneLeft:    pane.BorderForeground(colorBorderDim),
		PanePreview: pane.BorderForeground(colorPreview),
		PaneParam:   pane.BorderForeground(colorAccent),
		PaneConfirm: pane.BorderForeground(colorWarn),
		PaneRunning: pane.BorderForeground(colorInfo),
		PaneSuccess: pane.BorderForeground(colorOK),
		PaneFailed:  pane.BorderForeground(colorError),

		Badge: lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("240")).Padding(0, 1),
		// Android's brand guidelines pair the signature green with black
		// text, not white - matching the official lockup instead of our
		// other (white-on-color) badges.
		BadgeGood: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("#3DDC84")).Bold(true).Padding(0, 1),
		BadgeWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1),
		BadgeBad:  lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("160")).Bold(true).Padding(0, 1),

		// Non-adaptive, matching BadgeGood/the list title bar: the active
		// pill uses the fixed brand green with black text in both light and
		// dark terminals rather than needing a second adaptive foreground.
		PillActive:   lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("#3DDC84")).Bold(true).Padding(0, 1),
		PillInactive: lipgloss.NewStyle().Foreground(colorSubtle).Padding(0, 1),
	}
}

// androidListItemStyles starts from bubbles/list's own defaults and only
// swaps the selected-item colors: list.NewDefaultItemStyles() ships stock
// Charm pink/purple (#EE6FF8/#AD58B4) for the one piece of chrome that's
// visible almost constantly (whichever item is currently highlighted in the
// action list, device list, or backup list) - leaving it unstyled would mean
// the most-seen accent color in the whole app still reads as "generic Charm
// app" rather than Android green.
func androidListItemStyles() list.DefaultItemStyles {
	s := list.NewDefaultItemStyles()
	s.SelectedTitle = s.SelectedTitle.
		BorderForeground(colorAndroidGreen).
		Foreground(colorAndroidGreen)
	s.SelectedDesc = s.SelectedDesc.
		BorderForeground(colorAndroidGreen).
		Foreground(colorAndroidDark)
	return s
}

// androidListStyles starts from bubbles/list's own chrome defaults and
// re-colors the two spots that default to stock Charm colors: the title bar
// (background color "62", a blue-violet) and the filter cursor (#EE6FF8,
// the same pink as the selected item). Everything else (pagination dots,
// status bar, help text) is already a neutral gray that doesn't compete
// with the branding, so it's left alone.
func androidListStyles() list.Styles {
	s := list.DefaultStyles()
	// Non-adaptive, matching BadgeGood: Android's brand guidelines pair the
	// signature green with black text specifically, not white - using the
	// same fixed green in both light/dark keeps that pairing legible either
	// way instead of needing a second black/cream foreground swap.
	s.Title = s.Title.
		Background(lipgloss.Color("#3DDC84")).
		Foreground(lipgloss.Color("0"))
	s.FilterCursor = s.FilterCursor.Foreground(colorAndroidGreen)
	return s
}

// androidFilePickerStyles is the same swap for bubbles/filepicker (used by
// the APK Info tool's file browser - see screen_apkinfo.go): its cursor and
// selected-entry colors default to the same stock Charm pink (#212/"212")
// everything else above already replaces.
func androidFilePickerStyles() filepicker.Styles {
	s := filepicker.DefaultStyles()
	s.Cursor = s.Cursor.Foreground(colorAndroidGreen)
	s.Selected = s.Selected.Foreground(colorAndroidGreen)
	return s
}
