package screens

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type ChatMessage struct {
	Sender string
	Body   string
	Time   string
	IsMe   bool
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

	msgs := []ChatMessage{
		{Sender: targetID, Body: "hey, saw your profile", Time: "14:21", IsMe: false},
		{Sender: "you", Body: "hi! what are you working on?", Time: "14:22", IsMe: true},
		{Sender: targetID, Body: "building a CLI tool for dating 😄", Time: "14:22", IsMe: false},
	}

	s := ChatScreen{
		TargetID: targetID,
		Messages: msgs,
		Viewport: vp,
		Width:    width,
		Height:   height,
		Ready:    true,
	}
	s.Viewport.SetContent(s.renderMessages())
	s.Viewport.GotoBottom()
	return s
}

func (s ChatScreen) Update(msg tea.Msg) (ChatScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case components.SubmitMsg:
		if msg.IsCommand && msg.Value == "/exit" {
			return s, nil
		}
		s.Messages = append(s.Messages, ChatMessage{
			Sender: "you",
			Body:   msg.Value,
			Time:   time.Now().Format("15:04"),
			IsMe:   true,
		})
		s.Viewport.SetContent(s.renderMessages())
		s.Viewport.GotoBottom()
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
		{Key: "/profile", Desc: "view profile"},
	}
}
