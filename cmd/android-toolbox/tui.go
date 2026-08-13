package main

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"android-toolbox/internal/app"
)

// runTUI launches the interactive Bubbletea application.
func runTUI(ctx context.Context) error {
	ac := appContextFrom(ctx)

	model := app.New(ctx, ac.Paths, ac.Settings, ac.State, ac.Log)

	p := tea.NewProgram(model, tea.WithAltScreen())
	return ac.Log.Guard("tui", func() error {
		_, err := p.Run()
		if err != nil {
			return fmt.Errorf("TUI exited with an error: %w", err)
		}
		return nil
	})
}
