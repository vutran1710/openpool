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
	profile  *gh.DatingProfile
	page     int // 0=profile, 1=showcase
	viewport viewport.Model
	loaded   bool
	err      string
	Width    int
	Height   int
}

func NewProfileScreen() ProfileScreen {
	vp := viewport.New(60, 20)
	return ProfileScreen{viewport: vp}
}

// SetProfile sets the profile to display (from local file or relay).
func (s *ProfileScreen) SetProfile(p *gh.DatingProfile) {
	s.profile = p
	s.loaded = true
	s.updateContent()
}

func (s ProfileScreen) Update(msg tea.Msg) (ProfileScreen, tea.Cmd) {
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
		s.viewport.Height = msg.Height - 8
		s.updateContent()
		return s, nil
	}

	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *ProfileScreen) updateContent() {
	if s.profile == nil {
		return
	}

	width := s.Width
	if width < 40 {
		width = 60
	}

	var content string
	switch s.page {
	case 0:
		content = components.RenderProfile(*s.profile, width-4, components.ProfileFull)
	case 1:
		content = components.RenderShowcase(*s.profile, width-4)
	}

	s.viewport.SetContent(content)
	s.viewport.GotoTop()
}

func (s ProfileScreen) View() string {
	if !s.loaded {
		// Try loading from local profile
		s.loadLocal()
	}

	if s.err != "" {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.RedStyle.Render("Error: "+s.err),
		)
	}

	if s.profile == nil {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.DimStyle.Render("No profile yet. Join a pool to create one."),
		)
	}

	// Page indicator
	totalPages := 1
	if components.HasShowcase(*s.profile) {
		totalPages = 2
	}

	pageLabel := "Profile"
	if s.page == 1 {
		pageLabel = "Showcase"
	}

	header := theme.DimStyle.Render(fmt.Sprintf("  Page %d/%d", s.page+1, totalPages)) +
		"  " + theme.BoldStyle.Render(pageLabel)

	if totalPages > 1 {
		header += theme.DimStyle.Render("  ← → flip")
	}

	return header + "\n\n" + s.viewport.View()
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
	bindings := []components.KeyBind{
		{Key: "↑↓", Desc: "scroll"},
		{Key: "esc", Desc: "back"},
	}
	if s.profile != nil && components.HasShowcase(*s.profile) {
		bindings = append([]components.KeyBind{{Key: "← →", Desc: "flip page"}}, bindings...)
	}
	return bindings
}
