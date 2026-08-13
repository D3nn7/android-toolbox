package app

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// androidHuhTheme builds a huh.Theme (used for the app's interactive yes/no
// dialogs - see huh.NewConfirm in screen_confirm.go, screen_health.go, and
// screen_recover.go) matching the rest of the UI's Android-green branding,
// instead of huh's own default look.
func androidHuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	blurredButtonBG := lipgloss.AdaptiveColor{Light: "252", Dark: "237"}

	t.Focused.Base = t.Focused.Base.BorderForeground(colorAndroidGreen)
	t.Focused.Title = t.Focused.Title.Foreground(colorAndroidGreen).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(colorSubtle)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(colorError)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(colorError)
	// The highlighted Yes/No button: solid Android green with BLACK text -
	// same pairing as BadgeGood/PillActive (Android's brand guidelines pair
	// the signature green with black, not white/cream, for contrast) - was
	// previously a light cream foreground, which read as too washed-out
	// against the green background.
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("0")).Bold(true).Background(lipgloss.Color("#0F9D58"))
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(colorSubtle).Background(blurredButtonBG)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())

	return t
}
