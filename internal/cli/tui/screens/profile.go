package screens

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type ProfileScreen struct {
	profile    *gh.DatingProfile
	mode       components.ProfileMode
	leftVP     viewport.Model
	rightVP    viewport.Model
	loaded     bool
	err        string
	Width      int
	Height     int
}

func NewProfileScreen() ProfileScreen {
	return ProfileScreen{
		mode:    components.ProfileFull,
		leftVP:  viewport.New(40, 20),
		rightVP: viewport.New(40, 20),
	}
}

func (s *ProfileScreen) SetProfile(p *gh.DatingProfile) {
	s.profile = p
	s.loaded = true
	s.updateContent()
}

func (s ProfileScreen) Update(msg tea.Msg) (ProfileScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			// Cycle modes: full → short → compact → full
			s.mode = (s.mode + 1) % 3
			s.updateContent()
		}

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		panelHeight := msg.Height - 6
		if panelHeight < 5 {
			panelHeight = 10
		}
		leftWidth := msg.Width * 50 / 100
		rightWidth := msg.Width - leftWidth - 4
		s.leftVP.Width = leftWidth
		s.leftVP.Height = panelHeight
		s.rightVP.Width = rightWidth
		s.rightVP.Height = panelHeight
		s.updateContent()
		return s, nil
	}

	// Route scroll to the focused viewport (right panel for showcase)
	var cmd tea.Cmd
	s.rightVP, cmd = s.rightVP.Update(msg)
	return s, cmd
}

func (s *ProfileScreen) updateContent() {
	if s.profile == nil {
		return
	}

	// Left: profile card
	leftContent := components.RenderProfile(*s.profile, s.leftVP.Width-2, s.mode)
	s.leftVP.SetContent(leftContent)
	s.leftVP.GotoTop()

	// Right: showcase
	if components.HasShowcase(*s.profile) {
		rightContent := components.RenderShowcase(*s.profile, s.rightVP.Width-2)
		s.rightVP.SetContent(rightContent)
		s.rightVP.GotoTop()
	} else {
		s.rightVP.SetContent(theme.DimStyle.Render("\n  No showcase available.\n\n  Add one by creating a README.md in\n  your GitHub identity repo."))
	}
}

func (s ProfileScreen) View() string {
	if !s.loaded {
		s.loadLocal()
	}

	if s.err != "" {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.RedStyle.Render("Error: " + s.err),
		)
	}

	if s.profile == nil {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.DimStyle.Render("No profile yet. Join a pool to create one."),
		)
	}

	// Mode label
	modeLabels := []string{"Full", "Short", "Compact"}
	modeLabel := modeLabels[s.mode]

	header := theme.BoldStyle.Render("Profile") +
		theme.DimStyle.Render("  mode: ") + theme.AccentStyle.Render(modeLabel) +
		theme.DimStyle.Render("  tab to switch")

	// Left panel: profile card
	leftBorder := theme.Border
	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorder).
		Render(s.leftVP.View())

	// Right panel: showcase
	rightLabel := theme.DimStyle.Render("  Showcase")
	rightPanel := rightLabel + "\n" + lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Render(s.rightVP.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	return header + "\n\n" + panels
}

func (s *ProfileScreen) loadLocal() {
	data, err := os.ReadFile(config.ProfilePath())
	if err != nil {
		s.loaded = true
		return
	}

	var p gh.DatingProfile
	if err := json.Unmarshal(data, &p); err != nil {
		s.err = "invalid profile.json"
		s.loaded = true
		return
	}

	s.profile = &p
	s.loaded = true
	s.updateContent()
}

func (s ProfileScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "tab", Desc: "switch mode"},
		{Key: "↑↓", Desc: "scroll showcase"},
		{Key: "esc", Desc: "back"},
	}
}

// MatchDetailScreen wraps a profile with page-flipping (for matches).
type MatchDetailScreen struct {
	profile  *gh.DatingProfile
	page     int // 0=profile, 1=showcase
	viewport viewport.Model
	Width    int
	Height   int
}

func NewMatchDetailScreen(p *gh.DatingProfile, width, height int) MatchDetailScreen {
	vp := viewport.New(width-4, height-6)
	s := MatchDetailScreen{
		profile:  p,
		viewport: vp,
		Width:    width,
		Height:   height,
	}
	s.updateContent()
	return s
}

func (s MatchDetailScreen) Update(msg tea.Msg) (MatchDetailScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if s.page > 0 {
				s.page = 0
				s.updateContent()
			}
		case "right", "l":
			if s.page == 0 && s.profile != nil && components.HasShowcase(*s.profile) {
				s.page = 1
				s.updateContent()
			}
		case "tab":
			if s.profile != nil && components.HasShowcase(*s.profile) {
				s.page = (s.page + 1) % 2
				s.updateContent()
			}
		}
	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		s.viewport.Width = msg.Width - 4
		s.viewport.Height = msg.Height - 6
		s.updateContent()
		return s, nil
	}

	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *MatchDetailScreen) updateContent() {
	if s.profile == nil {
		return
	}
	width := s.viewport.Width
	var content string
	switch s.page {
	case 0:
		content = components.RenderProfile(*s.profile, width, components.ProfileFull)
	case 1:
		content = components.RenderShowcase(*s.profile, width)
	}
	s.viewport.SetContent(content)
	s.viewport.GotoTop()
}

func (s MatchDetailScreen) View() string {
	if s.profile == nil {
		return theme.DimStyle.Render("No profile")
	}

	totalPages := 1
	if components.HasShowcase(*s.profile) {
		totalPages = 2
	}
	pageLabel := "Profile"
	if s.page == 1 {
		pageLabel = "Showcase"
	}

	header := theme.DimStyle.Render(fmt.Sprintf("Page %d/%d", s.page+1, totalPages)) +
		"  " + theme.BoldStyle.Render(pageLabel)
	if totalPages > 1 {
		header += theme.DimStyle.Render("  ← → flip")
	}

	return header + "\n\n" + s.viewport.View()
}

func (s MatchDetailScreen) HelpBindings() []components.KeyBind {
	bindings := []components.KeyBind{
		{Key: "↑↓", Desc: "scroll"},
		{Key: "esc", Desc: "back"},
	}
	if s.profile != nil && components.HasShowcase(*s.profile) {
		bindings = append([]components.KeyBind{{Key: "← →", Desc: "flip page"}}, bindings...)
	}
	return bindings
}
