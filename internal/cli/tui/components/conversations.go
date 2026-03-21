package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/chat"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type ConversationSelectMsg struct {
	PeerMatchHash string
}

type ConversationsPanel struct {
	conversations []chat.Conversation
	cursor        int
	focused       bool
	width, height int
}

func NewConversationsPanel() ConversationsPanel {
	return ConversationsPanel{}
}

func (p ConversationsPanel) SetConversations(convos []chat.Conversation) ConversationsPanel {
	p.conversations = convos
	if p.cursor >= len(convos) && len(convos) > 0 {
		p.cursor = len(convos) - 1
	}
	return p
}

func (p ConversationsPanel) SetFocused(focused bool) ConversationsPanel {
	p.focused = focused
	return p
}

func (p ConversationsPanel) SetSize(w, h int) ConversationsPanel {
	p.width = w
	p.height = h
	return p
}

func (p ConversationsPanel) Update(msg tea.Msg) (ConversationsPanel, tea.Cmd) {
	if !p.focused {
		return p, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.conversations)-1 {
				p.cursor++
			}
		case "enter":
			if p.cursor < len(p.conversations) {
				return p, func() tea.Msg {
					return ConversationSelectMsg{PeerMatchHash: p.conversations[p.cursor].PeerMatchHash}
				}
			}
		}
	}
	return p, nil
}

func (p ConversationsPanel) View() string {
	if len(p.conversations) == 0 {
		return theme.DimStyle.Render("  No conversations yet")
	}
	var b strings.Builder
	for i, c := range p.conversations {
		cursor := "  "
		nameStyle := theme.TextStyle
		if p.focused && i == p.cursor {
			cursor = theme.BrandStyle.Render("▸ ")
			nameStyle = theme.BoldStyle
		}
		hash := c.PeerMatchHash
		if len(hash) > 12 {
			hash = hash[:12] + "..."
		}
		msg := c.LastMessage
		if len(msg) > 20 {
			msg = msg[:17] + "..."
		}
		line := fmt.Sprintf("%s%s  %s", cursor, nameStyle.Render(hash), theme.DimStyle.Render("\""+msg+"\""))
		if c.UnreadCount > 0 {
			line += " " + theme.BrandStyle.Render(fmt.Sprintf("(%d)", c.UnreadCount))
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
