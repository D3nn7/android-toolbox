package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// helpModel wraps bubbles/help so every screen can render a consistent
// footer from just its own list of relevant key.Binding values.
type helpModel struct {
	model help.Model
	width int
}

func newHelpModel() helpModel {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Bold(true).Foreground(colorHighlight)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colorSubtle)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colorSubtle)
	return helpModel{model: h}
}

func (h helpModel) render(km helpBindings) string {
	h.model.Width = h.width
	return h.model.View(km)
}
