package screens

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	"gopkg.in/yaml.v3"
)

type joinStep int

const (
	joinConfigSources joinStep = iota
	joinFetchingSources
	joinToggleFields
	joinFetchTemplate
	joinFillTemplate
	joinEncrypting
	joinSubmitting
	joinDone
	joinError
)

// JoinDoneMsg is emitted when join completes.
type JoinDoneMsg struct {
	PoolName string
	UserHash string
}

// messages
type sourcesFetchedMsg struct {
	profile  *gh.DatingProfile
	ghLogin  string // GitHub login, backfilled if missing from config
	err      error
}
type templateFetchedMsg struct {
	template *gh.RegistrationTemplate
	err      error
}
type issueCreatedMsg struct {
	number int
	err    error
}
type pollResultMsg struct {
	state  string // "open", "closed"
	reason string // "completed", "not_planned"
	err    error
}
type postRegMsg struct {
	userHash string
	err      error
}

type JoinScreen struct {
	step     joinStep
	spinner  spinner.Model
	checkbox components.Checkbox
	input    textinput.Model
	Width    int
	Height   int
	errMsg   string

	// pool info
	poolName    string
	poolRepo    string
	operatorPub string
	relayURL    string

	// user info
	username string
	userID   string
	token    string

	// collected data
	profile       *gh.DatingProfile
	template      *gh.RegistrationTemplate
	templateVals  map[string]string
	templateIdx   int
	issueNumber   int
	userHash      string
	log           []string

	// source config
	includeShowcase bool
	datingProfile   string // profile name (e.g. "default" → dating/default.md), empty = skip
	configCursor    int    // 0=showcase toggle, 1=dating name input, 2=submit
	editingDating   bool   // true when typing in the dating profile name
}

func NewJoinScreen(poolName, poolRepo, operatorPub, relayURL, username, userID string) JoinScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)

	ti := textinput.New()
	ti.Prompt = theme.Cursor()
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Dim)
	ti.CharLimit = 512
	ti.Placeholder = "default"

	return JoinScreen{
		step:            joinConfigSources,
		spinner:         sp,
		input:           ti,
		poolName:        poolName,
		poolRepo:        poolRepo,
		operatorPub:     operatorPub,
		relayURL:        relayURL,
		username:        username,
		userID:          userID,
		templateVals:    make(map[string]string),
		includeShowcase: true,
	}
}

func (s *JoinScreen) addLog(line string) {
	s.log = append(s.log, line)
}

func (s JoinScreen) Update(msg tea.Msg) (JoinScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s.step {
		case joinConfigSources:
			if msg.String() == "esc" {
				return s, func() tea.Msg { return JoinDoneMsg{} }
			}
			if s.editingDating {
				switch msg.String() {
				case "enter":
					s.datingProfile = strings.TrimSpace(s.input.Value())
					s.editingDating = false
					s.configCursor = 2
				case "esc":
					s.editingDating = false
					s.input.Blur()
				default:
					var cmd tea.Cmd
					s.input, cmd = s.input.Update(msg)
					return s, cmd
				}
				return s, nil
			}
			switch msg.String() {
			case "up", "k":
				if s.configCursor > 0 {
					s.configCursor--
				}
			case "down", "j":
				if s.configCursor < 2 {
					s.configCursor++
				}
			case " ":
				if s.configCursor == 0 {
					s.includeShowcase = !s.includeShowcase
				}
			case "enter":
				if s.configCursor == 1 {
					s.editingDating = true
					s.input.Focus()
					return s, textinput.Blink
				}
				if s.configCursor == 2 {
					s.step = joinFetchingSources
					s.addLog(theme.DimStyle.Render("Fetching profile sources..."))
					return s, tea.Batch(s.fetchSources, s.spinner.Tick)
				}
			}
			return s, nil

		case joinToggleFields:
			if msg.String() == "esc" {
				return s, func() tea.Msg { return JoinDoneMsg{} }
			}
			var cmd tea.Cmd
			s.checkbox, cmd = s.checkbox.Update(msg)
			return s, cmd

		case joinFillTemplate:
			if msg.String() == "enter" {
				val := strings.TrimSpace(s.input.Value())
				field := s.template.Fields[s.templateIdx]
				if field.Required && val == "" {
					return s, nil // don't advance
				}
				s.templateVals[field.ID] = val
				s.templateIdx++
				if s.templateIdx >= len(s.template.Fields) {
					// All fields filled → encrypt
					s.step = joinEncrypting
					s.addLog(theme.GreenStyle.Render("✓ ") + "Template fields complete")
					return s, tea.Batch(s.encrypt, s.spinner.Tick)
				}
				// Next field
				s.setupTemplateInput()
				return s, nil
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd

		case joinDone:
			if msg.String() == "enter" {
				return s, func() tea.Msg {
					return JoinDoneMsg{PoolName: s.poolName, UserHash: s.userHash}
				}
			}

		case joinError:
			if msg.String() == "enter" {
				return s, func() tea.Msg { return JoinDoneMsg{} }
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case components.CheckboxSubmitMsg:
		if s.step == joinToggleFields {
			// Build filtered profile from selected fields
			s.applyFieldSelection(msg.Selected)
			s.step = joinFetchTemplate
			s.addLog(theme.GreenStyle.Render("✓ ") + "Profile fields selected")
			s.addLog(theme.DimStyle.Render("  Checking registration template..."))
			return s, tea.Batch(s.fetchTemplate, s.spinner.Tick)
		}

	case sourcesFetchedMsg:
		if msg.err != nil {
			s.step = joinError
			s.errMsg = msg.err.Error()
			s.addLog(theme.RedStyle.Render("✗ ") + msg.err.Error())
			return s, nil
		}
		s.profile = msg.profile
		if msg.ghLogin != "" {
			s.username = msg.ghLogin
		}
		s.addLog(theme.GreenStyle.Render("✓ ") + "Profile fetched")
		// Build field toggle list
		s.step = joinToggleFields
		s.checkbox = s.buildFieldToggle()
		return s, nil

	case templateFetchedMsg:
		if msg.err != nil {
			s.addLog(theme.DimStyle.Render("  No custom template, using defaults"))
			s.template = gh.DefaultTemplate()
		} else {
			s.template = msg.template
			s.addLog(theme.GreenStyle.Render("✓ ") + fmt.Sprintf("Template loaded (%d fields)", len(s.template.Fields)))
		}
		if len(s.template.Fields) > 0 {
			s.step = joinFillTemplate
			s.templateIdx = 0
			s.setupTemplateInput()
		} else {
			s.step = joinEncrypting
			return s, tea.Batch(s.encrypt, s.spinner.Tick)
		}
		return s, nil

	case issueCreatedMsg:
		if msg.err != nil {
			s.step = joinError
			s.errMsg = msg.err.Error()
			s.addLog(theme.RedStyle.Render("✗ ") + msg.err.Error())
			return s, nil
		}
		s.issueNumber = msg.number
		s.addLog(theme.GreenStyle.Render("✓ ") + fmt.Sprintf("Registration issue #%d created", msg.number))
		s.addLog(theme.DimStyle.Render("  Processing will continue in background"))
		s.addLog("")
		s.addLog(theme.DimStyle.Render("  Press enter to continue"))

		// Save pool as pending immediately
		s.savePending()

		s.step = joinDone
		return s, nil

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
	}

	return s, nil
}

func (s JoinScreen) View() string {
	pad := lipgloss.NewStyle().Padding(1, 3)

	// Timeline sidebar
	timeline := s.renderTimeline()

	// Main content area
	var content string

	switch s.step {
	case joinConfigSources:
		content = s.configSourcesView()
	case joinFetchingSources:
		content = s.renderLogWithActive(s.spinner.View() + " Fetching profile data...")
	case joinToggleFields:
		content = s.renderLogWithActive(s.checkbox.View())
	case joinFetchTemplate:
		content = s.renderLogWithActive(s.spinner.View() + " Loading registration template...")
	case joinFillTemplate:
		content = s.renderLogWithActive(s.templateFieldView())
	case joinEncrypting:
		content = s.renderLogWithActive(s.spinner.View() + " Encrypting & submitting...")
	case joinSubmitting:
		content = s.renderLogWithActive(s.spinner.View() + " Submitting registration...")
	case joinDone:
		content = s.renderLogWithActive(
			theme.GreenStyle.Render("✓ ") + theme.BoldStyle.Render("Registration submitted!") + "\n\n" +
				theme.DimStyle.Render("  You'll be notified once the pool processes your registration.") + "\n" +
				theme.DimStyle.Render("  Press enter to go back to home."),
		)
	case joinError:
		content = s.renderLogWithActive(
			theme.RedStyle.Render("✗ " + s.errMsg) + "\n\n" +
				theme.DimStyle.Render("  Press enter to go back"),
		)
	}

	// Layout: timeline | content
	timelineWidth := 32
	left := lipgloss.NewStyle().Width(timelineWidth).Render(timeline)

	return pad.Render(lipgloss.JoinHorizontal(lipgloss.Top, left, content))
}

func (s JoinScreen) renderTimeline() string {
	type step struct {
		label string
		done  bool
		active bool
		failed bool
	}

	steps := []step{
		{label: "Configure Sources", done: s.step > joinConfigSources, active: s.step == joinConfigSources},
		{label: "Fetch Profile", done: s.step > joinFetchingSources, active: s.step == joinFetchingSources},
		{label: "Select Fields", done: s.step > joinToggleFields, active: s.step == joinToggleFields},
		{label: "Registration Form", done: s.step > joinFillTemplate, active: s.step == joinFetchTemplate || s.step == joinFillTemplate},
		{label: "Submit", done: s.step == joinDone, active: s.step == joinEncrypting || s.step == joinSubmitting, failed: s.step == joinError},
	}

	out := theme.BoldStyle.Render("Join: " + s.poolName) + "\n"
	out += theme.DimStyle.Render(s.poolRepo) + "\n\n"

	for i, st := range steps {
		icon := theme.DimStyle.Render("○")
		labelStyle := theme.DimStyle
		connector := theme.DimStyle.Render("│")

		if st.failed {
			icon = theme.RedStyle.Render("✗")
			labelStyle = theme.RedStyle
		} else if st.done {
			icon = theme.GreenStyle.Render("✓")
			labelStyle = theme.TextStyle
		} else if st.active {
			icon = s.spinner.View()
			labelStyle = theme.TextStyle
		}

		out += fmt.Sprintf("  %s %s\n", icon, labelStyle.Render(st.label))
		if i < len(steps)-1 {
			out += fmt.Sprintf("  %s\n", connector)
		}
	}

	return out
}

func (s JoinScreen) renderLogWithActive(active string) string {
	var out string
	for _, line := range s.log {
		out += line + "\n"
	}
	if active != "" {
		out += "\n" + active
	}
	return out
}

func (s JoinScreen) configSourcesView() string {
	labelCol := lipgloss.NewStyle().Width(18)

	out := theme.BoldStyle.Render("Profile Sources") + "\n"
	out += theme.DimStyle.Render("Your email & user ID are never shared") + "\n\n"

	// Consistent row helper
	row := func(cursorIdx int, check, label, desc string) string {
		cur := "  "
		if s.configCursor == cursorIdx {
			cur = theme.Cursor()
		}
		return fmt.Sprintf("  %s%s %s %s\n", cur, check, labelCol.Render(label), theme.DimStyle.Render(desc))
	}

	// GitHub Profile (mandatory, locked)
	out += row(-1, theme.GreenStyle.Render("[✓]"), "GitHub Profile", "always included")

	// Showcase toggle
	showcaseCheck := theme.DimStyle.Render("[ ]")
	if s.includeShowcase {
		showcaseCheck = theme.GreenStyle.Render("[✓]")
	}
	out += row(0, showcaseCheck, "Showcase", s.username+"/README.md")

	// Dating profile
	if s.editingDating {
		cur := theme.Cursor()
		out += fmt.Sprintf("  %s%s %s\n", cur, theme.AmberStyle.Render("[…]"), labelCol.Render("Dating Profile"))
		out += "       " + s.input.View() + "\n"
		out += "       " + theme.DimStyle.Render("→ "+s.username+"/dating/{name}.md") + "\n"
	} else {
		datingCheck := theme.DimStyle.Render("[ ]")
		datingDesc := "enter to set profile name"
		if s.datingProfile != "" {
			datingCheck = theme.GreenStyle.Render("[✓]")
			datingDesc = s.username + "/dating/" + s.datingProfile + ".md"
		}
		out += row(1, datingCheck, "Dating Profile", datingDesc)
	}

	out += "\n"
	out += row(2, " ", "Continue →", "")
	out += "\n"
	out += theme.DimStyle.Render("  You can fine-tune individual fields in the next step")

	return out
}

func (s JoinScreen) HelpBindings() []components.KeyBind {
	switch s.step {
	case joinConfigSources, joinToggleFields:
		return []components.KeyBind{
			{Key: "↑↓", Desc: "navigate"},
			{Key: "space", Desc: "toggle"},
			{Key: "enter", Desc: "confirm"},
			{Key: "esc", Desc: "cancel"},
		}
	case joinFillTemplate:
		return []components.KeyBind{
			{Key: "enter", Desc: "next field"},
			{Key: "esc", Desc: "cancel"},
		}
	default:
		return []components.KeyBind{
			{Key: "enter", Desc: "continue"},
		}
	}
}

// --- helpers ---

func (s *JoinScreen) setupTemplateInput() {
	if s.templateIdx >= len(s.template.Fields) {
		return
	}
	field := s.template.Fields[s.templateIdx]
	s.input.SetValue("")
	s.input.Placeholder = field.Label
	s.input.Focus()
}

func (s JoinScreen) templateFieldView() string {
	if s.templateIdx >= len(s.template.Fields) {
		return ""
	}
	field := s.template.Fields[s.templateIdx]

	out := "  " + theme.BoldStyle.Render(field.Label)
	if field.Required {
		out += theme.RedStyle.Render(" *")
	}
	out += "\n"
	if field.Description != "" {
		out += "  " + theme.DimStyle.Render(field.Description) + "\n"
	}
	out += "\n  " + s.input.View() + "\n"
	out += "\n  " + theme.DimStyle.Render(fmt.Sprintf("Field %d/%d", s.templateIdx+1, len(s.template.Fields)))
	return out
}

func (s JoinScreen) buildFieldToggle() components.Checkbox {
	var items []components.CheckboxItem
	p := s.profile

	add := func(id, label, value string) {
		muted := value == ""
		display := value
		if muted {
			display = "not set"
		} else if len(display) > 50 {
			display = display[:47] + "..."
		}
		items = append(items, components.CheckboxItem{
			ID: id, Label: label, Value: display,
			Checked: !muted, Muted: muted,
		})
	}

	add("display_name", "Name", p.DisplayName)
	add("bio", "Bio", p.Bio)
	add("location", "Location", p.Location)
	add("avatar_url", "Avatar", p.AvatarURL)
	add("website", "Website", p.Website)
	if len(p.Social) > 0 {
		add("social", "Social", strings.Join(p.Social, ", "))
	} else {
		add("social", "Social", "")
	}
	add("showcase", "Showcase", func() string {
		if p.Showcase != "" {
			return fmt.Sprintf("(%d chars, base64)", len(p.Showcase))
		}
		return ""
	}())
	if len(p.Interests) > 0 {
		add("interests", "Interests", strings.Join(p.Interests, ", "))
	} else {
		add("interests", "Interests", "")
	}
	if len(p.LookingFor) > 0 {
		add("looking_for", "Looking for", strings.Join(p.LookingFor, ", "))
	} else {
		add("looking_for", "Looking for", "")
	}
	add("about", "About", p.About)

	return components.NewCheckbox("Select fields to include", items)
}

func (s *JoinScreen) applyFieldSelection(selected []components.CheckboxItem) {
	enabled := make(map[string]bool)
	for _, item := range selected {
		enabled[item.ID] = true
	}

	if !enabled["display_name"] {
		s.profile.DisplayName = ""
	}
	if !enabled["bio"] {
		s.profile.Bio = ""
	}
	if !enabled["location"] {
		s.profile.Location = ""
	}
	if !enabled["avatar_url"] {
		s.profile.AvatarURL = ""
	}
	if !enabled["website"] {
		s.profile.Website = ""
	}
	if !enabled["social"] {
		s.profile.Social = nil
	}
	if !enabled["showcase"] {
		s.profile.Showcase = ""
	}
	if !enabled["interests"] {
		s.profile.Interests = nil
	}
	if !enabled["looking_for"] {
		s.profile.LookingFor = nil
	}
	if !enabled["about"] {
		s.profile.About = ""
	}
}

func (s JoinScreen) savePending() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	cfg.AddPool(config.PoolConfig{
		Name:           s.poolName,
		Repo:           s.poolRepo,
		OperatorPubKey: s.operatorPub,
		RelayURL:       s.relayURL,
		Status:         "pending",
	})
	if cfg.Active == "" {
		cfg.Active = s.poolName
	}
	cfg.Save()
}

// --- async commands ---

func (s JoinScreen) fetchSources() tea.Msg {
	ctx := context.Background()

	// Decrypt token
	cfg, err := config.Load()
	if err != nil {
		return sourcesFetchedMsg{err: fmt.Errorf("loading config: %w", err)}
	}
	_, priv, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return sourcesFetchedMsg{err: fmt.Errorf("loading keys: %w", err)}
	}
	token, err := cfg.DecryptToken(priv)
	if err != nil {
		return sourcesFetchedMsg{err: fmt.Errorf("decrypting token: %w", err)}
	}
	s.token = token

	var profiles []*gh.DatingProfile
	ghLogin := ""

	// GitHub profile (always)
	p, login, err := fetchGitHubProfileForJoin(ctx, token)
	if err == nil {
		profiles = append(profiles, p)
		ghLogin = login
	}

	// Showcase from identity repo
	if s.includeShowcase {
		showcase, err := fetchIdentityReadmeForJoin(s.username)
		if err == nil && showcase != "" {
			profiles = append(profiles, &gh.DatingProfile{Showcase: showcase})
		}
	}

	// Dating profile
	if s.datingProfile != "" {
		data, err := fetchDatingProfileForJoin(s.username, s.datingProfile)
		if err == nil && data != nil {
			profiles = append(profiles, &gh.DatingProfile{
				Interests:  data.interests,
				LookingFor: data.lookingFor,
				About:      data.about,
			})
		}
	}

	merged := mergeProfilesForJoin(profiles...)
	return sourcesFetchedMsg{profile: merged, ghLogin: ghLogin}
}

func (s JoinScreen) fetchTemplate() tea.Msg {
	poolURL := gitrepo.EnsureGitURL(s.poolRepo)
	repo, err := gitrepo.Clone(poolURL)
	if err != nil {
		return templateFetchedMsg{err: err}
	}

	data, err := repo.ReadFile(".github/registration.yml")
	if err != nil {
		return templateFetchedMsg{err: err}
	}

	tmpl, err := gh.ParseRegistrationTemplate(data)
	return templateFetchedMsg{template: tmpl, err: err}
}

func (s JoinScreen) encrypt() tea.Msg {
	if s.profile == nil {
		return issueCreatedMsg{err: fmt.Errorf("no profile data")}
	}

	// Load keys
	pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("loading keys: %w", err)}
	}

	// Decrypt token
	cfg, err := config.Load()
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("loading config: %w", err)}
	}
	token, err := cfg.DecryptToken(priv)
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("decrypting token: %w", err)}
	}

	// Serialize profile
	profileJSON, err := json.Marshal(s.profile)
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("serializing profile: %w", err)}
	}

	// Save profile locally
	os.MkdirAll(config.Dir(), 0700)
	os.WriteFile(config.ProfilePath(), profileJSON, 0600)

	// Pack binary
	operatorPubBytes, err := hex.DecodeString(s.operatorPub)
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("decoding operator key: %w", err)}
	}
	bin, err := crypto.PackUserBin(pub, operatorPubBytes, profileJSON)
	if err != nil {
		return issueCreatedMsg{err: fmt.Errorf("packing profile: %w", err)}
	}

	blobHex := hex.EncodeToString(bin)

	// Build issue body
	body := gh.RenderIssueBody(s.template, s.templateVals, blobHex)

	// Submit issue
	ctx := context.Background()
	client := gh.NewClient(s.poolRepo, token)
	number, err := client.CreateIssue(ctx, s.template.Title, body, s.template.Labels)
	return issueCreatedMsg{number: number, err: err}
}

func (s JoinScreen) pollIssue() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return pollResultMsg{err: err}
	}
	_, priv, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return pollResultMsg{err: err}
	}
	token, err := cfg.DecryptToken(priv)
	if err != nil {
		return pollResultMsg{err: err}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := gh.NewClient(s.poolRepo, token)
	issue, err := client.GetIssue(ctx, s.issueNumber)
	if err != nil {
		return pollResultMsg{err: err}
	}

	return pollResultMsg{state: issue.State, reason: issue.StateReason}
}

func (s JoinScreen) postRegistration() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return postRegMsg{err: err}
	}
	_, priv, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return postRegMsg{err: err}
	}
	token, err := cfg.DecryptToken(priv)
	if err != nil {
		return postRegMsg{err: err}
	}

	ctx := context.Background()

	// Star the repo
	client := gh.NewClient(s.poolRepo, token)
	client.StarRepo(ctx)

	// Get hash from relay
	pub, _, _ := crypto.LoadKeyPair(config.KeysDir())
	pubHex := hex.EncodeToString(pub)
	signature := crypto.Sign(priv, []byte("identity:"+pubHex))

	userHash := ""
	if s.relayURL != "" {
		hash, err := fetchIdentityFromRelay(ctx, s.relayURL, s.poolRepo, pubHex, signature)
		if err == nil {
			userHash = hash
		}
	}

	// Save to config
	cfg.AddPool(config.PoolConfig{
		Name:           s.poolName,
		Repo:           s.poolRepo,
		OperatorPubKey: s.operatorPub,
		RelayURL:       s.relayURL,
		Status:         "active",
		UserHash:       userHash,
	})
	if cfg.Active == "" {
		cfg.Active = s.poolName
	}
	if err := cfg.Save(); err != nil {
		return postRegMsg{err: err}
	}

	return postRegMsg{userHash: userHash}
}

// --- thin wrappers to avoid import cycles ---

type datingReadmeResult struct {
	interests  []string
	lookingFor []gh.LookingFor
	about      string
}

func fetchGitHubProfileForJoin(ctx context.Context, token string) (*gh.DatingProfile, string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("GitHub API %d", resp.StatusCode)
	}
	var user struct {
		Name            string `json:"name"`
		Login           string `json:"login"`
		Bio             string `json:"bio"`
		Location        string `json:"location"`
		AvatarURL       string `json:"avatar_url"`
		Blog            string `json:"blog"`
		TwitterUsername string `json:"twitter_username"`
	}
	json.NewDecoder(resp.Body).Decode(&user)
	name := user.Name
	if name == "" {
		name = user.Login
	}
	var social []string
	if user.TwitterUsername != "" {
		social = append(social, "https://twitter.com/"+user.TwitterUsername)
	}
	return &gh.DatingProfile{
		DisplayName: name, Bio: user.Bio, Location: user.Location,
		AvatarURL: user.AvatarURL, Website: user.Blog, Social: social,
	}, user.Login, nil
}

func fetchIdentityReadmeForJoin(username string) (string, error) {
	repoURL := gitrepo.EnsureGitURL(username + "/" + username)
	repo, err := gitrepo.Clone(repoURL)
	if err != nil {
		return "", err
	}
	data, err := repo.ReadFile("README.md")
	if err != nil {
		return "", err
	}
	return b64Encode(data), nil
}

func fetchDatingProfileForJoin(username, profileName string) (*datingReadmeResult, error) {
	repoURL := gitrepo.EnsureGitURL(username + "/dating")
	repo, err := gitrepo.Clone(repoURL)
	if err != nil {
		return nil, err
	}
	data, err := repo.ReadFile(profileName + ".md")
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return &datingReadmeResult{about: strings.TrimSpace(content)}, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return &datingReadmeResult{about: strings.TrimSpace(content)}, nil
	}

	var fm struct {
		Interests  []string        `yaml:"interests"`
		LookingFor []gh.LookingFor `yaml:"looking_for"`
	}
	yaml.Unmarshal([]byte(parts[1]), &fm)

	about := strings.TrimSpace(parts[2])
	if strings.HasPrefix(about, "# About") {
		about = strings.TrimSpace(strings.TrimPrefix(about, "# About"))
	}

	return &datingReadmeResult{
		interests: fm.Interests, lookingFor: fm.LookingFor, about: about,
	}, nil
}

func fetchDatingReadmeForJoin(username string) (*datingReadmeResult, error) {
	repoURL := gitrepo.EnsureGitURL(username + "/dating")
	repo, err := gitrepo.Clone(repoURL)
	if err != nil {
		return nil, err
	}
	data, err := repo.ReadFile("README.md")
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return &datingReadmeResult{about: strings.TrimSpace(content)}, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return &datingReadmeResult{about: strings.TrimSpace(content)}, nil
	}

	var fm struct {
		Interests  []string        `yaml:"interests"`
		LookingFor []gh.LookingFor `yaml:"looking_for"`
	}
	yaml.Unmarshal([]byte(parts[1]), &fm)

	about := strings.TrimSpace(parts[2])
	if strings.HasPrefix(about, "# About") {
		about = strings.TrimSpace(strings.TrimPrefix(about, "# About"))
	}

	return &datingReadmeResult{
		interests: fm.Interests, lookingFor: fm.LookingFor, about: about,
	}, nil
}

func mergeProfilesForJoin(profiles ...*gh.DatingProfile) *gh.DatingProfile {
	merged := &gh.DatingProfile{}
	for _, p := range profiles {
		if p == nil {
			continue
		}
		if p.DisplayName != "" {
			merged.DisplayName = p.DisplayName
		}
		if p.Bio != "" {
			merged.Bio = p.Bio
		}
		if p.Location != "" {
			merged.Location = p.Location
		}
		if p.AvatarURL != "" {
			merged.AvatarURL = p.AvatarURL
		}
		if p.Website != "" {
			merged.Website = p.Website
		}
		if len(p.Social) > 0 {
			merged.Social = p.Social
		}
		if p.Showcase != "" {
			merged.Showcase = p.Showcase
		}
		if len(p.Interests) > 0 {
			merged.Interests = p.Interests
		}
		if len(p.LookingFor) > 0 {
			merged.LookingFor = p.LookingFor
		}
		if p.About != "" {
			merged.About = p.About
		}
	}
	return merged
}

func fetchIdentityFromRelay(ctx context.Context, relayURL, poolRepo, pubKeyHex, signature string) (string, error) {
	body := fmt.Sprintf(`{"pool_repo":"%s","pub_key":"%s","signature":"%s"}`, poolRepo, pubKeyHex, signature)
	req, err := http.NewRequestWithContext(ctx, "POST", relayURL+"/identity", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("relay returned %d", resp.StatusCode)
	}

	var result struct {
		UserHash string `json:"user_hash"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.UserHash, nil
}

func b64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
