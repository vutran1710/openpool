package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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

// ProfileUpdateMsg is emitted when the user saves an edited profile.
type ProfileUpdateMsg struct {
	Profile *gh.DatingProfile
}

type profileEditStep int

const (
	editInterests profileEditStep = iota
	editIntent
	editGenderTarget
	editAbout
	editConfirm
)

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

	// Edit mode
	editing      bool
	editStep     profileEditStep
	editProfile  *gh.DatingProfile // working copy during edit
	interestInput textinput.Model
	aboutInput    textarea.Model
	intentCheckbox    components.Checkbox
	genderCheckbox    components.Checkbox
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
	if s.editing {
		return s.updateEdit(msg)
	}

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
		case "e":
			if s.profile != nil {
				s.startEdit()
				return s, nil
			}
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
		s.vp.Height = msg.Height - 12
		s.updateContent()
		return s, nil
	}

	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s *ProfileScreen) startEdit() {
	dbg.Log("profile: entering edit mode")
	s.editing = true
	s.editStep = editInterests

	// Copy profile for editing
	copy := *s.profile
	s.editProfile = &copy

	// Init interests input
	ti := textinput.New()
	ti.Placeholder = "type interest, press enter to add, ctrl+d when done"
	ti.Focus()
	ti.CharLimit = 100
	if len(s.profile.Interests) > 0 {
		ti.SetValue(strings.Join(s.profile.Interests, ", "))
	}
	s.interestInput = ti

	// Init about textarea
	ta := textarea.New()
	ta.Placeholder = "Tell people about yourself (500 chars max)"
	ta.CharLimit = 500
	ta.SetWidth(50)
	ta.SetHeight(5)
	if s.profile.About != "" {
		ta.SetValue(s.profile.About)
	}
	s.aboutInput = ta

	// Init intent checkbox
	var intentItems []components.CheckboxItem
	for _, opt := range gh.AllIntentOptions() {
		checked := false
		for _, v := range s.profile.Intent {
			if v == opt {
				checked = true
				break
			}
		}
		intentItems = append(intentItems, components.CheckboxItem{Label: opt, Checked: checked})
	}
	s.intentCheckbox = components.NewCheckbox("What are you looking for?", intentItems)

	// Init gender checkbox
	var genderItems []components.CheckboxItem
	for _, opt := range gh.AllGenderTargetOptions() {
		checked := false
		for _, v := range s.profile.GenderTarget {
			if v == opt {
				checked = true
				break
			}
		}
		genderItems = append(genderItems, components.CheckboxItem{Label: opt, Checked: checked})
	}
	s.genderCheckbox = components.NewCheckbox("Who are you interested in?", genderItems)
}

func (s ProfileScreen) updateEdit(msg tea.Msg) (ProfileScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s.editStep {
		case editInterests:
			switch msg.String() {
			case "ctrl+d", "ctrl+s":
				val := strings.TrimSpace(s.interestInput.Value())
				if val != "" {
					parts := strings.Split(val, ",")
					var interests []string
					for _, p := range parts {
						p = strings.TrimSpace(p)
						if p != "" {
							interests = append(interests, p)
						}
					}
					s.editProfile.Interests = interests
				} else {
					s.editProfile.Interests = nil
				}
				s.editStep = editIntent
				return s, nil
			case "esc":
				s.editing = false
				return s, nil
			}
			var cmd tea.Cmd
			s.interestInput, cmd = s.interestInput.Update(msg)
			return s, cmd

		case editIntent:
			switch msg.String() {
			case "esc":
				s.editing = false
				return s, nil
			}
			var cmd tea.Cmd
			s.intentCheckbox, cmd = s.intentCheckbox.Update(msg)
			return s, cmd

		case editGenderTarget:
			switch msg.String() {
			case "esc":
				s.editing = false
				return s, nil
			}
			var cmd tea.Cmd
			s.genderCheckbox, cmd = s.genderCheckbox.Update(msg)
			return s, cmd

		case editAbout:
			switch msg.String() {
			case "ctrl+d", "ctrl+s":
				s.editProfile.About = strings.TrimSpace(s.aboutInput.Value())
				s.editStep = editConfirm
				return s, nil
			case "esc":
				s.editing = false
				return s, nil
			}
			var cmd tea.Cmd
			s.aboutInput, cmd = s.aboutInput.Update(msg)
			return s, cmd

		case editConfirm:
			switch msg.String() {
			case "enter", "y":
				dbg.Log("profile: saving edited profile")
				s.profile = s.editProfile
				s.editing = false
				s.cacheWidth = 0 // force rebuild
				s.updateContent()
				// Save to disk
				profileJSON, err := json.Marshal(s.profile)
				if err != nil {
					dbg.Log("profile: marshal error: %v", err)
					return s, nil
				}
				os.MkdirAll(config.Dir(), 0700)
				if err := os.WriteFile(config.ProfilePath(), profileJSON, 0600); err != nil {
					dbg.Log("profile: write error: %v", err)
				}
				dbg.Log("profile: saved to %s", config.ProfilePath())
				// Also save to active pool profile
				cfg, _ := config.Load()
				if cfg != nil && cfg.Active != "" {
					poolPath := config.PoolProfilePath(cfg.Active)
					dir := poolPath[:strings.LastIndex(poolPath, "/")]
					os.MkdirAll(dir, 0700)
					os.WriteFile(poolPath, profileJSON, 0600)
					dbg.Log("profile: saved pool profile to %s", poolPath)
				}
				// Emit update message for app.go to submit to pool
				profile := s.profile
				return s, func() tea.Msg {
					return ProfileUpdateMsg{Profile: profile}
				}
			case "esc", "n":
				s.editing = false
				return s, nil
			}
		}

	case profileEditAdvanceMsg:
		switch s.editStep {
		case editIntent:
			s.editStep = editGenderTarget
		case editGenderTarget:
			s.editStep = editAbout
			s.aboutInput.Focus()
		}
		return s, nil

	case components.CheckboxSubmitMsg:
		dbg.Log("profile: checkbox submit at step %d, %d selected", s.editStep, len(msg.Selected))
		switch s.editStep {
		case editIntent:
			var intents []gh.Intent
			for _, item := range msg.Selected {
				if item.Checked {
					intents = append(intents, item.Label)
				}
			}
			s.editProfile.Intent = intents
			dbg.Log("profile: intent set to %v", intents)
			s.editStep = editGenderTarget
			return s, nil
		case editGenderTarget:
			var targets []gh.GenderTarget
			for _, item := range msg.Selected {
				if item.Checked {
					targets = append(targets, item.Label)
				}
			}
			s.editProfile.GenderTarget = targets
			dbg.Log("profile: gender target set to %v", targets)
			s.editStep = editAbout
			s.aboutInput.Focus()
			return s, nil
		}
	}
	return s, nil
}

type profileEditAdvanceMsg struct{}

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
	if s.editing {
		return s.editView()
	}

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
		theme.DimStyle.Render("  tab to switch  ·  e to edit")

	var content string
	switch s.mode {
	case components.ProfileNormal:
		content = s.cachedNormal
	case components.ProfileCompact:
		content = s.cachedCompact
	}

	return header + "\n\n" + content
}

func (s ProfileScreen) editView() string {
	pad := lipgloss.NewStyle().Padding(1, 3)

	stepLabels := []string{"Interests", "Intent", "Gender Target", "About", "Confirm"}
	progress := theme.DimStyle.Render(fmt.Sprintf("Edit Profile  ·  Step %d/5: ", s.editStep+1)) +
		theme.AccentStyle.Render(stepLabels[s.editStep])

	var content string
	switch s.editStep {
	case editInterests:
		current := ""
		if len(s.editProfile.Interests) > 0 {
			tags := ""
			for _, i := range s.editProfile.Interests {
				tags += theme.GreenStyle.Render(" #"+i)
			}
			current = "\n" + tags + "\n"
		}
		content = theme.BoldStyle.Render("Interests") + "\n" +
			theme.DimStyle.Render("Comma-separated. Ctrl+D when done.") + "\n\n" +
			s.interestInput.View() +
			current

	case editIntent:
		content = s.intentCheckbox.View()

	case editGenderTarget:
		content = s.genderCheckbox.View()

	case editAbout:
		content = theme.BoldStyle.Render("About You") + "\n" +
			theme.DimStyle.Render("500 chars max. Ctrl+D when done.") + "\n\n" +
			s.aboutInput.View()

	case editConfirm:
		var preview strings.Builder
		preview.WriteString(theme.BoldStyle.Render("Review Changes") + "\n\n")

		if len(s.editProfile.Interests) > 0 {
			preview.WriteString(theme.DimStyle.Render("Interests: "))
			for _, i := range s.editProfile.Interests {
				preview.WriteString(theme.GreenStyle.Render("#" + i + " "))
			}
			preview.WriteString("\n")
		}
		if len(s.editProfile.Intent) > 0 {
			preview.WriteString(theme.DimStyle.Render("Intent: ") + strings.Join(s.editProfile.Intent, ", ") + "\n")
		}
		if len(s.editProfile.GenderTarget) > 0 {
			preview.WriteString(theme.DimStyle.Render("Looking for: ") + strings.Join(s.editProfile.GenderTarget, ", ") + "\n")
		}
		if s.editProfile.About != "" {
			about := s.editProfile.About
			if len(about) > 100 {
				about = about[:100] + "..."
			}
			preview.WriteString(theme.DimStyle.Render("About: ") + about + "\n")
		}
		preview.WriteString("\n" + theme.DimStyle.Render("y/enter") + " save & submit  " +
			theme.DimStyle.Render("esc/n") + " cancel")
		content = preview.String()
	}

	return pad.Render(progress + "\n\n" + content)
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
	if s.editing {
		switch s.editStep {
		case editInterests, editAbout:
			return []components.KeyBind{
				{Key: "ctrl+d", Desc: "next"},
				{Key: "esc", Desc: "cancel"},
			}
		case editIntent, editGenderTarget:
			return []components.KeyBind{
				{Key: "space", Desc: "toggle"},
				{Key: "ctrl+d", Desc: "next"},
				{Key: "esc", Desc: "cancel"},
			}
		case editConfirm:
			return []components.KeyBind{
				{Key: "enter", Desc: "save"},
				{Key: "esc", Desc: "cancel"},
			}
		}
	}
	return []components.KeyBind{
		{Key: "e", Desc: "edit"},
		{Key: "tab", Desc: "switch mode"},
		{Key: "↑↓", Desc: "scroll"},
		{Key: "esc", Desc: "back"},
	}
}
