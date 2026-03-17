package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
)

type HomeScreen struct {
	Menu components.Menu
}

func NewHomeScreen() HomeScreen {
	items := []components.MenuItem{
		{Key: "discover", Label: "Discover", Desc: "find new people"},
		{Key: "matches", Label: "Matches", Desc: "view your matches"},
		{Key: "inbox", Label: "Inbox", Desc: "incoming interests"},
		{Key: "pools", Label: "Pools", Desc: "browse & manage pools"},
		{Key: "profile", Label: "Profile", Desc: "edit your profile"},
		{Key: "settings", Label: "Settings", Desc: "pool, registry, identity"},
	}
	return HomeScreen{
		Menu: components.NewMenu("", items),
	}
}

func (s HomeScreen) Update(msg tea.Msg) (HomeScreen, tea.Cmd) {
	var cmd tea.Cmd
	s.Menu, cmd = s.Menu.Update(msg)
	return s, cmd
}

func (s HomeScreen) View() string {
	return s.Menu.View()
}

func (s HomeScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "/", Desc: "command"},
		{Key: "q", Desc: "quit"},
	}
}
