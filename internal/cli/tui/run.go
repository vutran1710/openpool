package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
)

func inputInit() tea.Cmd {
	return textinput.Blink
}

func Run(user, pool, registry string, joinedPools []string, needsOnboarding bool) error {
	p := tea.NewProgram(
		newApp(user, pool, registry, joinedPools, needsOnboarding),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

func RunOrFallback(user, pool, registry string, joinedPools []string, needsOnboarding bool) {
	if err := Run(user, pool, registry, joinedPools, needsOnboarding); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
