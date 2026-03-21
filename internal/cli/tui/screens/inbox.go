package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// InboxFetchMsg is sent to trigger loading inbox data.
type InboxFetchMsg struct{}

// InboxFetchedResult carries the loaded inbox items.
type InboxFetchedResult struct {
	Likes []InboxLikeItem
	Err   error
}

// InboxActionResult is returned after accept/reject completes.
type InboxActionResult struct {
	Accepted bool
	Err      error
}

// InboxLikeItem is a parsed interest from an issue.
type InboxLikeItem struct {
	Issue     gh.Issue
	LikerHash string
	Message   string
}

// InboxAcceptMsg is emitted when the user accepts a like.
type InboxAcceptMsg struct {
	IssueNumber int
}

// InboxRejectMsg is emitted when the user rejects a like.
type InboxRejectMsg struct {
	IssueNumber int
}

type InboxScreen struct {
	likes   []InboxLikeItem
	cursor  int
	loading bool
	loaded  bool
	acting  bool // true while accept/reject in progress
	err     error
	spinner spinner.Model
	Width   int
	Height  int
}

func NewInboxScreen() InboxScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)
	return InboxScreen{spinner: sp}
}

func (s InboxScreen) Update(msg tea.Msg) (InboxScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case InboxFetchMsg:
		s.loading = true
		s.loaded = false
		return s, tea.Batch(s.spinner.Tick, nil) // actual fetch triggered by app.go

	case InboxFetchedResult:
		s.loading = false
		s.loaded = true
		s.likes = msg.Likes
		s.err = msg.Err
		s.cursor = 0
		return s, nil

	case InboxActionResult:
		s.acting = false
		if msg.Err != nil {
			s.err = msg.Err
			return s, nil
		}
		// Remove current item and advance
		if len(s.likes) > 0 {
			s.likes = append(s.likes[:s.cursor], s.likes[s.cursor+1:]...)
			if s.cursor >= len(s.likes) && s.cursor > 0 {
				s.cursor--
			}
		}
		return s, func() tea.Msg {
			if msg.Accepted {
				return components.ToastMsg{Text: "It's a match!", Level: components.ToastSuccess}
			}
			return components.ToastMsg{Text: "Like rejected", Level: components.ToastInfo}
		}

	case tea.KeyMsg:
		if s.loading || s.acting {
			return s, nil
		}
		if len(s.likes) == 0 {
			return s, nil
		}
		switch msg.String() {
		case "y":
			s.acting = true
			iss := s.likes[s.cursor].Issue
			return s, func() tea.Msg {
				return InboxAcceptMsg{IssueNumber: iss.Number}
			}
		case "n":
			s.acting = true
			iss := s.likes[s.cursor].Issue
			return s, func() tea.Msg {
				return InboxRejectMsg{IssueNumber: iss.Number}
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s InboxScreen) View() string {
	panelWidth := 54
	if s.Width > 0 && s.Width < panelWidth+10 {
		panelWidth = s.Width - 10
	}
	if panelWidth < 30 {
		panelWidth = 30
	}

	var content string

	if s.loading || s.acting {
		label := "Loading inbox..."
		if s.acting {
			label = "Processing..."
		}
		content = s.spinner.View() + " " + theme.DimStyle.Render(label)
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"), centerPanel(content, panelWidth, s.Width, s.Height))
	}

	if s.err != nil {
		content = theme.RedStyle.Render("Error: " + s.err.Error())
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"), centerPanel(content, panelWidth, s.Width, s.Height))
	}

	if !s.loaded {
		content = theme.DimStyle.Render("Press enter to load inbox")
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"), centerPanel(content, panelWidth, s.Width, s.Height))
	}

	if len(s.likes) == 0 {
		content = theme.DimStyle.Render("No incoming interests yet")
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"), centerPanel(content, panelWidth, s.Width, s.Height))
	}

	// Current like
	item := s.likes[s.cursor]
	counter := theme.DimStyle.Render(fmt.Sprintf("(%d/%d)", s.cursor+1, len(s.likes)))

	// Card header
	cardHeader := lipgloss.NewStyle().Foreground(theme.Pink).Bold(true).Render("♥ Someone likes you") +
		"  " + counter

	// Liker info
	likerLine := theme.AccentStyle.Render(crypto.ShortHash(item.LikerHash))

	// Time ago
	ago := timeAgo(item.Issue.CreatedAt)
	timeLine := theme.DimStyle.Render(ago)

	// Message preview
	msgLine := theme.DimStyle.Render("(encrypted message)")
	if item.Message != "" && len(item.Message) < 200 {
		msgLine = theme.TextStyle.Render(`"` + item.Message + `"`)
	}

	// Actions
	actions := theme.DimStyle.Render("y") + " accept  " +
		theme.DimStyle.Render("n") + " reject  " +
		theme.DimStyle.Render("esc") + " back"

	lines := []string{
		cardHeader,
		"",
		likerLine,
		timeLine,
		"",
		msgLine,
		"",
		actions,
	}

	content = strings.Join(lines, "\n")
	return components.ScreenLayout("Inbox", components.DimHints("incoming interests"), centerPanel(content, panelWidth, s.Width, s.Height))
}

func centerPanel(content string, panelWidth, screenWidth, screenHeight int) string {
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		Width(panelWidth).
		Render(content)

	return lipgloss.Place(
		screenWidth, screenHeight,
		lipgloss.Center, lipgloss.Center,
		panel,
	)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func (s InboxScreen) HelpBindings() []components.KeyBind {
	if s.loading || s.acting {
		return []components.KeyBind{
			{Key: "esc", Desc: "back"},
		}
	}
	if len(s.likes) > 0 {
		return []components.KeyBind{
			{Key: "y", Desc: "accept"},
			{Key: "n", Desc: "reject"},
			{Key: "esc", Desc: "back"},
		}
	}
	return []components.KeyBind{
		{Key: "esc", Desc: "back"},
	}
}
