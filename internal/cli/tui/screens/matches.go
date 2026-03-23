package screens

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type MatchItem struct {
	MatchHash string
	Greeting  string
	PubKey    ed25519.PublicKey
}

type MatchesFetchedMsg struct {
	Matches []MatchItem
	Err     error
}

// MatchChatMsg signals user wants to chat with a match.
type MatchChatMsg struct {
	MatchHash string
	PubKey    ed25519.PublicKey
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
		if pool == nil {
			return MatchesFetchedMsg{Err: fmt.Errorf("no active pool — join a pool first")}
		}
		if pool.MatchHash == "" {
			return MatchesFetchedMsg{Err: fmt.Errorf("registration pending — wait for pool to process your request")}
		}

		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		_, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
		if err != nil {
			return MatchesFetchedMsg{Err: fmt.Errorf("invalid operator pubkey")}
		}
		operatorPub := ed25519.PublicKey(operatorPubBytes)

		client := gh.NewPoolWithClient(gh.NewCLIOrHTTP(pool.Repo, ghToken))
		issues, err := client.Client().ListIssues(context.Background(), "closed", "interest")
		if err != nil {
			return MatchesFetchedMsg{Err: err}
		}

		var matches []MatchItem
		for _, iss := range issues {
			signedBlob, err := gh.FindOperatorReplyInIssue(context.Background(), client.Client(), iss.Number, "match", operatorPub)
			if err != nil {
				continue
			}
			plaintext, err := crypto.DecryptSignedBlob(signedBlob, priv)
			if err != nil {
				continue
			}
			var data struct {
				MatchedMatchHash string `json:"matched_match_hash"`
				Greeting         string `json:"greeting"`
				PubKey           string `json:"pubkey"`
			}
			if err := json.Unmarshal(plaintext, &data); err != nil || data.MatchedMatchHash == "" {
				continue
			}
			pubBytes, err := hex.DecodeString(data.PubKey)
			if err != nil || len(pubBytes) != ed25519.PublicKeySize {
				continue
			}
			matches = append(matches, MatchItem{
				MatchHash: data.MatchedMatchHash,
				Greeting:  data.Greeting,
				PubKey:    ed25519.PublicKey(pubBytes),
			})
		}

		return MatchesFetchedMsg{Matches: matches}
	}
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
				m := s.matches[s.cursor]
				return s, func() tea.Msg {
					return MatchChatMsg{MatchHash: m.MatchHash, PubKey: m.PubKey}
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
		label := m.MatchHash[:16]
		greeting := m.Greeting
		if greeting == "" {
			body += fmt.Sprintf("%s%s\n", cursor, nameStyle.Render(label))
		} else {
			if len(greeting) > 40 {
				greeting = greeting[:37] + "..."
			}
			body += fmt.Sprintf("%s%s  %s\n",
				cursor,
				nameStyle.Render(label),
				theme.DimStyle.Render("\""+greeting+"\""),
			)
		}
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

