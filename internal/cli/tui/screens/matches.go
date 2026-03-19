package screens

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type MatchItem struct {
	BinHash  string
	Greeting string
}

type MatchesFetchedMsg struct {
	Matches []MatchItem
	Err     error
}

// MatchChatMsg signals user wants to chat with a match.
type MatchChatMsg struct {
	BinHash string
}

type MatchesScreen struct {
	matches []MatchItem
	cursor  int
	Loading bool
	Empty   bool
	err     error
}

func NewMatchesScreen() MatchesScreen {
	return MatchesScreen{Empty: true}
}

func LoadMatchesCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil || pool.MatchHash == "" {
			return MatchesFetchedMsg{Err: fmt.Errorf("not registered")}
		}

		ghToken, err := resolveGHToken()
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		_, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		client := gh.NewPool(pool.Repo, ghToken)
		prs, err := client.Client().ListPullRequests(context.Background(), "closed")
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		var matches []MatchItem
		for _, pr := range prs {
			hasLabel := false
			for _, l := range pr.Labels {
				if l.Name == "interest" {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				continue
			}

			comments, err := client.Client().ListIssueComments(context.Background(), pr.Number)
			if err != nil {
				continue
			}
			for _, c := range comments {
				if c.User.Login != "github-actions[bot]" {
					continue
				}
				m, err := decryptMatchNotification(c.Body, priv)
				if err != nil {
					continue
				}
				matches = append(matches, *m)
			}
		}

		return MatchesFetchedMsg{Matches: matches}
	}
}

func decryptMatchNotification(body string, priv ed25519.PrivateKey) (*MatchItem, error) {
	blobBytes, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, err
	}
	plaintext, err := crypto.Decrypt(priv, blobBytes)
	if err != nil {
		return nil, err
	}
	var data struct {
		MatchedBinHash string `json:"matched_bin_hash"`
		Greeting       string `json:"greeting"`
	}
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, err
	}
	if data.MatchedBinHash == "" {
		return nil, fmt.Errorf("missing bin_hash")
	}
	return &MatchItem{BinHash: data.MatchedBinHash, Greeting: data.Greeting}, nil
}

func (s MatchesScreen) Update(msg tea.Msg) (MatchesScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case MatchesFetchedMsg:
		s.Loading = false
		if msg.Err != nil {
			s.err = msg.Err
			return s, nil
		}
		s.matches = msg.Matches
		s.Empty = len(s.matches) == 0
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.matches)-1 {
				s.cursor++
			}
		case "enter":
			if s.cursor < len(s.matches) {
				return s, func() tea.Msg {
					return MatchChatMsg{BinHash: s.matches[s.cursor].BinHash}
				}
			}
		}
	}
	return s, nil
}

func (s MatchesScreen) View() string {
	if s.Loading {
		return components.ScreenLayout("Matches", components.DimHints("loading..."), "")
	}
	if s.err != nil {
		return components.ScreenLayout("Matches", "", theme.RedStyle.Render(s.err.Error()))
	}
	if s.Empty {
		return components.ScreenLayout("Matches",
			components.DimHints("your connections"),
			theme.DimStyle.Render("No matches yet. Keep discovering!"))
	}

	var body string
	for i, m := range s.matches {
		cursor := "  "
		nameStyle := theme.TextStyle
		if i == s.cursor {
			cursor = theme.BrandStyle.Render("▸ ")
			nameStyle = theme.BoldStyle
		}
		greeting := m.Greeting
		if len(greeting) > 40 {
			greeting = greeting[:37] + "..."
		}
		body += fmt.Sprintf("%s%s  %s\n",
			cursor,
			nameStyle.Render(m.BinHash[:16]),
			theme.DimStyle.Render("\""+greeting+"\""),
		)
	}

	body += "\n" + theme.DimStyle.Render("  enter to chat  ·  ↑↓ navigate")

	return components.ScreenLayout("Matches",
		components.DimHints(fmt.Sprintf("%d connections", len(s.matches))),
		body)
}

func (s MatchesScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "open chat"},
		{Key: "esc", Desc: "back"},
	}
}

func resolveGHToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth required")
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token")
	}
	return token, nil
}
