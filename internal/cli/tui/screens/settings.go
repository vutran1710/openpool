package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// PoolSwitchMsg is sent when the user switches the active pool.
type PoolSwitchMsg struct {
	Name string
}

// RegistrySwitchMsg is sent when the user switches the active registry.
type RegistrySwitchMsg struct {
	Repo string
}

type settingsItem int

const (
	settingsRegistry settingsItem = iota
	settingsIdentity
)

// SettingsScreen shows app settings: active pool, registry, identity.
type SettingsScreen struct {
	cursor   settingsItem
	pool     string   // current active pool name
	registry string   // current active registry
	pools    []string // joined pool names
	regs     []string // configured registries
	user     string
	provider string
	Width    int
	Height   int

	// Picker state
	picking    bool         // true when showing picker list
	pickTarget settingsItem // what we're picking (pool or registry)
	pickItems  []string     // items in the picker
	pickCursor int          // cursor in picker
}

func NewSettingsScreen(pool, registry, user, provider string, pools []string, regs []string) SettingsScreen {
	return SettingsScreen{
		pool:     pool,
		registry: registry,
		pools:    pools,
		regs:     regs,
		user:     user,
		provider: provider,
	}
}

func (s SettingsScreen) Update(msg tea.Msg) (SettingsScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s.picking {
			return s.updatePicker(msg)
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < settingsIdentity {
				s.cursor++
			}
		case "enter":
			switch s.cursor {
			case settingsRegistry:
				if len(s.regs) > 0 {
					s.picking = true
					s.pickTarget = settingsRegistry
					s.pickItems = s.regs
					s.pickCursor = 0
					for i, r := range s.regs {
						if r == s.registry {
							s.pickCursor = i
							break
						}
					}
				}
			case settingsIdentity:
				return s, func() tea.Msg {
					return components.ToastMsg{
						Text:  "Run: dating auth whoami",
						Level: components.ToastInfo,
					}
				}
			}
		}
	}
	return s, nil
}

func (s SettingsScreen) updatePicker(msg tea.KeyMsg) (SettingsScreen, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if s.pickCursor > 0 {
			s.pickCursor--
		}
	case "down", "j":
		if s.pickCursor < len(s.pickItems)-1 {
			s.pickCursor++
		}
	case "enter":
		selected := s.pickItems[s.pickCursor]
		s.picking = false
		switch s.pickTarget {
		case settingsRegistry:
			s.registry = selected
			return s, func() tea.Msg { return RegistrySwitchMsg{Repo: selected} }
		}
	case "esc":
		s.picking = false
	}
	return s, nil
}

func (s SettingsScreen) View() string {
	if s.picking {
		return s.pickerView()
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Dim).
		MarginBottom(1).
		MarginLeft(2)

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 2).
		MarginLeft(2).
		MarginBottom(0).
		Width(min(60, s.Width-6))

	activeCardStyle := cardStyle.BorderForeground(theme.Pink)

	var cards []string

	// Registry card
	regValue := theme.DimStyle.Render(s.registry)
	if s.registry == "" {
		regValue = theme.DimStyle.Render("not set")
	}
	regCard := renderSettingsCard("⊞", "Active Registry", regValue, "/registry <owner/repo>", s.cursor == settingsRegistry, cardStyle, activeCardStyle)
	cards = append(cards, regCard)

	// Identity card
	idValue := theme.AccentStyle.Render(s.user)
	if s.provider != "" {
		idValue += theme.DimStyle.Render(" · " + s.provider)
	}
	idCard := renderSettingsCard("⬡", "Identity", idValue, "dating auth whoami", s.cursor == settingsIdentity, cardStyle, activeCardStyle)
	cards = append(cards, idCard)

	return titleStyle.Render("SETTINGS") + "\n" + lipgloss.JoinVertical(lipgloss.Left, cards...)
}

func (s SettingsScreen) pickerView() string {
	title := "Select Registry"

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Dim).
		MarginBottom(1).
		MarginLeft(2)

	var items string
	for i, item := range s.pickItems {
		cursor := "  "
		style := theme.DimStyle
		if i == s.pickCursor {
			cursor = theme.Cursor()
			style = theme.ActiveItem
		}
		// Mark current
		marker := ""
		if item == s.registry {
			marker = theme.GreenStyle.Render(" ✓")
		}
		items += cursor + style.Render(item) + marker + "\n"
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		MarginLeft(2).
		Width(min(60, s.Width-6)).
		Render(items)

	return titleStyle.Render(title) + "\n" + box
}

func renderSettingsCard(icon, label, value, hint string, active bool, normal, activeSty lipgloss.Style) string {
	sty := normal
	if active {
		sty = activeSty
	}

	row1 := theme.DimStyle.Render(icon+" ") + lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(label)
	row2 := "  " + value
	row3 := theme.DimStyle.Render("  " + hint)

	return sty.Render(row1 + "\n" + row2 + "\n" + row3)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s SettingsScreen) HelpBindings() []components.KeyBind {
	if s.picking {
		return []components.KeyBind{
			{Key: "↑↓", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
			{Key: "esc", Desc: "cancel"},
		}
	}
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "esc", Desc: "back"},
	}
}
