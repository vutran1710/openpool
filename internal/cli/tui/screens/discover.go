package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Schema      *gh.PoolSchema
	Err         error
}

type DiscoverScreen struct {
	suggestions []suggestions.Suggestion
	index       int
	total       int
	filtered    int
	pack        *suggestions.Pack
	packPath    string
	schema      *gh.PoolSchema
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
			Schema:      schema,
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
		s.schema = msg.Schema
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

	rec := s.pack.Find(cur.MatchHash)

	// Build card content
	width := s.Width - 6
	if width < 40 {
		width = 40
	}
	if width > 60 {
		width = 60
	}

	var lines []string

	// Row 1: Name + age + match %
	name := cur.MatchHash[:12] + "..."
	if rec != nil && rec.DisplayName != "" {
		name = rec.DisplayName
	}
	age := ""
	if rec != nil {
		if a, ok := rec.Filters.Fields["age"]; ok && a > 0 {
			age = fmt.Sprintf(", %d", a)
		}
	}
	matchPct := fmt.Sprintf("%.0f%%", cur.Score*100)
	matchColor := theme.DimStyle
	if cur.Score > 0.7 {
		matchColor = theme.GreenStyle
	} else if cur.Score > 0.4 {
		matchColor = theme.AmberStyle
	}
	heartScore := matchColor.Render("♥ " + matchPct)
	nameAge := theme.BoldStyle.Render(name+age)
	gap := width - lipgloss.Width(nameAge) - lipgloss.Width(heartScore)
	if gap < 1 {
		gap = 1
	}
	lines = append(lines, nameAge+spaces(gap)+heartScore)

	// Row 2: Gender + intent tags
	if rec != nil && s.schema != nil {
		labels := gh.DecodeFilters(s.schema, rec.Filters)
		var tags []string
		if g, ok := labels["gender"]; ok {
			tags = append(tags, g)
		}
		if i, ok := labels["intent"]; ok {
			tags = append(tags, i)
		}
		if gt, ok := labels["gender_target"]; ok {
			tags = append(tags, "→ "+gt)
		}
		if len(tags) > 0 {
			tagLine := ""
			for i, tag := range tags {
				if i > 0 {
					tagLine += theme.DimStyle.Render(" · ")
				}
				tagLine += theme.AccentStyle.Render(tag)
			}
			lines = append(lines, tagLine)
		}
	}

	// Row 3: Bio
	if rec != nil && rec.Bio != "" {
		bio := rec.Bio
		if len(bio) > width-4 {
			bio = bio[:width-7] + "..."
		}
		lines = append(lines, "")
		lines = append(lines, theme.DimStyle.Render("\""+bio+"\""))
	}

	// Row 4: Interests as tags
	if rec != nil && s.schema != nil {
		labels := gh.DecodeFilters(s.schema, rec.Filters)
		if interests, ok := labels["interests"]; ok && interests != "" {
			lines = append(lines, "")
			parts := splitComma(interests)
			tagLine := ""
			for i, p := range parts {
				if i > 0 {
					tagLine += "  "
				}
				tagLine += theme.BrandStyle.Render("♦") + " " + theme.TextStyle.Render(p)
			}
			lines = append(lines, tagLine)
		}
	}

	// Actions row
	lines = append(lines, "")
	actions := theme.BrandStyle.Render("[l]") + theme.TextStyle.Render(" ♥ like") + "   " +
		theme.DimStyle.Render("[→]") + theme.TextStyle.Render(" next") + "   " +
		theme.DimStyle.Render("[←]") + theme.TextStyle.Render(" prev")
	lines = append(lines, actions)

	// Render card with border
	content := ""
	for i, line := range lines {
		content += line
		if i < len(lines)-1 {
			content += "\n"
		}
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Violet).
		Width(width).
		Padding(1, 2)

	card := cardStyle.Render(content)

	// Footer below card
	footer := theme.DimStyle.Render(fmt.Sprintf(
		"%d/%d · %d filtered",
		s.index+1, len(s.suggestions), s.filtered))

	// Title header — matches Profile screen style
	header := theme.BoldStyle.Render("Discover") + "   " +
		theme.DimStyle.Render("browse profiles") + "  " +
		theme.DimStyle.Render("·") + "  " +
		theme.DimStyle.Render("l to like")

	// Center card + footer
	centered := lipgloss.NewStyle().Width(s.Width).Align(lipgloss.Center)

	return "\n" + header + "\n\n" + centered.Render(card) + "\n" + centered.Render(footer) + "\n"
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	s := ""
	for i := 0; i < n; i++ {
		s += " "
	}
	return s
}

func splitComma(s string) []string {
	var parts []string
	for _, p := range strings.Split(s, ", ") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func (s DiscoverScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "l", Desc: "like"},
		{Key: "s/→", Desc: "next"},
		{Key: "←", Desc: "prev"},
		{Key: "esc", Desc: "back"},
	}
}
