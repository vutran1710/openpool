package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type poolFocus int

const (
	focusList poolFocus = iota
	focusDetail
)

type poolItem struct {
	entry gh.PoolEntry
	stats gh.PoolStats
	joined bool
}

type poolsFetchedMsg struct {
	pools []poolItem
	err   error
}

type PoolsScreen struct {
	registry    string
	joinedPools map[string]bool
	pools       []poolItem
	cursor      int
	focus       poolFocus
	loading     bool
	loaded      bool
	err         error
	spinner     spinner.Model
	Width       int
	Height      int
}

func NewPoolsScreen(registry string, joinedPools []string) PoolsScreen {
	joined := make(map[string]bool)
	for _, p := range joinedPools {
		joined[p] = true
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)
	return PoolsScreen{
		registry:    registry,
		joinedPools: joined,
		spinner:     sp,
	}
}

func (s PoolsScreen) fetchPools() tea.Msg {
	reg, err := gh.CloneRegistry(s.registry)
	if err != nil {
		return poolsFetchedMsg{err: err}
	}
	entries, err := reg.ListPools()
	if err != nil {
		return poolsFetchedMsg{err: err}
	}

	var pools []poolItem
	for _, e := range entries {
		// Derive operator from repo owner if not set
		if e.Operator == "" {
			if parts := strings.SplitN(e.Repo, "/", 2); len(parts) == 2 {
				e.Operator = parts[0]
			}
		}

		// Get stats from cloned pool repo
		var stats gh.PoolStats
		poolURL := gitrepo.EnsureGitURL(e.Repo)
		if poolRepo, err := gitrepo.Clone(poolURL); err == nil {
			pool := gh.NewLocalPool(poolRepo)
			stats = pool.Stats()
		}

		pools = append(pools, poolItem{
			entry:  e,
			stats:  stats,
			joined: s.joinedPools[e.Name],
		})
	}
	return poolsFetchedMsg{pools: pools}
}

func (s PoolsScreen) Update(msg tea.Msg) (PoolsScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !s.loaded {
			return s, nil
		}
		switch msg.String() {
		case "tab", "right", "l":
			if s.focus == focusList && len(s.pools) > 0 {
				s.focus = focusDetail
			}
		case "shift+tab", "left", "h":
			if s.focus == focusDetail {
				s.focus = focusList
			}
		case "up", "k":
			if s.focus == focusList && s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.focus == focusList && s.cursor < len(s.pools)-1 {
				s.cursor++
			}
		}

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height

	case poolsFetchedMsg:
		s.loading = false
		s.loaded = true
		if msg.err != nil {
			s.err = msg.err
		} else {
			s.pools = msg.pools
		}
		return s, nil

	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
	}

	// Trigger fetch on first view
	if !s.loading && !s.loaded {
		s.loading = true
		return s, tea.Batch(s.fetchPools, s.spinner.Tick)
	}

	return s, nil
}

func (s PoolsScreen) View() string {
	if s.loading {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			fmt.Sprintf("%s Fetching pools from registry...", s.spinner.View()),
		)
	}

	if s.err != nil {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.RedStyle.Render("Error: " + s.err.Error()),
		)
	}

	if len(s.pools) == 0 {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.DimStyle.Render("No pools registered yet."),
		)
	}

	totalWidth := s.Width
	if totalWidth < 60 {
		totalWidth = 80
	}

	listWidth := totalWidth * 35 / 100
	if listWidth < 28 {
		listWidth = 28
	}
	detailWidth := totalWidth - listWidth - 6

	leftPanel := s.renderList(listWidth)
	rightPanel := s.renderDetail(detailWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
}

func (s PoolsScreen) renderList(width int) string {
	borderColor := theme.Border
	if s.focus == focusList {
		borderColor = theme.Violet
	}

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Padding(1, 1)

	title := theme.BoldStyle.Render("Pools") + "\n\n"
	maxDesc := width - 8

	var items string
	for i, p := range s.pools {
		items += components.RenderPoolListItem(
			p.entry.Name,
			p.entry.Description,
			p.joined,
			i == s.cursor,
			maxDesc,
		)
		items += "\n"
	}

	return listStyle.Render(title + items)
}

func (s PoolsScreen) renderDetail(width int) string {
	if len(s.pools) == 0 || s.cursor >= len(s.pools) {
		return ""
	}

	p := s.pools[s.cursor]

	opKey := p.entry.OperatorPubKey
	if len(opKey) > 16 {
		opKey = opKey[:16] + "..."
	}

	cardData := components.PoolCardData{
		Name:          p.entry.Name,
		Description:   p.entry.Description,
		Operator:      p.entry.Operator,
		OperatorKey:   opKey,
		Repo:          p.entry.Repo,
		RelayURL:      p.entry.RelayURL,
		CreatedAt:     p.entry.CreatedAt,
		Tags:          p.entry.Tags,
		Members:       p.stats.Members,
		Matches:       p.stats.Matches,
		Relationships: p.stats.Relationships,
		Joined:        p.joined,
	}

	return components.RenderPoolCard(cardData, width, s.focus == focusDetail)
}

func (s PoolsScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "tab", Desc: "switch panel"},
		{Key: "enter", Desc: "join"},
		{Key: "esc", Desc: "back"},
	}
}
