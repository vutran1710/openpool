package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/cli/tui/components"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	dbg "github.com/vutran1710/openpool/internal/debug"
	"github.com/vutran1710/openpool/internal/gitrepo"
	gh "github.com/vutran1710/openpool/internal/github"
)

type poolFocus int

const (
	focusList poolFocus = iota
	focusDetail
)

type poolItem struct {
	entry        gh.PoolEntry
	stats        gh.PoolStats
	logo         string
	status       string // "active", "pending", "rejected", "" (not joined)
	pendingIssue int
}

// PoolsInitMsg triggers the initial pool fetch.
type PoolsInitMsg struct{}

type poolsFetchedMsg struct {
	pools []poolItem
	err   error
}

// PoolJoinMsg is emitted when the user presses enter on a pool.
type PoolJoinMsg struct {
	Name   string
	Status string // "active", "pending", "rejected", ""
}

type PoolsScreen struct {
	registry     string
	activePool   string            // currently active pool name
	poolStatus   map[string]string // pool name → status
	poolIssues   map[string]int    // pool name → pending issue number
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

func (s *PoolsScreen) SetActivePool(name string) {
	s.activePool = name
}

func NewPoolsScreen(registry string, poolStatuses map[string]string, poolIssues ...map[string]int) PoolsScreen {
	if poolStatuses == nil {
		poolStatuses = make(map[string]string)
	}
	issues := make(map[string]int)
	if len(poolIssues) > 0 && poolIssues[0] != nil {
		issues = poolIssues[0]
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)
	return PoolsScreen{
		registry:   registry,
		poolStatus: poolStatuses,
		poolIssues: issues,
		spinner:    sp,
	}
}

func (s PoolsScreen) fetchPools() tea.Msg {
	done := dbg.Timer("PoolsScreen.fetchPools")
	defer done()

	// Clone registry (instant if already cloned)
	regRepo, err := gitrepo.CloneRegistry(gitrepo.EnsureGitURL(s.registry))
	if err != nil {
		return poolsFetchedMsg{err: err}
	}

	// Only sync registry if it has updates
	regRepo.Sync()

	reg := gh.NewLocalRegistry(regRepo)
	entries, err := reg.ListPools()
	if err != nil {
		return poolsFetchedMsg{err: err}
	}

	var pools []poolItem
	for _, e := range entries {
		if e.Operator == "" {
			if parts := strings.SplitN(e.Repo, "/", 2); len(parts) == 2 {
				e.Operator = parts[0]
			}
		}

		// Fetch stats: use local clone for joined pools, API for unjoined
		var stats gh.PoolStats
		var poolRepo *gitrepo.Repo
		isJoined := s.poolStatus[e.Name] == "active" || s.poolStatus[e.Name] == "pending"
		if isJoined {
			poolURL := gitrepo.EnsureGitURL(e.Repo)
			if repo, err := gitrepo.Clone(poolURL); err == nil {
				repo.Sync()
				poolRepo = repo
				pool := gh.NewLocalPool(repo)
				stats = pool.Stats()
			}
			// Download chain-encrypted index.db for discovery
			indexDir := filepath.Join(config.Dir(), "pools", e.Name)
			os.MkdirAll(indexDir, 0700)
			indexPath := filepath.Join(indexDir, "index.db")
			gh.DownloadReleaseAsset(e.Repo, "index-latest", "index.db", indexPath)
		} else {
			// Unjoined: use API (no token needed for public repos)
			pool := gh.NewPoolWithClient(gh.NewCLIOrHTTP(e.Repo, ""))
			stats = pool.Stats()
			// Still clone for logo
			poolURL := gitrepo.EnsureGitURL(e.Repo)
			if repo, err := gitrepo.Clone(poolURL); err == nil {
				poolRepo = repo
			}
		}

		// Get logo from pool repo (logo.txt in pool root)
		logo := components.PoolLogoFromRepo(poolRepo)

		pools = append(pools, poolItem{
			entry:  e,
			stats:  stats,
			logo:   logo,
			status:       s.poolStatus[e.Name],
			pendingIssue: s.poolIssues[e.Name],
		})
	}
	return poolsFetchedMsg{pools: pools}
}

// GetPools returns the pool entries for external use (e.g. join screen).
func (s PoolsScreen) GetPools() []gh.PoolEntry {
	var entries []gh.PoolEntry
	for _, p := range s.pools {
		entries = append(entries, p.entry)
	}
	return entries
}

func (s PoolsScreen) IsLoaded() bool {
	return s.loaded || s.loading
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
		case "enter":
			if s.cursor < len(s.pools) {
				p := s.pools[s.cursor]
				return s, func() tea.Msg {
					return PoolJoinMsg{Name: p.entry.Name, Status: p.status}
				}
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
			return s, nil
		}
		s.pools = msg.pools
		return s, nil

	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}

	case PoolsInitMsg:
		// Explicit init trigger
	}

	// Trigger fetch on first view or init
	if !s.loading && !s.loaded {
		s.loading = true
		return s, tea.Batch(s.fetchPools, s.spinner.Tick)
	}

	return s, nil
}

func (s PoolsScreen) View() string {
	if s.loading {
		return components.ScreenLayout("Pools",
			components.DimHints("browse and join"),
			fmt.Sprintf("%s Fetching pools from registry...", s.spinner.View()))
	}

	if s.err != nil {
		return components.ScreenLayout("Pools",
			components.DimHints("browse and join"),
			theme.RedStyle.Render("Error: "+s.err.Error()))
	}

	if len(s.pools) == 0 {
		return components.ScreenLayout("Pools",
			components.DimHints("browse and join"),
			theme.DimStyle.Render("No pools registered yet."))
	}

	totalWidth := s.Width
	if totalWidth < 40 {
		totalWidth = 80
	}

	// 50/50 split, minus borders (2 each) and gap (2)
	panelWidth := (totalWidth - 6) / 2
	listWidth := panelWidth
	detailWidth := panelWidth

	leftPanel := s.renderList(listWidth)
	rightPanel := s.renderDetail(detailWidth)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	return components.ScreenLayout("Pools",
		components.DimHints("browse and join"),
		body)
}

func (s PoolsScreen) renderList(width int) string {
	listStyle := lipgloss.NewStyle().
		Width(width).
		Padding(1, 1)

	title := ""
	maxDesc := width - 8

	var items string
	for i, p := range s.pools {
		items += components.RenderPoolListItem(
			p.entry.Name,
			p.entry.Description,
			p.status,
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
		About:         p.entry.About,
		Operator:      p.entry.Operator,
		OperatorKey:   opKey,
		Repo:          p.entry.Repo,
		RelayURL:      p.entry.RelayURL,
		Website:       p.entry.Website,
		CreatedAt:     p.entry.CreatedAt,
		Tags:          p.entry.Tags,
		Members:       p.stats.Members,
		Matches:       p.stats.Matches,
		Relationships: p.stats.Relationships,
		Status:        p.status,
		IsCurrentPool: p.entry.Name == s.activePool,
		PendingIssue:  p.pendingIssue,
		Logo:          p.logo,
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
