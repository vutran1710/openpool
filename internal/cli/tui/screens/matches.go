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
	var body string
	if s.Empty {
		body = theme.DimStyle.Render("No matches yet. Try discovering profiles.")
	} else {
		body = s.Menu.View()
	}
	return components.ScreenLayout("Matches", components.DimHints("your connections"), body)
}

func (s MatchesScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "open chat"},
		{Key: "esc", Desc: "back"},
	}
}
