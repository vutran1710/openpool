package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/tui/components"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/crypto"
	gh "github.com/vutran1710/openpool/internal/github"
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
	if s.loading || s.acting {
		label := "Loading inbox..."
		if s.acting {
			label = "Processing..."
		}
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"),
			s.spinner.View()+" "+theme.DimStyle.Render(label))
	}

	if s.err != nil {
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"),
			theme.RedStyle.Render(s.err.Error()))
	}

	if !s.loaded {
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"),
			theme.DimStyle.Render("Press enter to load inbox"))
	}

	if len(s.likes) == 0 {
		return components.ScreenLayout("Inbox", components.DimHints("incoming interests"),
			theme.DimStyle.Render("No incoming interests yet"))
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

	lines := []string{
		cardHeader,
		"",
		likerLine,
		timeLine,
		"",
		msgLine,
	}

	return components.ScreenLayout("Inbox",
		components.DimHints(fmt.Sprintf("%d incoming", len(s.likes))),
		strings.Join(lines, "\n"))
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
