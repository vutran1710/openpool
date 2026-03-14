package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type MenuItem struct {
	Key   string
	Label string
	Desc  string
}

type MenuSelectMsg struct {
	Key string
}

type Menu struct {
	Items  []MenuItem
	Cursor int
	Width  int
	Title  string
}

func NewMenu(title string, items []MenuItem) Menu {
	return Menu{
		Title: title,
		Items: items,
	}
}

func (m Menu) Update(msg tea.Msg) (Menu, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Items)-1 {
				m.Cursor++
			}
		case "enter":
			if m.Cursor < len(m.Items) {
				key := m.Items[m.Cursor].Key
				return m, func() tea.Msg { return MenuSelectMsg{Key: key} }
			}
		}
	}
	return m, nil
}

func (m Menu) View() string {
	s := ""

	if m.Title != "" {
		s += "  " + theme.BoldStyle.Render(m.Title) + "\n\n"
	}

	for i, item := range m.Items {
		cursor := "  "
		label := theme.NormalItem
		if i == m.Cursor {
			cursor = theme.Cursor()
			label = theme.ActiveItem
		}

		line := fmt.Sprintf("  %s%s", cursor, label.Render(item.Label))
		if item.Desc != "" {
			line += "  " + theme.DimStyle.Render(item.Desc)
		}
		s += line + "\n"
	}

	return lipgloss.NewStyle().Padding(1, 0).Render(s)
}
