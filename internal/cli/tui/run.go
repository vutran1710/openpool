package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(registered, poolActive bool) error {
	p := tea.NewProgram(
		initialModel(registered, poolActive),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	m := finalModel.(model)
	if m.quitting {
		return nil
	}

	return nil
}

func RunOrFallback(registered, poolActive bool) {
	if err := Run(registered, poolActive); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
