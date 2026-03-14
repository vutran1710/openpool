package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	brand      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	highlight  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D")).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	activeItem = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D")).Bold(true)
	normalItem = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
)

type screen int

const (
	screenMain screen = iota
	screenAuth
	screenPools
	screenDiscover
	screenMatches
	screenProfile
)

type menuItem struct {
	label   string
	desc    string
	action  func() tea.Cmd
	screen  screen
}

type model struct {
	screen    screen
	cursor    int
	items     []menuItem
	width     int
	height    int
	message   string
	quitting  bool
	registered bool
	poolActive bool
}

func initialModel(registered, poolActive bool) model {
	m := model{
		screen:     screenMain,
		registered: registered,
		poolActive: poolActive,
	}
	m.items = m.mainMenu()
	return m
}

func (m model) mainMenu() []menuItem {
	items := []menuItem{
		{label: "Get Started", desc: "Create your identity", screen: screenAuth},
		{label: "Pools", desc: "Browse and join dating pools", screen: screenPools},
		{label: "Discover", desc: "Find new people", screen: screenDiscover},
		{label: "Matches", desc: "View your matches", screen: screenMatches},
		{label: "Profile", desc: "Edit your profile", screen: screenProfile},
	}
	return items
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case statusMsg:
		m.message = string(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectItem()
	case "esc":
		if m.screen != screenMain {
			m.screen = screenMain
			m.items = m.mainMenu()
			m.cursor = 0
			m.message = ""
		}
	}
	return m, nil
}

func (m model) selectItem() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.items) {
		return m, nil
	}

	item := m.items[m.cursor]
	if item.action != nil {
		return m, item.action()
	}

	switch item.screen {
	case screenAuth:
		m.screen = screenAuth
		m.items = m.authMenu()
	case screenPools:
		m.screen = screenPools
		m.items = m.poolsMenu()
	case screenDiscover:
		if !m.poolActive {
			m.message = "Join a pool first"
			return m, nil
		}
		m.screen = screenDiscover
		m.items = m.discoverMenu()
	case screenMatches:
		if !m.poolActive {
			m.message = "Join a pool first"
			return m, nil
		}
		m.screen = screenMatches
		m.items = m.matchesMenu()
	case screenProfile:
		m.screen = screenProfile
		m.items = m.profileMenu()
	}
	m.cursor = 0
	m.message = ""
	return m, nil
}

type statusMsg string

func (m model) authMenu() []menuItem {
	return []menuItem{
		{label: "Register", desc: "Create a new identity"},
		{label: "Who Am I", desc: "Show current identity"},
	}
}

func (m model) poolsMenu() []menuItem {
	return []menuItem{
		{label: "Browse Pools", desc: "Discover available pools"},
		{label: "Join Pool", desc: "Join a pool by name"},
		{label: "My Pools", desc: "List pools you've joined"},
		{label: "Switch Pool", desc: "Change active pool"},
	}
}

func (m model) discoverMenu() []menuItem {
	return []menuItem{
		{label: "Fetch", desc: "Discover a random profile"},
		{label: "Inbox", desc: "View incoming interests"},
	}
}

func (m model) matchesMenu() []menuItem {
	return []menuItem{
		{label: "View Matches", desc: "See all your matches"},
		{label: "Chat", desc: "Chat with a match"},
	}
}

func (m model) profileMenu() []menuItem {
	return []menuItem{
		{label: "Edit Profile", desc: "Update your bio, city, interests"},
		{label: "Show Profile", desc: "View your current profile"},
	}
}

func (m model) View() string {
	if m.quitting {
		return "\n  " + dimStyle.Render("Bye! 💜") + "\n\n"
	}

	s := "\n"
	s += "  " + brand.Render("♥ dating") + dimStyle.Render("  interactive mode") + "\n"
	s += "  " + dimStyle.Render("─────────────────────────────") + "\n\n"

	title := m.screenTitle()
	if title != "" {
		s += "  " + highlight.Render(title) + "\n\n"
	}

	for i, item := range m.items {
		cursor := "  "
		style := normalItem
		if i == m.cursor {
			cursor = brand.Render("→ ")
			style = activeItem
		}

		s += fmt.Sprintf("  %s%s", cursor, style.Render(item.label))
		s += "  " + dimStyle.Render(item.desc) + "\n"
	}

	s += "\n"

	if m.message != "" {
		s += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD93D")).Render("  "+m.message) + "\n\n"
	}

	nav := "↑/↓ navigate  enter select"
	if m.screen != screenMain {
		nav += "  esc back"
	}
	nav += "  q quit"
	s += "  " + dimStyle.Render(nav) + "\n"

	return s
}

func (m model) screenTitle() string {
	switch m.screen {
	case screenAuth:
		return "Authentication"
	case screenPools:
		return "Pools"
	case screenDiscover:
		return "Discover"
	case screenMatches:
		return "Matches"
	case screenProfile:
		return "Profile"
	default:
		return ""
	}
}
