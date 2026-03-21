package screens

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/chat"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// ConversationOpenMsg is emitted when a conversation is selected from the panel.
type ConversationOpenMsg struct {
	PeerMatchHash string
}

type HomeScreen struct {
	Menu          components.Menu
	Conversations components.ConversationsPanel
	menuFocused   bool
	width         int
	height        int
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
		Menu:          components.NewMenu("", items),
		Conversations: components.NewConversationsPanel(),
		menuFocused:   true,
	}
}

func (s HomeScreen) SetConversations(convos []chat.Conversation) HomeScreen {
	s.Conversations = s.Conversations.SetConversations(convos)
	return s
}

func (s HomeScreen) SetSize(w, h int) HomeScreen {
	s.width = w
	s.height = h
	rightW := w / 2
	s.Conversations = s.Conversations.SetSize(rightW, h)
	return s
}

func (s HomeScreen) Update(msg tea.Msg) (HomeScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "tab" {
			s.menuFocused = !s.menuFocused
			s.Conversations = s.Conversations.SetFocused(!s.menuFocused)
			return s, nil
		}

	case components.ConversationSelectMsg:
		return s, func() tea.Msg {
			return ConversationOpenMsg{PeerMatchHash: msg.PeerMatchHash}
		}
	}

	if s.menuFocused {
		var cmd tea.Cmd
		s.Menu, cmd = s.Menu.Update(msg)
		return s, cmd
	}

	var cmd tea.Cmd
	s.Conversations, cmd = s.Conversations.Update(msg)
	return s, cmd
}

func (s HomeScreen) View() string {
	leftW := s.width / 2
	if leftW < 20 {
		leftW = 20
	}
	rightW := s.width - leftW - 1 // -1 for border

	leftContent := components.ScreenLayout("Home", "", s.Menu.View())
	leftPanel := lipgloss.NewStyle().Width(leftW).Render(leftContent)

	border := lipgloss.NewStyle().
		Foreground(theme.Border).
		Render("│")

	convoHeader := theme.BoldStyle.Render("Conversations")
	if !s.menuFocused {
		convoHeader = theme.BrandStyle.Render("Conversations")
	}
	rightContent := convoHeader + "\n\n" + s.Conversations.View()
	rightPanel := lipgloss.NewStyle().Width(rightW).Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, border, rightPanel)
}

func (s HomeScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "tab", Desc: "switch panel"},
		{Key: "/", Desc: "command"},
		{Key: "q", Desc: "quit"},
	}
}
