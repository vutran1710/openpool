package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type MatchesScreen struct {
	Menu  components.Menu
	Empty bool
}

func NewMatchesScreen() MatchesScreen {
	return MatchesScreen{
		Empty: true,
	}
}

func (s MatchesScreen) Update(msg tea.Msg) (MatchesScreen, tea.Cmd) {
	var cmd tea.Cmd
	s.Menu, cmd = s.Menu.Update(msg)
	return s, cmd
}

func (s MatchesScreen) View() string {
	if s.Empty {
		return "\n  " + theme.DimStyle.Render("No matches yet. Try discovering profiles.") + "\n"
	}
	return s.Menu.View()
}

func (s MatchesScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "open chat"},
		{Key: "esc", Desc: "back"},
	}
}
