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

func Run(userName, userHash, pool, registry string, poolStatuses map[string]string, poolIssues map[string]int, needsOnboarding bool) error {
	a := newApp(userName, userHash, pool, registry, poolStatuses, poolIssues, needsOnboarding)
	p := tea.NewProgram(
		&a,
		tea.WithAltScreen(),
	)
	a.program = p

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

func RunOrFallback(userName, userHash, pool, registry string, poolStatuses map[string]string, poolIssues map[string]int, needsOnboarding bool) {
	if err := Run(userName, userHash, pool, registry, poolStatuses, poolIssues, needsOnboarding); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
