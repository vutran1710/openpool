package screens

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

var onboardingHTTPClient = &http.Client{Timeout: 30 * time.Second}

type onboardingStep int

const (
	stepWelcome onboardingStep = iota
	stepCheckGH
	stepAskToken
	stepValidating
	stepGenerateKeys
	stepAskRegistry
	stepCloningRegistry
	stepFetchingPools
	stepSaving
	stepDone
	stepError
)

type OnboardingDoneMsg struct {
	DisplayName string
	Username    string
	UserID      string
	Token       string
	Registry    string
}

type ghCheckResult struct {
	token string
	err   error
}
type tokenValidResult struct {
	userID, username, displayName string
	err                          error
}
type keysResult struct {
	pub ed25519.PublicKey
	err error
}
type registryCloneResult struct {
	repoURL string
	err     error
}
type poolsFetchResult struct {
	pools []gh.PoolEntry
	err   error
}
type saveResult struct {
	err error
}

// timeline step states
type stepState int

const (
	statePending stepState = iota
	stateActive
	stateDone
	stateFailed
)

type timelineStep struct {
	label string
	state stepState
	info  string // result text shown after completion
}

type OnboardingScreen struct {
	step     onboardingStep
	spinner  spinner.Model
	input    textinput.Model
	viewport viewport.Model
	Width    int
	Height   int
	errMsg   string
	errRetry onboardingStep

	// timeline
	timeline []timelineStep
	log      []string // accumulated log lines

	// collected data
	token       string
	userID      string
	username    string
	displayName string
	pubKey      ed25519.PublicKey
	registryURL string
	pools       []gh.PoolEntry
}

func NewOnboardingScreen() OnboardingScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.Pink)

	ti := textinput.New()
	ti.Placeholder = "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	ti.Prompt = theme.Cursor()
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Dim)
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 256

	vp := viewport.New(80, 20)

	return OnboardingScreen{
		step:     stepWelcome,
		spinner:  sp,
		input:    ti,
		viewport: vp,
		timeline: []timelineStep{
			{label: "GitHub Authentication", state: statePending},
			{label: "Generate Keys", state: statePending},
			{label: "Registry Setup", state: statePending},
			{label: "Save Configuration", state: statePending},
		},
	}
}

func (s *OnboardingScreen) addLog(line string) {
	s.log = append(s.log, line)
}

func (s *OnboardingScreen) setTimelineState(idx int, state stepState, info string) {
	if idx < len(s.timeline) {
		s.timeline[idx].state = state
		if info != "" {
			s.timeline[idx].info = info
		}
	}
}

func (s OnboardingScreen) Update(msg tea.Msg) (OnboardingScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s.step {
		case stepWelcome:
			if msg.String() == "enter" {
				s.step = stepCheckGH
				s.setTimelineState(0, stateActive, "")
				s.addLog(theme.DimStyle.Render("Checking for GitHub CLI..."))
				return s, tea.Batch(s.checkGH, s.spinner.Tick)
			}

		case stepAskToken:
			if msg.String() == "enter" {
				val := strings.TrimSpace(s.input.Value())
				if val != "" {
					s.token = val
					s.step = stepValidating
					s.addLog(theme.DimStyle.Render("Validating token..."))
					return s, tea.Batch(s.validateToken, s.spinner.Tick)
				}
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd

		case stepAskRegistry:
			if msg.String() == "enter" {
				val := strings.TrimSpace(s.input.Value())
				if val != "" {
					s.registryURL = val
					s.step = stepCloningRegistry
					s.addLog(theme.DimStyle.Render("Cloning registry..."))
					return s, tea.Batch(s.cloneRegistry, s.spinner.Tick)
				}
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd

		case stepError:
			if msg.String() == "enter" {
				s.step = s.errRetry
				s.errMsg = ""
				s.input.SetValue("")
				s.input.Focus()
				if s.step == stepAskToken {
					s.input.EchoMode = textinput.EchoPassword
					s.input.Placeholder = "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
				} else if s.step == stepAskRegistry {
					s.input.EchoMode = textinput.EchoNormal
					s.input.Placeholder = "owner/repo or git URL"
				}
				return s, textinput.Blink
			}

		case stepDone:
			if msg.String() == "enter" {
				return s, func() tea.Msg {
					return OnboardingDoneMsg{
						DisplayName: s.displayName,
						Username:    s.username,
						UserID:      s.userID,
						Token:       s.token,
						Registry:    s.registryURL,
					}
				}
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case ghCheckResult:
		if msg.err == nil && msg.token != "" {
			s.token = msg.token
			s.step = stepValidating
			s.addLog(theme.GreenStyle.Render("✓ ") + theme.TextStyle.Render("GitHub CLI token detected"))
			s.addLog(theme.DimStyle.Render("  Validating token..."))
			return s, tea.Batch(s.validateToken, s.spinner.Tick)
		}
		s.addLog(theme.DimStyle.Render("  GitHub CLI not found — manual token needed"))
		s.step = stepAskToken
		s.input.Focus()
		s.input.EchoMode = textinput.EchoPassword
		s.input.Placeholder = "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
		return s, textinput.Blink

	case tokenValidResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskToken
			s.errMsg = msg.err.Error()
			s.setTimelineState(0, stateFailed, msg.err.Error())
			s.addLog(theme.RedStyle.Render("✗ ") + theme.RedStyle.Render(msg.err.Error()))
			return s, nil
		}
		s.userID = msg.userID
		s.username = msg.username
		s.displayName = msg.displayName
		s.setTimelineState(0, stateDone, fmt.Sprintf("%s (@%s)", s.displayName, s.username))
		s.addLog(theme.GreenStyle.Render("✓ ") + theme.TextStyle.Render(fmt.Sprintf("Authenticated as %s (@%s)", s.displayName, s.username)))

		s.step = stepGenerateKeys
		s.setTimelineState(1, stateActive, "")
		s.addLog(theme.DimStyle.Render("  Generating keypair..."))
		return s, tea.Batch(s.generateKeys, s.spinner.Tick)

	case keysResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskToken
			s.errMsg = msg.err.Error()
			s.setTimelineState(1, stateFailed, msg.err.Error())
			s.addLog(theme.RedStyle.Render("✗ ") + theme.RedStyle.Render(msg.err.Error()))
			return s, nil
		}
		s.pubKey = msg.pub
		keyID := crypto.ShortHash(hex.EncodeToString(s.pubKey))
		s.setTimelineState(1, stateDone, keyID)
		s.addLog(theme.GreenStyle.Render("✓ ") + theme.TextStyle.Render("Keypair ready: ") + theme.DimStyle.Render(keyID))

		s.step = stepAskRegistry
		s.setTimelineState(2, stateActive, "")
		s.input.SetValue("")
		s.input.EchoMode = textinput.EchoNormal
		s.input.Placeholder = "owner/repo or git URL"
		s.input.Focus()
		return s, textinput.Blink

	case registryCloneResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskRegistry
			s.errMsg = msg.err.Error()
			s.setTimelineState(2, stateFailed, msg.err.Error())
			s.addLog(theme.RedStyle.Render("✗ ") + theme.RedStyle.Render(msg.err.Error()))
			return s, nil
		}
		s.registryURL = msg.repoURL
		s.addLog(theme.GreenStyle.Render("✓ ") + theme.TextStyle.Render("Registry cloned"))
		s.addLog(theme.DimStyle.Render("  Discovering pools..."))
		s.step = stepFetchingPools
		return s, tea.Batch(s.fetchPools, s.spinner.Tick)

	case poolsFetchResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskRegistry
			s.errMsg = msg.err.Error()
			s.setTimelineState(2, stateFailed, msg.err.Error())
			s.addLog(theme.RedStyle.Render("✗ ") + theme.RedStyle.Render(msg.err.Error()))
			return s, nil
		}
		s.pools = msg.pools
		s.setTimelineState(2, stateDone, fmt.Sprintf("%d pool(s) found", len(s.pools)))
		if len(s.pools) > 0 {
			for _, p := range s.pools {
				s.addLog(theme.AccentStyle.Render("  ◈ ") + theme.TextStyle.Render(p.Name) + theme.DimStyle.Render("  "+p.Description))
			}
		} else {
			s.addLog(theme.DimStyle.Render("  No pools found in this registry"))
		}

		s.step = stepSaving
		s.setTimelineState(3, stateActive, "")
		s.addLog(theme.DimStyle.Render("  Encrypting token & saving..."))
		return s, tea.Batch(s.saveConfig, s.spinner.Tick)

	case saveResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskRegistry
			s.errMsg = msg.err.Error()
			s.setTimelineState(3, stateFailed, msg.err.Error())
			s.addLog(theme.RedStyle.Render("✗ ") + theme.RedStyle.Render(msg.err.Error()))
			return s, nil
		}
		s.setTimelineState(3, stateDone, "saved")
		s.addLog(theme.GreenStyle.Render("✓ ") + theme.TextStyle.Render("Configuration saved"))
		s.addLog("")
		s.addLog(theme.GreenStyle.Render("  Setup complete! ") + theme.DimStyle.Render("Press enter to continue"))
		s.step = stepDone
		return s, nil

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
		s.viewport.Width = msg.Width - 6
		s.viewport.Height = msg.Height - 12 // leave room for header/footer
	}

	return s, nil
}

func (s OnboardingScreen) View() string {
	pad := lipgloss.NewStyle().Padding(1, 3)

	if s.step == stepWelcome {
		return pad.Render(s.welcomeView())
	}

	// Timeline sidebar
	timeline := s.renderTimeline()

	// Log + active content
	var logContent string
	for _, line := range s.log {
		logContent += "  " + line + "\n"
	}

	// Active input area (only for input steps)
	activeContent := ""
	switch s.step {
	case stepCheckGH, stepValidating, stepGenerateKeys, stepCloningRegistry, stepFetchingPools, stepSaving:
		activeContent = "\n  " + s.spinner.View() + " " + s.activeLabel()
	case stepAskToken:
		activeContent = "\n" + s.tokenInputCompact()
	case stepAskRegistry:
		activeContent = "\n" + s.registryInputCompact()
	case stepError:
		activeContent = "\n  " + theme.RedStyle.Render("✗ "+s.errMsg) + "\n  " + theme.DimStyle.Render("Press enter to retry")
	}

	mainContent := logContent + activeContent

	// Update viewport
	s.viewport.SetContent(mainContent)
	s.viewport.GotoBottom()

	// Layout: timeline | viewport
	left := lipgloss.NewStyle().Width(28).Render(timeline)
	right := s.viewport.View()

	return pad.Render(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
}

func (s OnboardingScreen) renderTimeline() string {
	out := theme.BoldStyle.Render("Setup") + "\n\n"

	for i, step := range s.timeline {
		icon := theme.DimStyle.Render("○")
		labelStyle := theme.DimStyle
		connector := theme.DimStyle.Render("│")

		switch step.state {
		case stateActive:
			icon = s.spinner.View()
			labelStyle = theme.TextStyle
		case stateDone:
			icon = theme.GreenStyle.Render("✓")
			labelStyle = theme.TextStyle
		case stateFailed:
			icon = theme.RedStyle.Render("✗")
			labelStyle = theme.RedStyle
		}

		out += fmt.Sprintf("  %s %s\n", icon, labelStyle.Render(step.label))
		if step.info != "" {
			out += fmt.Sprintf("  %s %s\n", connector, theme.DimStyle.Render(step.info))
		}
		if i < len(s.timeline)-1 {
			out += fmt.Sprintf("  %s\n", connector)
		}
	}

	return out
}

func (s OnboardingScreen) activeLabel() string {
	switch s.step {
	case stepCheckGH:
		return "Checking GitHub CLI..."
	case stepValidating:
		return "Validating token..."
	case stepGenerateKeys:
		return "Generating keypair..."
	case stepCloningRegistry:
		return "Cloning registry..."
	case stepFetchingPools:
		return "Discovering pools..."
	case stepSaving:
		return "Saving configuration..."
	}
	return ""
}

func (s OnboardingScreen) tokenInputCompact() string {
	out := "  " + theme.BoldStyle.Render("Enter GitHub token") + "\n"
	out += "  " + theme.DimStyle.Render("Create at: ") + theme.AccentStyle.Render("https://github.com/settings/tokens/new") + "\n"
	out += "  " + theme.DimStyle.Render("Scopes: ") + theme.GreenStyle.Render("repo") + theme.DimStyle.Render(", ") + theme.GreenStyle.Render("read:user") + "\n\n"
	out += "  " + s.input.View() + "\n"
	return out
}

func (s OnboardingScreen) registryInputCompact() string {
	out := "  " + theme.BoldStyle.Render("Enter pool registry") + "\n"
	out += "  " + theme.DimStyle.Render("Discover at: ") + theme.AccentStyle.Render("https://dating.dev/pools") + "\n"
	out += "  " + theme.DimStyle.Render("Example: ") + theme.TextStyle.Render("owner/registry") + theme.DimStyle.Render(" or full git URL") + "\n\n"
	out += "  " + s.input.View() + "\n"
	return out
}

func (s OnboardingScreen) welcomeView() string {
	title := theme.BoldStyle.Render("Welcome to dating.dev") + "\n\n"
	body := theme.TextStyle.Render("To get started, we need to set up:") + "\n\n"
	body += theme.TextStyle.Render("  1. ") + theme.AccentStyle.Render("GitHub token") + theme.DimStyle.Render("  — for identity & registration") + "\n"
	body += theme.TextStyle.Render("  2. ") + theme.AccentStyle.Render("Keypair") + theme.DimStyle.Render("       — for encryption & signing") + "\n"
	body += theme.TextStyle.Render("  3. ") + theme.AccentStyle.Render("Registry") + theme.DimStyle.Render("     — to discover dating pools") + "\n\n"
	body += theme.DimStyle.Render("Your token will be encrypted and stored locally.") + "\n"
	body += theme.DimStyle.Render("Your private key never leaves your machine.") + "\n\n"
	hint := theme.DimStyle.Render("Press enter to begin")
	return title + body + hint
}

// --- async commands ---

func (s OnboardingScreen) checkGH() tea.Msg {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ghCheckResult{err: err}
	}
	token := strings.TrimSpace(string(out))
	return ghCheckResult{token: token}
}

func (s OnboardingScreen) validateToken() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return tokenValidResult{err: fmt.Errorf("creating request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := onboardingHTTPClient.Do(req)
	if err != nil {
		return tokenValidResult{err: fmt.Errorf("network error: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return tokenValidResult{err: fmt.Errorf("invalid token — check scopes and expiry")}
	}
	if resp.StatusCode != 200 {
		return tokenValidResult{err: fmt.Errorf("GitHub API returned %d", resp.StatusCode)}
	}

	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return tokenValidResult{err: fmt.Errorf("parsing response: %w", err)}
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return tokenValidResult{
		userID:      fmt.Sprintf("%d", data.ID),
		username:    data.Login,
		displayName: name,
	}
}

func (s OnboardingScreen) generateKeys() tea.Msg {
	pub, _, err := crypto.LoadKeyPair(config.KeysDir())
	if err == nil {
		return keysResult{pub: pub}
	}
	pub, _, err = crypto.GenerateKeyPair(config.KeysDir())
	return keysResult{pub: pub, err: err}
}

func (s OnboardingScreen) cloneRegistry() tea.Msg {
	repoURL := gitrepo.EnsureGitURL(s.registryURL)
	_, err := gitrepo.CloneRegistry(repoURL)
	if err != nil {
		return registryCloneResult{err: err}
	}
	return registryCloneResult{repoURL: repoURL}
}

func (s OnboardingScreen) fetchPools() tea.Msg {
	reg, err := gh.CloneRegistry(s.registryURL)
	if err != nil {
		return poolsFetchResult{err: err}
	}
	pools, err := reg.ListPools()
	if err != nil {
		return poolsFetchResult{err: err}
	}
	return poolsFetchResult{pools: pools}
}

func (s OnboardingScreen) saveConfig() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return saveResult{err: err}
	}

	pub, _, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return saveResult{err: fmt.Errorf("loading keys: %w", err)}
	}

	encrypted, err := crypto.Encrypt(pub, []byte(s.token))
	if err != nil {
		return saveResult{err: fmt.Errorf("encrypting token: %w", err)}
	}

	cfg.User.DisplayName = s.displayName
	cfg.User.Username = s.username
	cfg.User.Provider = "github"
	cfg.User.ProviderUserID = s.userID
	cfg.User.EncryptedToken = hex.EncodeToString(encrypted)

	cfg.AddRegistry(s.registryURL)
	cfg.ActiveRegistry = s.registryURL

	return saveResult{err: cfg.Save()}
}

func (s OnboardingScreen) HelpBindings() []components.KeyBind {
	switch s.step {
	case stepAskToken, stepAskRegistry:
		return []components.KeyBind{
			{Key: "enter", Desc: "submit"},
			{Key: "esc", Desc: "quit"},
		}
	default:
		return []components.KeyBind{
			{Key: "enter", Desc: "continue"},
		}
	}
}
