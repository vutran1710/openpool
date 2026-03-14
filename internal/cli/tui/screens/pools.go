package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
)

type PoolsScreen struct {
	Menu components.Menu
}

func NewPoolsScreen() PoolsScreen {
	items := []components.MenuItem{
		{Key: "browse", Label: "Browse Pools", Desc: "discover available pools"},
		{Key: "join", Label: "Join Pool", Desc: "join by name"},
		{Key: "list", Label: "My Pools", Desc: "pools you've joined"},
		{Key: "switch", Label: "Switch Pool", Desc: "change active pool"},
		{Key: "create", Label: "Create Pool", Desc: "register a new pool"},
	}
	return PoolsScreen{
		Menu: components.NewMenu("Pools", items),
	}
}

func (s PoolsScreen) Update(msg tea.Msg) (PoolsScreen, tea.Cmd) {
	var cmd tea.Cmd
	s.Menu, cmd = s.Menu.Update(msg)
	return s, cmd
}

func (s PoolsScreen) View() string {
	return s.Menu.View()
}

func (s PoolsScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "esc", Desc: "back"},
	}
}
