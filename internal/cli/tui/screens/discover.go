package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// DiscoverMsg carries loaded suggestions to the screen.
type DiscoverMsg struct {
	Suggestions []suggestions.Suggestion
	Total       int
	Filtered    int
	Pack        *suggestions.Pack
	PackPath    string
	Err         error
}

type DiscoverScreen struct {
	suggestions []suggestions.Suggestion
	index       int
	total       int
	filtered    int
	pack        *suggestions.Pack
	packPath    string
	Loading     bool
	Empty       bool
	Width       int
	err         error
}

func NewDiscoverScreen() DiscoverScreen {
	return DiscoverScreen{Loading: true}
}

// LoadCmd loads suggestions for the active pool.
func LoadDiscoverCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return DiscoverMsg{Err: err}
		}

		var pool *config.PoolConfig
		for i := range cfg.Pools {
			if cfg.Pools[i].Name == poolName {
				pool = &cfg.Pools[i]
				break
			}
		}
		if pool == nil {
			return DiscoverMsg{Err: fmt.Errorf("pool not found: %s", poolName)}
		}
		if pool.MatchHash == "" {
			return DiscoverMsg{Err: fmt.Errorf("not registered in pool (no match_hash)")}
		}

		// Sync repo
		repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
		if err == nil {
			repo.Sync()
		}

		// Load pack
		packPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.pack")
		pack, err := suggestions.Load(packPath)
		if err != nil {
			return DiscoverMsg{Err: err}
		}

		// Sync new .rec files
		if repo != nil {
			indexDir := filepath.Join(repo.LocalDir, "index")
			added, _ := pack.SyncFromRecDir(indexDir)
			if added > 0 {
				pack.Save(packPath)
			}
		}

		if len(pack.Records) == 0 {
			return DiscoverMsg{Err: fmt.Errorf("no users in pool yet")}
		}

		me := pack.Find(pool.MatchHash)
		if me == nil {
			return DiscoverMsg{Err: fmt.Errorf("your vector not found — run: dating pool sync %s", poolName)}
		}

		// Load schema for filtered ranking
		var schema *gh.PoolSchema
		if repo != nil {
			manifest, mErr := loadManifestFromDir(repo.LocalDir)
			if mErr == nil && manifest.Schema != nil {
				schema = manifest.Schema
			}
		}

		ranked := suggestions.RankSuggestions(schema, *me, pack.Records, pack.Seen, 50)
		total := len(pack.Records) - 1
		filtered := total - len(ranked)

		return DiscoverMsg{
			Suggestions: ranked,
			Total:       total,
			Filtered:    filtered,
			Pack:        pack,
			PackPath:    packPath,
		}
	}
}

func loadManifestFromDir(dir string) (*gh.PoolManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "pool.json"))
	if err != nil {
		return nil, err
	}
	var m gh.PoolManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s DiscoverScreen) Update(msg tea.Msg) (DiscoverScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case DiscoverMsg:
		s.Loading = false
		if msg.Err != nil {
			s.err = msg.Err
			s.Empty = true
			return s, nil
		}
		s.suggestions = msg.Suggestions
		s.total = msg.Total
		s.filtered = msg.Filtered
		s.pack = msg.Pack
		s.packPath = msg.PackPath
		s.index = 0
		s.Empty = len(s.suggestions) == 0
		// Mark first suggestion as seen
		if cur := s.current(); cur != nil {
			s.markCurrentSeen()
		}
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "l":
			if s.current() != nil {
				name := s.current().MatchHash
				s.advance()
				return s, func() tea.Msg {
					return components.ToastMsg{
						Text:  fmt.Sprintf("Liked %s", name[:12]),
						Level: components.ToastSuccess,
					}
				}
			}
		case "s", "n", "right", "j":
			s.advance()
		case "left", "k":
			if s.index > 0 {
				s.index--
			}
		}
	}
	return s, nil
}

func (s *DiscoverScreen) advance() {
	if s.index < len(s.suggestions)-1 {
		s.index++
		s.markCurrentSeen()
	}
}

func (s *DiscoverScreen) markCurrentSeen() {
	if s.pack == nil {
		return
	}
	cur := s.current()
	if cur != nil {
		s.pack.MarkSeen(cur.MatchHash)
		s.pack.Save(s.packPath) // persist immediately
	}
}

func (s DiscoverScreen) current() *suggestions.Suggestion {
	if s.index >= 0 && s.index < len(s.suggestions) {
		return &s.suggestions[s.index]
	}
	return nil
}

func (s DiscoverScreen) View() string {
	if s.Loading {
		return "\n  " + theme.DimStyle.Render("Loading suggestions...") + "\n"
	}

	if s.err != nil {
		return "\n  " + theme.RedStyle.Render(s.err.Error()) + "\n"
	}

	if s.Empty || len(s.suggestions) == 0 {
		return "\n  " + theme.DimStyle.Render("No suggestions found. Try again later.") + "\n"
	}

	cur := s.current()
	if cur == nil {
		return "\n  " + theme.DimStyle.Render("End of suggestions.") + "\n"
	}

	// Header
	header := fmt.Sprintf("  %s  (%d/%d, %d filtered)",
		theme.BrandStyle.Render("Discover"),
		s.index+1, len(s.suggestions), s.filtered)

	// Card
	card := fmt.Sprintf(
		"\n  %s\n  Score: %s\n",
		theme.BoldStyle.Render(cur.MatchHash),
		theme.BrandStyle.Render(fmt.Sprintf("%.2f", cur.Score)),
	)

	// Actions
	actions := fmt.Sprintf(
		"\n  %s  %s  %s",
		theme.BrandStyle.Render("[l]")+theme.TextStyle.Render(" like"),
		theme.DimStyle.Render("[s/→]")+theme.TextStyle.Render(" next"),
		theme.DimStyle.Render("[←]")+theme.TextStyle.Render(" prev"),
	)

	return "\n" + header + card + actions + "\n"
}

func (s DiscoverScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "l", Desc: "like"},
		{Key: "s/→", Desc: "next"},
		{Key: "←", Desc: "prev"},
		{Key: "esc", Desc: "back"},
	}
}
