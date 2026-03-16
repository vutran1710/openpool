package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/screens"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

type activeScreen int

const (
	screenOnboarding activeScreen = iota
	screenHome
	screenDiscover
	screenMatches
	screenChat
	screenPools
	screenJoin
	screenProfile
)

type app struct {
	screen     activeScreen
	prevScreen activeScreen
	width      int
	height     int
	quitting   bool

	statusBar  components.StatusBar
	input      components.Input
	toast      components.Toast
	helpBar    components.HelpBar

	onboarding screens.OnboardingScreen
	home       screens.HomeScreen
	discover   screens.DiscoverScreen
	matches    screens.MatchesScreen
	chat       screens.ChatScreen
	pools      screens.PoolsScreen
	join       screens.JoinScreen
	profile    screens.ProfileScreen

	user     string
	pool     string
	registry string
}

func newApp(userName, userHash, pool, registry string, poolStatuses map[string]string, poolIssues map[string]int, needsOnboarding bool) app {
	startScreen := activeScreen(screenHome)
	if needsOnboarding {
		startScreen = screenOnboarding
	}

	a := app{
		screen:     startScreen,
		user:       userName,
		pool:       pool,
		registry:   registry,
		statusBar:  components.NewStatusBar(),
		input:      components.NewInput("Type / for commands..."),
		toast:      components.NewToast(),
		onboarding: screens.NewOnboardingScreen(),
		home:       screens.NewHomeScreen(),
		discover:   screens.NewDiscoverScreen(),
		matches:    screens.NewMatchesScreen(),
		pools:      screens.NewPoolsScreen(registry, poolStatuses, poolIssues),
		profile:    screens.NewProfileScreen(),
	}
	a.statusBar.User = userName
	a.statusBar.UserHash = userHash
	a.statusBar.Pool = pool
	a.updateHelp()
	return a
}

func (a app) Init() tea.Cmd {
	cmds := []tea.Cmd{inputInit(), a.statusBar.Heart.Tick()}
	// Start background polling for pending pools
	cmds = append(cmds, tea.Tick(10*time.Second, func(time.Time) tea.Msg {
		return pendingPollTickMsg{}
	}))
	return tea.Batch(cmds...)
}

func (a *app) updateHelp() {
	var bindings []components.KeyBind
	switch a.screen {
	case screenOnboarding:
		bindings = a.onboarding.HelpBindings()
	case screenHome:
		bindings = a.home.HelpBindings()
	case screenDiscover:
		bindings = a.discover.HelpBindings()
	case screenMatches:
		bindings = a.matches.HelpBindings()
	case screenChat:
		bindings = a.chat.HelpBindings()
	case screenPools:
		bindings = a.pools.HelpBindings()
	case screenJoin:
		bindings = a.join.HelpBindings()
	case screenProfile:
		bindings = a.profile.HelpBindings()
	}
	a.helpBar = components.NewHelpBar(bindings...)
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.statusBar.Width = msg.Width
		a.input.Width = msg.Width
		a.helpBar.Width = msg.Width
		a.discover.Width = msg.Width
		a.pools.Width = msg.Width
		a.pools.Height = msg.Height
		a.onboarding.Width = msg.Width
		a.onboarding.Height = msg.Height
		a.profile.Width = msg.Width
		a.profile.Height = msg.Height
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		case "esc":
			if a.screen == screenOnboarding {
				a.quitting = true
				return a, tea.Quit
			}
			if a.screen != screenHome {
				a.screen = screenHome
				a.updateHelp()
				return a, nil
			}
		}

	case screens.OnboardingDoneMsg:
		a.user = msg.DisplayName
		a.registry = msg.Registry
		a.statusBar.User = msg.DisplayName
		a.statusBar.UserHash = msg.Username
		a.pools = screens.NewPoolsScreen(msg.Registry, nil)
		a.pools.Width = a.width
		a.pools.Height = a.height
		a.screen = screenHome
		a.updateHelp()
		return a, func() tea.Msg {
			return components.ToastMsg{
				Text:  "Welcome, " + msg.DisplayName + "!",
				Level: components.ToastSuccess,
			}
		}

	case components.MenuSelectMsg:
		return a.handleMenuSelect(msg.Key)

	case components.SubmitMsg:
		if a.screen == screenOnboarding {
			return a.updateActiveScreen(msg)
		}
		return a.handleSubmit(msg)

	case components.ToastMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

	case screens.PoolJoinMsg:
		if msg.Status == "active" {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Already a member of " + msg.Name, Level: components.ToastInfo}
			}
		}
		if msg.Status == "pending" {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Registration for " + msg.Name + " is still being processed", Level: components.ToastInfo}
			}
		}
		// Launch join flow (for "" or "rejected")
		cfg, _ := config.Load()
		username := ""
		userID := ""
		if cfg != nil {
			username = cfg.User.Username
			if username == "" {
				username = cfg.User.DisplayName // fallback for old configs
			}
			userID = cfg.User.ProviderUserID
		}
		// Find pool entry from the pools screen (fetched from registry)
		opKey := ""
		relayURL := ""
		poolRepo := ""
		for _, p := range a.pools.GetPools() {
			if p.Name == msg.Name {
				opKey = p.OperatorPubKey
				relayURL = p.RelayURL
				poolRepo = p.Repo
				break
			}
		}
		// Fallback: check local config (for rejoin)
		if poolRepo == "" && cfg != nil {
			for _, p := range cfg.Pools {
				if p.Name == msg.Name {
					opKey = p.OperatorPubKey
					relayURL = p.RelayURL
					poolRepo = p.Repo
					break
				}
			}
		}
		if poolRepo == "" {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Pool not found: " + msg.Name, Level: components.ToastError}
			}
		}
		a.join = screens.NewJoinScreen(msg.Name, poolRepo, opKey, relayURL, username, userID)
		a.join.Width = a.width
		a.join.Height = a.height
		a.screen = screenJoin
		a.updateHelp()
		return a, nil

	case screens.JoinDoneMsg:
		if msg.PoolName != "" {
			a.pool = msg.PoolName
			a.statusBar.Pool = msg.PoolName
			// Refresh pools screen with pending status
			cfg, _ := config.Load()
			poolStatuses := make(map[string]string)
			if cfg != nil {
				for _, p := range cfg.Pools {
					s := p.Status
					if s == "" {
						s = "active"
					}
					poolStatuses[p.Name] = s
				}
			}
			a.pools = screens.NewPoolsScreen(a.registry, poolStatuses)
			a.pools.Width = a.width
			a.pools.Height = a.height
		}
		a.screen = screenHome
		a.updateHelp()
		if msg.PoolName != "" {
			return a, func() tea.Msg {
				return components.ToastMsg{
					Text:  "Registration submitted for " + msg.PoolName + " — we'll notify you when it's processed",
					Level: components.ToastInfo,
				}
			}
		}
		return a, nil

	case pendingPollResultMsg:
		if msg.poolName != "" {
			// Refresh pools screen with updated statuses
			cfg, _ := config.Load()
			if cfg != nil {
				ps := make(map[string]string)
				pi := make(map[string]int)
				for _, p := range cfg.Pools {
					s := p.Status
					if s == "" {
						s = "active"
					}
					ps[p.Name] = s
					if p.PendingIssue > 0 {
						pi[p.Name] = p.PendingIssue
					}
				}
				a.pools = screens.NewPoolsScreen(a.registry, ps, pi)
				a.pools.Width = a.width
				a.pools.Height = a.height

				// If currently on pools screen, trigger re-fetch
				if a.screen == screenPools {
					var cmd tea.Cmd
					a.pools, cmd = a.pools.Update(screens.PoolsInitMsg{})
					if cmd != nil {
						return a, tea.Batch(cmd, func() tea.Msg {
							return components.ToastMsg{
								Text:  "Registration " + msg.status + " for " + msg.poolName,
								Level: components.ToastSuccess,
							}
						})
					}
				}
			}

			if msg.status == "active" {
				return a, func() tea.Msg {
					return components.ToastMsg{
						Text:  "Registration accepted for " + msg.poolName + "!",
						Level: components.ToastSuccess,
					}
				}
			} else if msg.status == "rejected" {
				return a, func() tea.Msg {
					return components.ToastMsg{
						Text:  "Registration rejected for " + msg.poolName,
						Level: components.ToastError,
					}
				}
			}
		}
		// Schedule next poll
		return a, tea.Tick(30*time.Second, func(time.Time) tea.Msg {
			return pendingPollTickMsg{}
		})

	case pendingPollTickMsg:
		return a, pollPendingPools

	case components.HeartTickMsg:
		var cmd tea.Cmd
		a.statusBar.Heart, cmd = a.statusBar.Heart.Update(msg)
		return a, cmd

	case components.ToastClearMsg:
		a.toast, _ = a.toast.Update(msg)
		return a, nil
	}

	return a.updateActiveScreen(msg)
}

func (a app) handleMenuSelect(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "discover":
		a.screen = screenDiscover
	case "matches":
		a.screen = screenMatches
	case "pools":
		a.screen = screenPools
		a.updateHelp()
		// Trigger fetch if not yet loaded
		if !a.pools.IsLoaded() {
			var cmd tea.Cmd
			a.pools, cmd = a.pools.Update(screens.PoolsInitMsg{})
			return a, cmd
		}
		return a, nil
	case "inbox":
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Inbox coming soon", Level: components.ToastInfo}
		}
	case "profile":
		a.screen = screenProfile
		a.profile.Width = a.width
		a.profile.Height = a.height
		if !a.profile.IsLoaded() {
			return a, a.profile.LoadCmd
		}
	case "auth":
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Run: dating auth register", Level: components.ToastInfo}
		}
	default:
		if a.screen == screenMatches {
			a.chat = screens.NewChatScreen(key, a.width, a.height)
			a.screen = screenChat
		}
	}
	a.updateHelp()
	return a, nil
}

func (a app) handleSubmit(msg components.SubmitMsg) (tea.Model, tea.Cmd) {
	if msg.IsCommand {
		switch msg.Value {
		case "/quit", "/q":
			a.quitting = true
			return a, tea.Quit
		case "/home":
			a.screen = screenHome
			a.updateHelp()
		case "/discover", "/fetch":
			a.screen = screenDiscover
			a.updateHelp()
		case "/matches":
			a.screen = screenMatches
			a.updateHelp()
		case "/pools":
			a.screen = screenPools
			a.updateHelp()
		case "/profile":
			a.screen = screenProfile
			a.profile.Width = a.width
			a.profile.Height = a.height
			a.updateHelp()
		case "/exit":
			if a.screen == screenChat {
				a.screen = screenMatches
				a.updateHelp()
			}
		default:
			return a, func() tea.Msg {
				return components.ToastMsg{
					Text:  "Unknown command: " + msg.Value,
					Level: components.ToastWarning,
				}
			}
		}
		return a, nil
	}

	if a.screen == screenChat {
		var cmd tea.Cmd
		a.chat, cmd = a.chat.Update(msg)
		return a, cmd
	}

	return a, nil
}

func (a app) updateActiveScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch a.screen {
	case screenOnboarding:
		a.onboarding, cmd = a.onboarding.Update(msg)
		a.updateHelp()
	case screenHome:
		a.home, cmd = a.home.Update(msg)
	case screenDiscover:
		a.discover, cmd = a.discover.Update(msg)
	case screenMatches:
		a.matches, cmd = a.matches.Update(msg)
	case screenChat:
		a.chat, cmd = a.chat.Update(msg)
	case screenPools:
		a.pools, cmd = a.pools.Update(msg)
	case screenJoin:
		a.join, cmd = a.join.Update(msg)
		a.updateHelp()
	case screenProfile:
		a.profile, cmd = a.profile.Update(msg)
	}

	if cmd != nil {
		// Don't forward to input during onboarding (it steals key events)
		if a.screen == screenOnboarding || a.screen == screenJoin || a.screen == screenProfile {
			return a, cmd
		}
		var inputCmd tea.Cmd
		a.input, inputCmd = a.input.Update(msg)
		return a, tea.Batch(cmd, inputCmd)
	}

	if a.screen == screenOnboarding || a.screen == screenJoin || a.screen == screenProfile {
		return a, nil
	}

	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

func (a app) View() string {
	if a.quitting {
		return ""
	}

	top := a.statusBar.View()

	var content string
	switch a.screen {
	case screenOnboarding:
		content = a.onboarding.View()
	case screenHome:
		content = a.home.View()
	case screenDiscover:
		content = a.discover.View()
	case screenMatches:
		content = a.matches.View()
	case screenChat:
		content = a.chat.View()
	case screenPools:
		content = a.pools.View()
	case screenJoin:
		content = a.join.View()
	case screenProfile:
		content = a.profile.View()
	}

	toastView := a.toast.View()

	// During onboarding, hide the command input
	bottom := ""
	if a.screen != screenOnboarding && a.screen != screenJoin {
		palette := a.input.PaletteView()
		if palette != "" {
			bottom += palette + "\n"
		}
		bottom += a.helpBar.View() + "\n" + a.input.View()
	} else {
		bottom += a.helpBar.View()
	}

	// Status toast below input
	if toastView != "" {
		bottom += "\n" + toastView
	}

	contentHeight := a.height - 2 - countLines(top) - countLines(bottom)
	if contentHeight < 1 {
		contentHeight = 1
	}

	content = padToHeight(content, contentHeight)

	out := top + "\n" + content + "\n" + bottom

	return out
}

func countLines(s string) int {
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

// --- background polling for pending pools ---

type pendingPollTickMsg struct{}

type pendingPollResultMsg struct {
	poolName string
	status   string // "active", "rejected", ""
}

func pollPendingPools() tea.Msg {
	cfg, err := config.Load()
	if err != nil {
		return pendingPollResultMsg{}
	}

	_, priv, err := crypto.LoadKeyPair(config.KeysDir())
	if err != nil {
		return pendingPollResultMsg{}
	}

	token, err := cfg.DecryptToken(priv)
	if err != nil {
		return pendingPollResultMsg{}
	}

	for i, p := range cfg.Pools {
		if p.Status != "pending" || p.PendingIssue == 0 {
			continue
		}

		// Poll the actual issue status
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", p.Repo, p.PendingIssue)
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		var issue struct {
			State       string `json:"state"`
			StateReason string `json:"state_reason"`
		}
		json.NewDecoder(resp.Body).Decode(&issue)
		resp.Body.Close()

		if issue.State == "closed" {
			if issue.StateReason == "completed" {
				cfg.Pools[i].Status = "active"
				cfg.Pools[i].PendingIssue = 0
				cfg.Save()
				return pendingPollResultMsg{poolName: p.Name, status: "active"}
			}
			// Rejected
			cfg.Pools[i].Status = "rejected"
			cfg.Pools[i].PendingIssue = 0
			cfg.Save()
			return pendingPollResultMsg{poolName: p.Name, status: "rejected"}
		}
		// Still open — keep polling
	}

	return pendingPollResultMsg{}
}

func padToHeight(s string, height int) string {
	lines := countLines(s)
	for lines < height {
		s += "\n"
		lines++
	}
	return s
}
