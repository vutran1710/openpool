package screens

import (
	"encoding/json"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	dbg "github.com/vutran1710/dating-dev/internal/debug"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type profileLoadedMsg struct {
	profile *gh.DatingProfile
	err     error
}

type ProfileScreen struct {
	profile *gh.DatingProfile
	mode    components.ProfileMode
	vp      viewport.Model
	loaded  bool
	err     string
	Width   int
	Height  int

	// Cache
	cachedNormal  string
	cachedCompact string
	cacheWidth    int
}

func NewProfileScreen() ProfileScreen {
	done := dbg.Timer("ProfileScreen.New")
	defer done()

	s := ProfileScreen{
		mode: components.ProfileNormal,
		vp:   viewport.New(60, 20),
	}
	s.loadLocal()
	return s
}

func (s *ProfileScreen) SetProfile(p *gh.DatingProfile) {
	s.profile = p
	s.loaded = true
	s.updateContent()
}

func (s ProfileScreen) IsLoaded() bool {
	return s.loaded
}

func (s ProfileScreen) LoadCmd() tea.Msg {
	return s.loadLocalCmd()
}

func (s ProfileScreen) Init() tea.Cmd {
	return s.LoadCmd
}

func (s ProfileScreen) Update(msg tea.Msg) (ProfileScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if s.mode == components.ProfileNormal {
				s.mode = components.ProfileCompact
			} else {
				s.mode = components.ProfileNormal
			}
			dbg.Log("ProfileScreen.mode → %d", s.mode)
			s.swapContent()
			return s, nil
		}

	case profileLoadedMsg:
		s.loaded = true
		if msg.err != nil {
			s.err = msg.err.Error()
		} else if msg.profile != nil {
			s.profile = msg.profile
			s.updateContent()
		}
		return s, nil

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		s.vp.Width = msg.Width - 4
		s.vp.Height = msg.Height - 12 // header(7) + profile header(2) + help/input(3)
		s.updateContent()
		return s, nil
	}

	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s *ProfileScreen) buildCache() {
	if s.profile == nil {
		return
	}
	done := dbg.Timer("ProfileScreen.buildCache")
	defer done()

	w := s.vp.Width
	s.cachedNormal = components.RenderProfile(*s.profile, w-4, components.ProfileNormal)
	s.cachedCompact = components.RenderProfile(*s.profile, w-4, components.ProfileCompact)
	s.cacheWidth = w
}

func (s *ProfileScreen) updateContent() {
	if s.profile == nil {
		return
	}
	w := s.vp.Width
	if w < 20 {
		return
	}
	if s.cacheWidth != w {
		s.buildCache()
	}
	s.swapContent()
}

func (s *ProfileScreen) swapContent() {
	switch s.mode {
	case components.ProfileNormal:
		s.vp.SetContent(s.cachedNormal)
	case components.ProfileCompact:
		s.vp.SetContent(s.cachedCompact)
	}
	s.vp.GotoTop()
}

func (s ProfileScreen) View() string {
	if !s.loaded && s.profile == nil {
		return lipgloss.NewStyle().Padding(2, 3).Render(
			theme.DimStyle.Render("Loading profile..."),
		)
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

	modeLabel := "Normal"
	if s.mode == components.ProfileCompact {
		modeLabel = "Compact"
	}

	header := theme.BoldStyle.Render("Profile") +
		theme.DimStyle.Render("  mode: ") + theme.AccentStyle.Render(modeLabel) +
		theme.DimStyle.Render("  tab to switch")

	var content string
	switch s.mode {
	case components.ProfileNormal:
		content = s.cachedNormal
	case components.ProfileCompact:
		content = s.cachedCompact
	}

	return header + "\n\n" + content
}

func (s ProfileScreen) loadLocalCmd() tea.Msg {
	data, err := os.ReadFile(config.ProfilePath())
	if err != nil {
		return profileLoadedMsg{}
	}
	var p gh.DatingProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return profileLoadedMsg{err: err}
	}
	return profileLoadedMsg{profile: &p}
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
}

func (s ProfileScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "tab", Desc: "switch mode"},
		{Key: "↑↓", Desc: "scroll"},
		{Key: "esc", Desc: "back"},
	}
}
