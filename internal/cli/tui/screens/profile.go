package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type profileLoadedMsg struct {
	profile *gh.DatingProfile
	err     error
}

type ProfileScreen struct {
	profile *gh.DatingProfile
	mode    components.ProfileMode
	leftVP  viewport.Model
	rightVP viewport.Model
	loaded  bool
	err     string
	Width   int
	Height  int
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

func (s ProfileScreen) Init() tea.Cmd {
	return s.LoadCmd
}

func (s ProfileScreen) IsLoaded() bool {
	return s.loaded
}

func (s ProfileScreen) LoadCmd() tea.Msg {
	return s.loadLocalCmd()
}

func (s ProfileScreen) Update(msg tea.Msg) (ProfileScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			s.mode = (s.mode + 1) % 3
			s.updateContent()
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
		h := msg.Height - 5
		if h < 5 {
			h = 10
		}
		leftW := msg.Width*48/100 - 2
		rightW := msg.Width - leftW - 6
		s.leftVP.Width = leftW
		s.leftVP.Height = h
		s.rightVP.Width = rightW
		s.rightVP.Height = h
		s.updateContent()
		return s, nil
	}

	// Scroll right viewport
	var cmd tea.Cmd
	s.rightVP, cmd = s.rightVP.Update(msg)
	return s, cmd
}

func (s *ProfileScreen) updateContent() {
	if s.profile == nil {
		return
	}

	leftContent := components.RenderProfile(*s.profile, s.leftVP.Width, s.mode)
	s.leftVP.SetContent(leftContent)
	s.leftVP.GotoTop()

	if components.HasShowcase(*s.profile) {
		rightContent := renderShowcaseClean(*s.profile, s.rightVP.Width)
		s.rightVP.SetContent(rightContent)
		s.rightVP.GotoTop()
	} else {
		s.rightVP.SetContent(theme.DimStyle.Render("\n  No showcase available.\n\n  Create a README.md in your\n  GitHub identity repo."))
	}
}

func (s ProfileScreen) View() string {
	// Trigger load if not loaded
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

	modeLabels := []string{"Full", "Short", "Compact"}

	header := theme.BoldStyle.Render("Profile") +
		theme.DimStyle.Render("  mode: ") + theme.AccentStyle.Render(modeLabels[s.mode]) +
		theme.DimStyle.Render("  tab to switch")

	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Violet).
		BorderTop(true).
		Render(s.leftVP.View())

	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		BorderTop(true).
		Render(s.rightVP.View())

	// Title overlays
	leftTitle := theme.BoldStyle.Render(" Profile ")
	rightTitle := theme.DimStyle.Render(" Showcase ")

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	// Inject titles into first line
	_ = leftTitle
	_ = rightTitle

	return header + "\n" + panels
}

func (s ProfileScreen) loadLocalCmd() tea.Msg {
	data, err := os.ReadFile(config.ProfilePath())
	if err != nil {
		return profileLoadedMsg{err: nil} // no profile is not an error
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
	s.updateContent()
}

func (s ProfileScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "tab", Desc: "switch mode"},
		{Key: "↑↓", Desc: "scroll showcase"},
		{Key: "esc", Desc: "back"},
	}
}

// renderShowcaseClean pre-processes the markdown to be terminal-friendly,
// then renders with glamour.
func renderShowcaseClean(p gh.DatingProfile, width int) string {
	if !components.HasShowcase(p) {
		return ""
	}

	raw := components.DecodeShowcase(p)
	if raw == "" {
		return ""
	}

	// Pre-process: strip images, keep alt text
	imgRe := regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	raw = imgRe.ReplaceAllString(raw, "$1")

	// Pre-process: shorten inline links to just the text
	// [text](url) → text
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	raw = linkRe.ReplaceAllString(raw, "$1")

	// Strip raw URLs that are very long (> 60 chars)
	urlRe := regexp.MustCompile(`https?://[^\s)]+`)
	raw = urlRe.ReplaceAllStringFunc(raw, func(url string) string {
		if len(url) > 60 {
			return url[:57] + "..."
		}
		return url
	})

	// Render with glamour
	mdWidth := width - 4
	if mdWidth < 30 {
		mdWidth = 40
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(mdWidth),
	)
	if err != nil {
		return raw
	}
	rendered, err := renderer.Render(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(rendered)
}

// ── Match Detail Screen (page-flipping brochure) ──

type MatchDetailScreen struct {
	profile  *gh.DatingProfile
	page     int
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
		content = renderShowcaseClean(*s.profile, width)
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

