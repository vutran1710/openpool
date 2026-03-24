package screens

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/cli/tui/components"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/explorer"
)

// DiscoverLikeMsg signals the user wants to like someone.
type DiscoverLikeMsg struct {
	TargetMatchHash string
}

// discoverLoadedMsg carries the initialized explorer.
type discoverLoadedMsg struct {
	explorer *explorer.Explorer
	buckets  []explorer.BucketInfo
	prefs    explorer.Preferences
	err      error
}

// discoverGrindResultMsg carries one unlocked profile.
type discoverGrindResultMsg struct {
	result *explorer.GrindResult
	err    error
}

type DiscoverScreen struct {
	explorer    *explorer.Explorer
	buckets     []explorer.BucketInfo
	bucketIdx   int
	permutation int
	current     *explorer.GrindResult
	history     []explorer.GrindResult
	historyIdx  int
	grinding    bool
	loading     bool
	done        bool // all profiles in bucket exhausted
	err         error
	spinner     spinner.Model
	Width       int
	Height      int
}

func NewDiscoverScreen() DiscoverScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)
	return DiscoverScreen{
		loading: true,
		spinner: sp,
	}
}

// LoadDiscoverCmd downloads index.db and initializes the explorer.
func LoadDiscoverCmd(poolName string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return discoverLoadedMsg{err: err}
		}

		pool := cfg.ActivePool()
		if pool == nil {
			return discoverLoadedMsg{err: fmt.Errorf("no active pool")}
		}
		if pool.MatchHash == "" {
			return discoverLoadedMsg{err: fmt.Errorf("registration pending — wait for pool to process")}
		}

		// Read local index.db (synced by pools screen)
		indexPath := filepath.Join(config.Dir(), "pools", poolName, "index.db")
		if _, statErr := os.Stat(indexPath); statErr != nil {
			return discoverLoadedMsg{err: fmt.Errorf("no index found — sync from Pools screen first")}
		}

		// Open explorer
		statePath := filepath.Join(config.Dir(), "pools", poolName, "explorer.db")
		exp, err := explorer.Open(indexPath, statePath)
		if err != nil {
			return discoverLoadedMsg{err: fmt.Errorf("opening explorer: %w", err)}
		}

		// Clear checkpoints — index may have been rebuilt with new permutations
		exp.OnIndexUpdated()

		buckets, err := exp.ListBuckets()
		if err != nil {
			exp.Close()
			return discoverLoadedMsg{err: fmt.Errorf("listing buckets: %w", err)}
		}

		if len(buckets) == 0 {
			exp.Close()
			return discoverLoadedMsg{err: fmt.Errorf("no profiles in pool yet")}
		}

		// Load preferences to auto-select bucket
		prefsPath := filepath.Join(config.Dir(), "pools", poolName, "preferences.yaml")
		prefs := explorer.LoadPreferences(prefsPath)

		return discoverLoadedMsg{explorer: exp, buckets: buckets, prefs: prefs}
	}
}

func (s DiscoverScreen) Update(msg tea.Msg) (DiscoverScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case discoverLoadedMsg:
		s.loading = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		s.explorer = msg.explorer
		// Rank buckets by preference match
		s.buckets = explorer.RankBuckets(msg.buckets, msg.prefs)
		s.bucketIdx = 0
		s.permutation = rand.Intn(s.buckets[0].Permutations)
		// Start grinding first profile
		s.grinding = true
		return s, tea.Batch(s.grindNext(), s.spinner.Tick)

	case discoverGrindResultMsg:
		s.grinding = false
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		if msg.result == nil {
			s.done = true
			return s, nil
		}
		s.current = msg.result
		s.history = append(s.history, *msg.result)
		s.historyIdx = len(s.history) - 1
		return s, nil

	case spinner.TickMsg:
		if s.grinding || s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}

	case tea.KeyMsg:
		if s.grinding || s.loading {
			return s, nil
		}

		switch msg.String() {
		case "l":
			// Like current profile
			if s.current != nil {
				target := s.current.Tag
				s.grinding = true
				return s, tea.Batch(
					func() tea.Msg { return DiscoverLikeMsg{TargetMatchHash: target} },
					s.markSeenAndGrindNext("like"),
				)
			}
		case "s", "right", "n":
			// Skip — grind next
			if s.current != nil && !s.done {
				s.grinding = true
				return s, tea.Batch(s.markSeenAndGrindNext("skip"), s.spinner.Tick)
			}
		case "left", "p":
			// Browse history backwards
			if s.historyIdx > 0 {
				s.historyIdx--
				s.current = &s.history[s.historyIdx]
			}
		}
	}
	return s, nil
}

func (s DiscoverScreen) grindNext() tea.Cmd {
	exp := s.explorer
	bid := s.buckets[s.bucketIdx].ID
	perm := s.permutation
	return func() tea.Msg {
		result, err := exp.GrindNext(bid, perm)
		return discoverGrindResultMsg{result: result, err: err}
	}
}

func (s DiscoverScreen) markSeenAndGrindNext(action string) tea.Cmd {
	exp := s.explorer
	tag := s.current.Tag
	bid := s.buckets[s.bucketIdx].ID
	perm := s.permutation
	return func() tea.Msg {
		exp.MarkSeen(tag, action)
		result, err := exp.GrindNext(bid, perm)
		return discoverGrindResultMsg{result: result, err: err}
	}
}

func (s DiscoverScreen) View() string {
	if s.loading {
		return components.ScreenLayout("Discover", "",
			"  "+s.spinner.View()+" Loading discovery index...")
	}

	if s.err != nil {
		return components.ScreenLayout("Discover", "",
			"  "+theme.RedStyle.Render(s.err.Error()))
	}

	if s.grinding {
		hint := components.DimHints("grinding")
		body := "  " + s.spinner.View() + " Unlocking next profile..."
		if len(s.history) > 0 {
			body += "\n" + theme.DimStyle.Render(fmt.Sprintf("  %d profiles unlocked so far", len(s.history)))
		}
		return components.ScreenLayout("Discover", hint, body)
	}

	if s.done {
		return components.ScreenLayout("Discover", components.DimHints("complete"),
			"  "+theme.DimStyle.Render("All profiles in this bucket explored. Try again later."))
	}

	if s.current == nil {
		return components.ScreenLayout("Discover", "", "  "+theme.DimStyle.Render("No profiles found."))
	}

	// Render current profile
	body := s.renderProfile()
	hint := components.DimHints(fmt.Sprintf("%d/%d", s.historyIdx+1, len(s.history)))
	return components.ScreenLayout("Discover", hint, body)
}

func (s DiscoverScreen) renderProfile() string {
	r := s.current
	width := s.Width - 8
	if width < 40 {
		width = 40
	}
	if width > 60 {
		width = 60
	}

	var lines []string

	// Name / ID
	name := r.Tag[:12] + "..."
	if n, ok := r.Profile["name"].(string); ok && n != "" {
		name = n
	}
	if n, ok := r.Profile["display_name"].(string); ok && n != "" {
		name = n
	}

	warmLabel := ""
	if r.Warm {
		warmLabel = theme.DimStyle.Render(" (known)")
	}
	lines = append(lines, theme.BoldStyle.Render(name)+warmLabel)

	// Age
	if age, ok := r.Profile["age"]; ok {
		lines = append(lines, theme.DimStyle.Render("Age: ")+theme.TextStyle.Render(fmt.Sprintf("%v", age)))
	}

	// Role
	if role, ok := r.Profile["role"].(string); ok {
		lines = append(lines, theme.DimStyle.Render("Role: ")+theme.TextStyle.Render(role))
	}

	// Interests
	if interests, ok := r.Profile["interests"]; ok {
		var tags []string
		switch v := interests.(type) {
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok {
					tags = append(tags, theme.AccentStyle.Render("#"+str))
				}
			}
		case []string:
			for _, str := range v {
				tags = append(tags, theme.AccentStyle.Render("#"+str))
			}
		}
		if len(tags) > 0 {
			lines = append(lines, strings.Join(tags, "  "))
		}
	}

	// About
	if about, ok := r.Profile["about"].(string); ok && about != "" {
		lines = append(lines, "")
		lines = append(lines, theme.DimStyle.Render("About"))
		aboutText := about
		if len(aboutText) > width-4 {
			aboutText = aboutText[:width-7] + "..."
		}
		lines = append(lines, theme.TextStyle.Render(aboutText))
	}

	// Attempts info
	lines = append(lines, "")
	lines = append(lines, theme.DimStyle.Render(fmt.Sprintf("  %d attempts to unlock", r.Attempts)))

	// Actions
	lines = append(lines, "")
	actions := theme.BrandStyle.Render("[l]") + " like   " +
		theme.DimStyle.Render("[→]") + " skip   " +
		theme.DimStyle.Render("[←]") + " prev"
	lines = append(lines, actions)

	content := strings.Join(lines, "\n")

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Violet).
		Width(width).
		Padding(1, 2)

	return lipgloss.NewStyle().Width(s.Width).Align(lipgloss.Center).Render(cardStyle.Render(content))
}

func (s DiscoverScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "l", Desc: "like"},
		{Key: "→/s", Desc: "skip"},
		{Key: "←", Desc: "prev"},
		{Key: "esc", Desc: "back"},
	}
}
