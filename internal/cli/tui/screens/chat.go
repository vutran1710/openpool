package screens

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/chat"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type ChatMessage struct {
	Sender string
	Body   string
	Time   string
	IsMe   bool
}

// ChatSendMsg is emitted when the user sends a message from the chat screen.
type ChatSendMsg struct {
	PeerMatchHash string
	Text          string
}

// ChatUnmatchMsg is emitted when the user wants to unmatch from the chat screen.
type ChatUnmatchMsg struct {
	TargetMatchHash string
}

type ChatScreen struct {
	TargetID string
	Messages []ChatMessage
	Viewport viewport.Model
	Width    int
	Height   int
	Ready    bool
}

func NewChatScreen(targetID string, width, height int) ChatScreen {
	vp := viewport.New(width, height-6)
	vp.Style = lipgloss.NewStyle().Padding(0, 2)

	return ChatScreen{
		TargetID: targetID,
		Viewport: vp,
		Width:    width,
		Height:   height,
		Ready:    true,
	}
}

func (s *ChatScreen) LoadHistory(msgs []chat.Message) {
	s.Messages = nil
	for _, m := range msgs {
		sender := "you"
		isMe := m.IsMe
		if !isMe {
			sender = s.TargetID
			if len(sender) > 12 {
				sender = sender[:12] + "..."
			}
		}
		s.Messages = append(s.Messages, ChatMessage{
			Sender: sender,
			Body:   m.Body,
			Time:   m.CreatedAt.Format("15:04"),
			IsMe:   isMe,
		})
	}
	s.Viewport.SetContent(s.renderMessages())
	s.Viewport.GotoBottom()
}

func (s *ChatScreen) AppendMessage(body string, isMe bool) {
	sender := "you"
	if !isMe {
		sender = s.TargetID
		if len(sender) > 12 {
			sender = sender[:12] + "..."
		}
	}
	s.Messages = append(s.Messages, ChatMessage{
		Sender: sender,
		Body:   body,
		Time:   time.Now().Format("15:04"),
		IsMe:   isMe,
	})
	s.Viewport.SetContent(s.renderMessages())
	s.Viewport.GotoBottom()
}

func (s ChatScreen) Update(msg tea.Msg) (ChatScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case components.SubmitMsg:
		if msg.IsCommand && msg.Value == "/exit" {
			return s, nil
		}
		if msg.Value != "" {
			return s, func() tea.Msg {
				return ChatSendMsg{PeerMatchHash: s.TargetID, Text: msg.Value}
			}
		}

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		s.Viewport.Width = msg.Width
		s.Viewport.Height = msg.Height - 6
	}

	var cmd tea.Cmd
	s.Viewport, cmd = s.Viewport.Update(msg)
	return s, cmd
}

func (s ChatScreen) View() string {
	separator := lipgloss.NewStyle().
		Width(s.Width).
		Foreground(theme.Border).
		Render(components.Repeat("─", s.Width))

	body := separator + "\n" + s.Viewport.View()
	return components.ScreenLayout("Chat", components.DimHints(s.TargetID), body)
}

func (s ChatScreen) renderMessages() string {
	if len(s.Messages) == 0 {
		return theme.DimStyle.Render("  No messages yet. Say hello!")
	}
	out := ""
	for _, m := range s.Messages {
		ts := theme.DimStyle.Render("[" + m.Time + "]")
		var sender string
		if m.IsMe {
			sender = theme.BrandStyle.Render("you")
		} else {
			sender = theme.AccentStyle.Render(m.Sender)
		}
		out += fmt.Sprintf("%s %s: %s\n", ts, sender, m.Body)
	}
	return out
}

func (s ChatScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "enter", Desc: "send"},
		{Key: "/exit", Desc: "leave"},
	}
}
