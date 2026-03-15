package screens

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

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

// OnboardingDoneMsg is sent when onboarding completes successfully.
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
	userID      string
	username    string
	displayName string
	err         error
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

type OnboardingScreen struct {
	step     onboardingStep
	spinner  spinner.Model
	input    textinput.Model
	Width    int
	Height   int
	errMsg   string
	errRetry onboardingStep // which step to retry on error

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

	return OnboardingScreen{
		step:    stepWelcome,
		spinner: sp,
		input:   ti,
	}
}

func (s OnboardingScreen) Update(msg tea.Msg) (OnboardingScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch s.step {
		case stepWelcome:
			if msg.String() == "enter" {
				s.step = stepCheckGH
				return s, tea.Batch(s.checkGH, s.spinner.Tick)
			}

		case stepAskToken:
			if msg.String() == "enter" {
				val := strings.TrimSpace(s.input.Value())
				if val != "" {
					s.token = val
					s.step = stepValidating
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
			return s, tea.Batch(s.validateToken, s.spinner.Tick)
		}
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
			return s, nil
		}
		s.userID = msg.userID
		s.username = msg.username
		s.displayName = msg.displayName
		s.step = stepGenerateKeys
		return s, tea.Batch(s.generateKeys, s.spinner.Tick)

	case keysResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskToken
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.pubKey = msg.pub
		// Move to registry setup
		s.step = stepAskRegistry
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
			return s, nil
		}
		s.registryURL = msg.repoURL
		s.step = stepFetchingPools
		return s, tea.Batch(s.fetchPools, s.spinner.Tick)

	case poolsFetchResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskRegistry
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.pools = msg.pools
		s.step = stepSaving
		return s, tea.Batch(s.saveConfig, s.spinner.Tick)

	case saveResult:
		if msg.err != nil {
			s.step = stepError
			s.errRetry = stepAskRegistry
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.step = stepDone
		return s, nil

	case tea.WindowSizeMsg:
		s.Width = msg.Width
		s.Height = msg.Height
	}

	return s, nil
}

func (s OnboardingScreen) View() string {
	pad := lipgloss.NewStyle().Padding(1, 3)

	// Step indicator
	totalSteps := 4
	currentStep := 0
	stepLabel := ""
	switch s.step {
	case stepWelcome:
		return pad.Render(s.welcomeView())
	case stepCheckGH, stepAskToken, stepValidating:
		currentStep = 1
		stepLabel = "GitHub Authentication"
	case stepGenerateKeys:
		currentStep = 2
		stepLabel = "Generate Keys"
	case stepAskRegistry, stepCloningRegistry, stepFetchingPools:
		currentStep = 3
		stepLabel = "Registry Setup"
	case stepSaving:
		currentStep = 4
		stepLabel = "Saving"
	case stepDone:
		return pad.Render(s.doneView())
	case stepError:
		return pad.Render(s.errorView())
	}

	progress := theme.DimStyle.Render(fmt.Sprintf("Step %d/%d", currentStep, totalSteps)) +
		"  " + theme.BoldStyle.Render(stepLabel) + "\n" +
		renderProgressBar(currentStep, totalSteps, s.Width-8) + "\n\n"

	var content string
	switch s.step {
	case stepCheckGH:
		content = fmt.Sprintf("%s Checking for GitHub CLI...", s.spinner.View())
	case stepAskToken:
		content = s.tokenInputView()
	case stepValidating:
		content = fmt.Sprintf("%s Validating token...\n\n", s.spinner.View()) +
			theme.DimStyle.Render("  Fetching your GitHub identity")
	case stepGenerateKeys:
		content = fmt.Sprintf("%s Generating ed25519 keypair...", s.spinner.View())
	case stepAskRegistry:
		content = s.registryInputView()
	case stepCloningRegistry:
		content = fmt.Sprintf("%s Cloning registry...", s.spinner.View())
	case stepFetchingPools:
		content = fmt.Sprintf("%s Discovering pools...", s.spinner.View())
	case stepSaving:
		content = fmt.Sprintf("%s Encrypting token & saving configuration...", s.spinner.View())
	}

	return pad.Render(progress + content)
}

func renderProgressBar(current, total, width int) string {
	if width < 10 {
		width = 40
	}
	filled := width * current / total
	empty := width - filled

	bar := theme.BrandStyle.Render(strings.Repeat("█", filled)) +
		theme.DimStyle.Render(strings.Repeat("░", empty))
	return bar
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

func (s OnboardingScreen) tokenInputView() string {
	title := theme.BoldStyle.Render("GitHub Personal Access Token") + "\n\n"

	guide := theme.TextStyle.Render("Create a token at:") + "\n"
	guide += theme.AccentStyle.Render("  https://github.com/settings/tokens/new") + "\n\n"
	guide += theme.DimStyle.Render("Required scopes:") + "\n"
	guide += theme.TextStyle.Render("  - ") + theme.GreenStyle.Render("repo") + theme.DimStyle.Render("       — create issues for registration") + "\n"
	guide += theme.TextStyle.Render("  - ") + theme.GreenStyle.Render("read:user") + theme.DimStyle.Render("  — verify your identity") + "\n\n"
	guide += theme.DimStyle.Render("Or run: ") + theme.TextStyle.Render("gh auth login") +
		theme.DimStyle.Render("  (we'll detect it automatically next time)") + "\n\n"

	input := s.input.View() + "\n\n"
	hint := theme.DimStyle.Render("Paste your token and press enter")

	return title + guide + input + hint
}

func (s OnboardingScreen) registryInputView() string {
	title := theme.GreenStyle.Render("✓") + " " +
		theme.BoldStyle.Render(fmt.Sprintf("Authenticated as %s (@%s)", s.displayName, s.username)) + "\n\n"

	title += theme.BoldStyle.Render("Pool Registry") + "\n\n"

	guide := theme.TextStyle.Render("A registry is a directory of dating pools.") + "\n"
	guide += theme.TextStyle.Render("Enter a registry to discover and join pools.") + "\n\n"
	guide += theme.DimStyle.Render("Discover registries at: ") +
		theme.AccentStyle.Render("https://dating.dev/pools") + "\n\n"
	guide += theme.DimStyle.Render("Examples:") + "\n"
	guide += theme.TextStyle.Render("  vutran1710/dating-test-registry") +
		theme.DimStyle.Render("         — GitHub shorthand") + "\n"
	guide += theme.TextStyle.Render("  https://github.com/owner/registry") +
		theme.DimStyle.Render("    — full URL") + "\n"
	guide += theme.TextStyle.Render("  git@gitlab.com:owner/registry.git") +
		theme.DimStyle.Render("    — any git host") + "\n\n"

	input := s.input.View() + "\n\n"
	hint := theme.DimStyle.Render("Enter registry and press enter")

	return title + guide + input + hint
}

func (s OnboardingScreen) doneView() string {
	title := theme.GreenStyle.Render("✓") + " " + theme.BoldStyle.Render("Setup complete") + "\n\n"

	info := ""
	info += theme.DimStyle.Render("  Name       ") + theme.TextStyle.Render(s.displayName) + "\n"
	info += theme.DimStyle.Render("  GitHub     ") + theme.AccentStyle.Render("@"+s.username) + "\n"
	if s.pubKey != nil && len(s.pubKey) >= 8 {
		info += theme.DimStyle.Render("  Key        ") + theme.DimStyle.Render(hex.EncodeToString(s.pubKey[:8])+"...") + "\n"
	}
	info += theme.DimStyle.Render("  Token      ") + theme.GreenStyle.Render("encrypted & saved") + "\n"
	info += theme.DimStyle.Render("  Registry   ") + theme.TextStyle.Render(s.registryURL) + "\n"

	if len(s.pools) > 0 {
		info += "\n" + theme.DimStyle.Render("  Pools found:") + "\n"
		for _, p := range s.pools {
			info += theme.TextStyle.Render("    - ") + theme.AccentStyle.Render(p.Name)
			if p.Description != "" {
				info += theme.DimStyle.Render("  "+p.Description)
			}
			info += "\n"
		}
	}

	info += "\n"
	hint := theme.DimStyle.Render("Press enter to continue")
	return title + info + hint
}

func (s OnboardingScreen) errorView() string {
	title := theme.RedStyle.Render("✗") + " " + theme.BoldStyle.Render("Error") + "\n\n"
	body := theme.RedStyle.Render("  " + s.errMsg) + "\n\n"
	hint := theme.DimStyle.Render("Press enter to try again")
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
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := http.DefaultClient.Do(req)
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

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
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
	input := s.registryURL

	// Normalize: owner/repo → https://github.com/owner/repo.git
	repoURL := gitrepo.EnsureGitURL(input)

	// Validate by cloning
	_, err := gitrepo.Clone(repoURL)
	if err != nil {
		return registryCloneResult{err: fmt.Errorf("cannot access registry: %s", repoURL)}
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

	// Encrypt token with user's public key
	pub, _, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return saveResult{err: fmt.Errorf("loading keys: %w", err)}
	}

	encrypted, err := crypto.Encrypt(pub, []byte(s.token))
	if err != nil {
		return saveResult{err: fmt.Errorf("encrypting token: %w", err)}
	}

	cfg.User.DisplayName = s.displayName
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
