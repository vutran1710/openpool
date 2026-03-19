package tui

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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/svc"
	"github.com/vutran1710/dating-dev/internal/cli/tui/screens"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/crypto"
	dbg "github.com/vutran1710/dating-dev/internal/debug"
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
	screenProfileForm
	screenSettings
	screenInbox
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
	profile     screens.ProfileScreen
	profileForm screens.ProfileFormScreen
	settings    screens.SettingsScreen
	inbox      screens.InboxScreen

	user     string
	pool     string
	registry string
}

func newApp(userName, userHash, pool, registry string, poolStatuses map[string]string, poolIssues map[string]int, needsOnboarding bool) app {
	startScreen := activeScreen(screenHome)
	if needsOnboarding {
		startScreen = screenOnboarding
	} else if pool == "" {
		startScreen = screenPools
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
		settings:   screens.NewSettingsScreen(pool, registry, userName, "", poolNames(poolStatuses), registries(registry)),
		inbox:      screens.NewInboxScreen(),
	}
	a.statusBar.User = userName
	a.statusBar.UserHash = userHash
	a.statusBar.Pool = pool
	a.statusBar.Registry = registry
	a.updateHelp()
	return a
}

func (a app) Init() tea.Cmd {
	cmds := []tea.Cmd{inputInit(), a.statusBar.Heart.Tick()}
	// Hint when starting with no active pool
	if a.pool == "" && a.screen == screenPools {
		cmds = append(cmds, func() tea.Msg {
			return components.ToastMsg{Text: "Join or activate a pool to get started", Level: components.ToastInfo}
		})
	}
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
	case screenProfileForm:
		bindings = a.profileForm.HelpBindings()
	case screenSettings:
		bindings = a.settings.HelpBindings()
	case screenInbox:
		bindings = a.inbox.HelpBindings()
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
		a.settings.Width = msg.Width
		a.settings.Height = msg.Height
		a.inbox.Width = msg.Width
		a.inbox.Height = msg.Height
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
		ps, pi := poolStatusesFromConfig()
		a.pools = screens.NewPoolsScreen(msg.Registry, ps, pi)
		a.pools.Width = a.width
		a.pools.Height = a.height
		// Go to pools screen — user needs to join a pool first
		a.screen = screenPools
		a.updateHelp()
		return a, func() tea.Msg {
			return components.ToastMsg{
				Text:  "Welcome! Join a pool to get started.",
				Level: components.ToastSuccess,
			}
		}

	case profileSubmitResultMsg:
		if msg.err != nil {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Profile update failed: " + msg.err.Error(), Level: components.ToastError}
			}
		}
		return a, func() tea.Msg {
			return components.ToastMsg{
				Text:  fmt.Sprintf("Profile updated (Issue #%d) — waiting for pool to process", msg.issueNum),
				Level: components.ToastSuccess,
			}
		}

	case screens.ProfileUpdateMsg:
		// Submit updated profile to pool
		return a, submitProfileUpdate(msg.Profile)

	case screens.InboxAcceptMsg:
		return a, acceptLike(a.pool, msg.PRNumber)

	case screens.InboxRejectMsg:
		return a, rejectLike(a.pool, msg.PRNumber)

	case screens.PoolSwitchMsg:
		a.pool = msg.Name
		a.statusBar.Pool = msg.Name
		// Save to config
		if cfg, err := config.Load(); err == nil {
			cfg.Active = msg.Name
			cfg.Save()
		}
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Switched to pool: " + msg.Name, Level: components.ToastSuccess}
		}

	case screens.RegistrySwitchMsg:
		a.registry = msg.Repo
		a.statusBar.Registry = msg.Repo
		// Save to config
		if cfg, err := config.Load(); err == nil {
			cfg.ActiveRegistry = msg.Repo
			cfg.Save()
		}
		// Refresh pools screen with new registry
		ps, pi := poolStatusesFromConfig()
		a.pools = screens.NewPoolsScreen(msg.Repo, ps, pi)
		a.pools.Width = a.width
		a.pools.Height = a.height
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Switched to registry: " + msg.Repo, Level: components.ToastSuccess}
		}

	case noActivePoolMsg:
		a.screen = screenPools
		a.updateHelp()
		if !a.pools.IsLoaded() {
			var cmd tea.Cmd
			a.pools, cmd = a.pools.Update(screens.PoolsInitMsg{})
			return a, tea.Batch(cmd, func() tea.Msg {
				return components.ToastMsg{Text: "No active pool — join or activate one", Level: components.ToastInfo}
			})
		}
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "No active pool — join or activate one", Level: components.ToastInfo}
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

	case screens.DiscoverLikeMsg:
		return a, sendLike(a.pool, a.registry, msg.TargetMatchHash)

	case screens.MatchChatMsg:
		a.screen = screenChat
		a.chat = screens.NewChatScreen(msg.BinHash, a.width, a.height)
		a.updateHelp()
		return a, screens.ConnectChatCmd(msg.BinHash)

	case screens.PoolJoinMsg:
		if msg.Status == "active" {
			// Activate this pool (set as active)
			return a, func() tea.Msg {
				return screens.PoolSwitchMsg{Name: msg.Name}
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
			ps2, pi2 := poolStatusesFromConfig()
			a.pools = screens.NewPoolsScreen(a.registry, ps2, pi2)
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
		a.discover = screens.NewDiscoverScreen()
		return a, screens.LoadDiscoverCmd(a.pool)
	case "matches":
		a.screen = screenMatches
		a.matches = screens.MatchesScreen{Loading: true}
		return a, screens.LoadMatchesCmd(a.pool)
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
		a.screen = screenInbox
		a.inbox = screens.NewInboxScreen()
		a.inbox.Width = a.width
		a.inbox.Height = a.height
		a.updateHelp()
		// Trigger fetch
		return a, fetchInbox(a.pool, a.registry)
	case "profile":
		dbg.Log("navigate → profile")
		a.screen = screenProfile
		// Send WindowSizeMsg to initialize viewport dimensions
		a.profile, _ = a.profile.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		if !a.profile.IsLoaded() {
			return a, a.profile.LoadCmd
		}
	case "settings":
		a.screen = screenSettings
		a.settings.Width = a.width
		a.settings.Height = a.height
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
			if a.pool == "" {
				return a, func() tea.Msg { return noActivePoolMsg{} }
			}
			a.screen = screenHome
			a.updateHelp()
		case "/discover", "/fetch":
			a.screen = screenDiscover
			a.discover = screens.NewDiscoverScreen()
			a.updateHelp()
			return a, screens.LoadDiscoverCmd(a.pool)
		case "/matches":
			a.screen = screenMatches
			a.matches = screens.MatchesScreen{Loading: true}
			a.updateHelp()
			return a, screens.LoadMatchesCmd(a.pool)
		case "/pools":
			a.screen = screenPools
			a.updateHelp()
		case "/profile":
			a.screen = screenProfile
			a.profile, _ = a.profile.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			a.updateHelp()
		case "/profile-edit", "/edit":
			a.screen = screenProfileForm
			a.profileForm = screens.NewProfileFormScreen()
			a.updateHelp()
			return a, screens.LoadProfileFormCmd(a.pool)
		case "/inbox":
			a.screen = screenInbox
			a.inbox = screens.NewInboxScreen()
			a.inbox.Width = a.width
			a.inbox.Height = a.height
			a.updateHelp()
			return a, fetchInbox(a.pool, a.registry)
		case "/settings":
			a.screen = screenSettings
			a.settings.Width = a.width
			a.settings.Height = a.height
			a.updateHelp()
		case "/exit":
			if a.screen == screenChat {
				a.screen = screenMatches
				a.updateHelp()
			}
		default:
			// Handle /pool <name> and /registry <name>
			if strings.HasPrefix(msg.Value, "/pool ") {
				name := strings.TrimSpace(strings.TrimPrefix(msg.Value, "/pool "))
				if name != "" {
					// Check pool exists in config
					cfg, _ := config.Load()
					found := false
					if cfg != nil {
						for _, p := range cfg.Pools {
							if p.Name == name && (p.Status == "active" || p.Status == "") {
								found = true
								break
							}
						}
					}
					if found {
						return a, func() tea.Msg { return screens.PoolSwitchMsg{Name: name} }
					}
					return a, func() tea.Msg {
						return components.ToastMsg{Text: "Pool not found or not active: " + name, Level: components.ToastError}
					}
				}
			}
			if strings.HasPrefix(msg.Value, "/registry ") {
				repo := strings.TrimSpace(strings.TrimPrefix(msg.Value, "/registry "))
				if repo != "" {
					cfg, _ := config.Load()
					found := false
					if cfg != nil {
						for _, r := range cfg.Registries {
							if r == repo {
								found = true
								break
							}
						}
					}
					if found {
						return a, func() tea.Msg { return screens.RegistrySwitchMsg{Repo: repo} }
					}
					return a, func() tea.Msg {
						return components.ToastMsg{Text: "Registry not found: " + repo, Level: components.ToastError}
					}
				}
			}
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
	case screenProfileForm:
		a.profileForm, cmd = a.profileForm.Update(msg)
	case screenSettings:
		a.settings, cmd = a.settings.Update(msg)
	case screenInbox:
		a.inbox, cmd = a.inbox.Update(msg)
	}

	if cmd != nil {
		// Don't forward to input during onboarding (it steals key events)
		if a.screen == screenOnboarding || a.screen == screenJoin || a.screen == screenProfile || a.screen == screenProfileForm || a.screen == screenSettings || a.screen == screenInbox || a.screen == screenDiscover {
			return a, cmd
		}
		var inputCmd tea.Cmd
		a.input, inputCmd = a.input.Update(msg)
		return a, tea.Batch(cmd, inputCmd)
	}

	if a.screen == screenOnboarding || a.screen == screenJoin || a.screen == screenProfile || a.screen == screenProfileForm || a.screen == screenSettings || a.screen == screenInbox || a.screen == screenDiscover {
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
	case screenProfileForm:
		content = a.profileForm.View()
	case screenSettings:
		content = a.settings.View()
	case screenInbox:
		content = a.inbox.View()
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

	// Debug log (only with DEBUG=1)
	if debugView := dbg.View(3); debugView != "" {
		bottom += "\n" + theme.DimStyle.Render(debugView)
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

type noActivePoolMsg struct{}

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
			// Issue not found or API error — remove stale pool entry
			cfg.Pools = append(cfg.Pools[:i], cfg.Pools[i+1:]...)
			cfg.Save()
			return pendingPollResultMsg{}
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

func fetchInbox(poolName, registry string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return screens.InboxFetchedResult{Err: err}
		}

		pool := cfg.ActivePool()
		if pool == nil || pool.MatchHash == "" {
			return screens.InboxFetchedResult{Err: fmt.Errorf("not registered")}
		}

		ghToken, err := resolveGitHubTokenNonInteractive()
		if err != nil {
			return screens.InboxFetchedResult{Err: fmt.Errorf("GitHub auth required")}
		}

		client := gh.NewPool(pool.Repo, ghToken)
		prs, err := client.ListInterestsForMe(context.Background(), pool.MatchHash)
		if err != nil {
			return screens.InboxFetchedResult{Err: err}
		}

		var items []screens.InboxLikeItem
		for _, pr := range prs {
			items = append(items, screens.InboxLikeItem{
				PR:        pr,
				LikerHash: pr.Title, // title is the target match_hash
			})
		}

		return screens.InboxFetchedResult{Likes: items}
	}
}

func acceptLike(poolName string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return screens.InboxActionResult{Accepted: true, Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil {
			return screens.InboxActionResult{Accepted: true, Err: fmt.Errorf("no active pool")}
		}
		_, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return screens.InboxActionResult{Accepted: true, Err: err}
		}
		token, err := cfg.DecryptToken(priv)
		if err != nil {
			return screens.InboxActionResult{Accepted: true, Err: err}
		}
		client := gh.NewPool(pool.Repo, token)
		err = client.AcceptLike(context.Background(), prNumber)
		return screens.InboxActionResult{Accepted: true, Err: err}
	}
}

func rejectLike(poolName string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return screens.InboxActionResult{Accepted: false, Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil {
			return screens.InboxActionResult{Accepted: false, Err: fmt.Errorf("no active pool")}
		}
		_, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return screens.InboxActionResult{Accepted: false, Err: err}
		}
		token, err := cfg.DecryptToken(priv)
		if err != nil {
			return screens.InboxActionResult{Accepted: false, Err: err}
		}
		client := gh.NewPool(pool.Repo, token)
		err = client.RejectLike(context.Background(), prNumber)
		return screens.InboxActionResult{Accepted: false, Err: err}
	}
}

type profileSubmitResultMsg struct {
	issueNum int
	err      error
}

func submitProfileUpdate(profile *gh.DatingProfile) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return profileSubmitResultMsg{err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil {
			return profileSubmitResultMsg{err: fmt.Errorf("no active pool")}
		}
		pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return profileSubmitResultMsg{err: err}
		}
		token, err := cfg.DecryptToken(priv)
		if err != nil {
			return profileSubmitResultMsg{err: err}
		}

		ctx := context.Background()
		num, err := svc.SubmitProfileToPool(ctx, pool.Repo, pool.OperatorPubKey, token,
			profile, cfg.User.IDHash, pub, priv)
		return profileSubmitResultMsg{issueNum: num, err: err}
	}
}

func poolStatusesFromConfig() (map[string]string, map[string]int) {
	cfg, _ := config.Load()
	ps := make(map[string]string)
	pi := make(map[string]int)
	if cfg != nil {
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
	}
	return ps, pi
}

func poolNames(statuses map[string]string) []string {
	var names []string
	for name, status := range statuses {
		if status == "active" || status == "" {
			names = append(names, name)
		}
	}
	return names
}

func registries(active string) []string {
	if active == "" {
		return nil
	}
	cfg, _ := config.Load()
	if cfg != nil && len(cfg.Registries) > 0 {
		return cfg.Registries
	}
	return []string{active}
}

func resolveGitHubTokenNonInteractive() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token from gh CLI")
	}
	return token, nil
}

func sendLike(poolName, registry, targetMatchHash string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return components.ToastMsg{Text: "Error: " + err.Error(), Level: components.ToastError}
		}
		var pool *config.PoolConfig
		for i := range cfg.Pools {
			if cfg.Pools[i].Name == poolName {
				pool = &cfg.Pools[i]
				break
			}
		}
		if pool == nil || pool.BinHash == "" || pool.MatchHash == "" {
			return components.ToastMsg{Text: "Not registered in pool", Level: components.ToastError}
		}

		ghToken, err := resolveGitHubTokenNonInteractive()
		if err != nil {
			return components.ToastMsg{Text: "GitHub auth required", Level: components.ToastError}
		}

		operatorPubBytes, _ := hex.DecodeString(pool.OperatorPubKey)
		client := gh.NewPool(pool.Repo, ghToken)
		_, err = client.CreateInterestPR(
			context.Background(),
			pool.BinHash,
			pool.MatchHash,
			targetMatchHash,
			"Hey! I'd like to connect.",
			ed25519.PublicKey(operatorPubBytes),
		)
		if err != nil {
			if strings.Contains(err.Error(), "422") {
				return components.ToastMsg{Text: "Already liked this person", Level: components.ToastInfo}
			}
			return components.ToastMsg{Text: "Error: " + err.Error(), Level: components.ToastError}
		}

		return components.ToastMsg{Text: "♥ Interest sent!", Level: components.ToastSuccess}
	}
}

func padToHeight(s string, height int) string {
	lines := countLines(s)
	for lines < height {
		s += "\n"
		lines++
	}
	return s
}
