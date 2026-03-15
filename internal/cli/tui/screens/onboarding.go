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
)

type onboardingStep int

const (
	stepWelcome onboardingStep = iota
	stepCheckGH
	stepAskToken
	stepValidating
	stepGenerateKeys
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

	// collected data
	token       string
	userID      string
	username    string
	displayName string
	pubKey      ed25519.PublicKey
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

		case stepError:
			if msg.String() == "enter" {
				s.step = stepAskToken
				s.input.SetValue("")
				s.input.Focus()
				s.errMsg = ""
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
		// gh not available, ask manually
		s.step = stepAskToken
		s.input.Focus()
		return s, textinput.Blink

	case tokenValidResult:
		if msg.err != nil {
			s.step = stepError
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
			s.errMsg = msg.err.Error()
			return s, nil
		}
		s.pubKey = msg.pub
		s.step = stepSaving
		return s, tea.Batch(s.saveConfig, s.spinner.Tick)

	case saveResult:
		if msg.err != nil {
			s.step = stepError
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

	switch s.step {
	case stepWelcome:
		return pad.Render(s.welcomeView())
	case stepCheckGH:
		return pad.Render(fmt.Sprintf("%s Checking for GitHub CLI...", s.spinner.View()))
	case stepAskToken:
		return pad.Render(s.tokenInputView())
	case stepValidating:
		return pad.Render(fmt.Sprintf("%s Validating token...", s.spinner.View()))
	case stepGenerateKeys:
		return pad.Render(fmt.Sprintf("%s Generating keypair...", s.spinner.View()))
	case stepSaving:
		return pad.Render(fmt.Sprintf("%s Saving configuration...", s.spinner.View()))
	case stepDone:
		return pad.Render(s.doneView())
	case stepError:
		return pad.Render(s.errorView())
	}
	return ""
}

func (s OnboardingScreen) welcomeView() string {
	title := theme.BoldStyle.Render("Welcome to dating.dev") + "\n\n"
	body := theme.TextStyle.Render("To get started, we need to set up your identity.") + "\n"
	body += theme.TextStyle.Render("This requires a GitHub account and a personal access token.") + "\n\n"
	body += theme.DimStyle.Render("We'll try to detect your GitHub CLI token automatically.") + "\n"
	body += theme.DimStyle.Render("If that doesn't work, you'll need to create a token manually.") + "\n\n"
	hint := theme.DimStyle.Render("Press enter to continue")
	return title + body + hint
}

func (s OnboardingScreen) tokenInputView() string {
	title := theme.BoldStyle.Render("GitHub Personal Access Token") + "\n\n"

	guide := theme.TextStyle.Render("Create a token at:") + "\n"
	guide += theme.AccentStyle.Render("  https://github.com/settings/tokens/new") + "\n\n"
	guide += theme.DimStyle.Render("Required scopes:") + "\n"
	guide += theme.TextStyle.Render("  - ") + theme.GreenStyle.Render("repo") + theme.DimStyle.Render(" (to create issues for registration)") + "\n"
	guide += theme.TextStyle.Render("  - ") + theme.GreenStyle.Render("read:user") + theme.DimStyle.Render(" (to verify your identity)") + "\n\n"
	guide += theme.DimStyle.Render("Or install GitHub CLI and run: ") + theme.TextStyle.Render("gh auth login") + "\n\n"

	input := s.input.View() + "\n\n"
	hint := theme.DimStyle.Render("Paste your token and press enter")

	return title + guide + input + hint
}

func (s OnboardingScreen) doneView() string {
	title := theme.GreenStyle.Render("✓") + " " + theme.BoldStyle.Render("Setup complete") + "\n\n"

	info := ""
	info += theme.DimStyle.Render("  Name     ") + theme.TextStyle.Render(s.displayName) + "\n"
	info += theme.DimStyle.Render("  GitHub   ") + theme.AccentStyle.Render("@"+s.username) + "\n"
	info += theme.DimStyle.Render("  Key      ") + theme.DimStyle.Render(hex.EncodeToString(s.pubKey[:8])+"...") + "\n"
	info += theme.DimStyle.Render("  Token    ") + theme.GreenStyle.Render("encrypted & saved") + "\n\n"

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
	// Load existing or generate new
	pub, _, err := crypto.LoadKeyPair(config.KeysDir())
	if err == nil {
		return keysResult{pub: pub}
	}

	pub, _, err = crypto.GenerateKeyPair(config.KeysDir())
	return keysResult{pub: pub, err: err}
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

	return saveResult{err: cfg.Save()}
}

func (s OnboardingScreen) HelpBindings() []components.KeyBind {
	switch s.step {
	case stepAskToken:
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
