package screens

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

type ChatMessage struct {
	Sender string
	Body   string
	Time   string
	IsMe   bool
}

// ChatConnectedMsg signals relay connection established.
type ChatConnectedMsg struct {
	Client *relayclient.Client
	Err    error
}

// ChatIncomingMsg carries an incoming message from the relay.
type ChatIncomingMsg struct {
	Message protocol.Message
}

type ChatScreen struct {
	TargetID string
	Messages []ChatMessage
	Viewport viewport.Model
	Width    int
	Height   int
	Ready    bool
	client   *relayclient.Client
	connected bool
	err      error
}

func NewChatScreen(targetID string, width, height int) ChatScreen {
	vp := viewport.New(width, height-6)
	vp.Style = lipgloss.NewStyle().Padding(0, 2)

	return ChatScreen{
		TargetID: targetID,
		Viewport: vp,
		Width:    width,
		Height:   height,
	}
}

// ConnectChatCmd connects to the relay for chatting.
func ConnectChatCmd(targetBinHash string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return ChatConnectedMsg{Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil || pool.RelayURL == "" {
			return ChatConnectedMsg{Err: fmt.Errorf("no relay configured")}
		}

		pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return ChatConnectedMsg{Err: err}
		}

		client := relayclient.NewClient(relayclient.Config{
			RelayURL:  pool.RelayURL,
			PoolURL:   pool.Repo,
			BinHash:   pool.BinHash,
			MatchHash: pool.MatchHash,
			Pub:       pub,
			Priv:      priv,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.Connect(ctx); err != nil {
			return ChatConnectedMsg{Err: err}
		}

		return ChatConnectedMsg{Client: client}
	}
}

func (s ChatScreen) Update(msg tea.Msg) (ChatScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case ChatConnectedMsg:
		if msg.Err != nil {
			s.err = msg.Err
			return s, nil
		}
		s.client = msg.Client
		s.connected = true
		s.Ready = true

		// Start listening for incoming messages
		return s, s.listenForMessages()

	case ChatIncomingMsg:
		m := msg.Message
		sender := m.SourceHash
		if len(sender) > 12 {
			sender = sender[:12] + "..."
		}
		s.Messages = append(s.Messages, ChatMessage{
			Sender: sender,
			Body:   m.Body,
			Time:   time.Now().Format("15:04"),
			IsMe:   false,
		})
		s.Viewport.SetContent(s.renderMessages())
		s.Viewport.GotoBottom()
		// Keep listening
		return s, s.listenForMessages()

	case components.SubmitMsg:
		if msg.IsCommand && msg.Value == "/exit" {
			if s.client != nil {
				s.client.Close()
			}
			return s, nil
		}
		if s.connected && msg.Value != "" {
			err := s.client.SendMessage(s.TargetID, msg.Value)
			if err != nil {
				s.Messages = append(s.Messages, ChatMessage{
					Body: "(send failed: " + err.Error() + ")",
					Time: time.Now().Format("15:04"),
				})
			} else {
				s.Messages = append(s.Messages, ChatMessage{
					Sender: "you",
					Body:   msg.Value,
					Time:   time.Now().Format("15:04"),
					IsMe:   true,
				})
			}
			s.Viewport.SetContent(s.renderMessages())
			s.Viewport.GotoBottom()
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

func (s ChatScreen) listenForMessages() tea.Cmd {
	if s.client == nil {
		return nil
	}
	ch := make(chan protocol.Message, 1)
	s.client.OnMessage(func(m protocol.Message) {
		ch <- m
	})
	return func() tea.Msg {
		m := <-ch
		return ChatIncomingMsg{Message: m}
	}
}

func (s ChatScreen) View() string {
	if s.err != nil {
		body := theme.RedStyle.Render("Connection failed: " + s.err.Error())
		return components.ScreenLayout("Chat", components.DimHints(s.TargetID), body)
	}

	if !s.connected {
		return components.ScreenLayout("Chat", components.DimHints(s.TargetID),
			theme.DimStyle.Render("Connecting to relay..."))
	}

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
