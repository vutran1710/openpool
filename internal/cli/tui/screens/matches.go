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
	items := []components.MenuItem{
		{Key: "3f90a", Label: "♥ 3f90a", Desc: "Alex · Berlin"},
		{Key: "b12c4", Label: "♥ b12c4", Desc: "Sam · Tokyo"},
	}
	return MatchesScreen{
		Menu: components.NewMenu("Matches", items),
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
