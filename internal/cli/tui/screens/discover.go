package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// DiscoverPollMsg triggers a background sync check.
type DiscoverPollMsg struct{}

// DiscoverLikeMsg signals the user wants to like someone.
type DiscoverLikeMsg struct {
	TargetMatchHash string
}

// DiscoverMsg carries loaded suggestions to the screen.
type DiscoverMsg struct {
	Err error
}

type DiscoverScreen struct {
	Loading bool
	Empty   bool
	Width   int
	err     error
}

func NewDiscoverScreen() DiscoverScreen {
	return DiscoverScreen{}
}

func LoadDiscoverCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		return DiscoverMsg{Err: nil}
	}
}

func (s DiscoverScreen) Update(msg tea.Msg) (DiscoverScreen, tea.Cmd) {
	switch msg.(type) {
	case DiscoverMsg:
		s.Loading = false
		s.Empty = true
	}
	return s, nil
}

func (s DiscoverScreen) View() string {
	body := theme.DimStyle.Render("  Discovery is being migrated to the new chain encryption engine.\n  Coming soon.")
	return components.ScreenLayout("Discover", "", body)
}

func (s DiscoverScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "esc", Desc: "back"},
	}
}
