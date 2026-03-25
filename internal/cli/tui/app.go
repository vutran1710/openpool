package tui

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/openpool/internal/cli/chat"
	"github.com/vutran1710/openpool/internal/cli/config"
	relayclient "github.com/vutran1710/openpool/internal/cli/relay"
	"github.com/vutran1710/openpool/internal/cli/tui/components"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/openpool/internal/cli/tui/screens"
	"github.com/vutran1710/openpool/internal/gitclient"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/crypto"
	dbg "github.com/vutran1710/openpool/internal/debug"
	gh "github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/schema"
)

type activeScreen int

const (
	screenOnboarding activeScreen = iota
	screenHome
	screenDiscover
	screenMatches
	screenChat
	screenPools
	screenProfile
	screenSettings
	screenInbox
	screenPoolOnboard
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
	poolOnboard screens.PoolOnboardScreen
	profile     screens.ProfileScreen
	settings    screens.SettingsScreen
	inbox      screens.InboxScreen

	user     string
	pool     string
	registry string

	chatClient *chat.ChatClient
	program    *tea.Program
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
	a.pools.SetActivePool(pool)
	a.statusBar.User = userName
	a.statusBar.UserHash = userHash
	a.statusBar.Pool = pool
	a.statusBar.Registry = registry

	// Load registry branding (accent color, name)
	if registry != "" {
		if reg, err := gh.CloneRegistry(registry); err == nil {
			if meta := reg.Meta(); meta != nil {
				if meta.Accent != "" {
					theme.SetAccent(meta.Accent)
				}
				if meta.Name != "" {
					a.statusBar.Registry = meta.Name
				}
			}
		}
	}
	a.updateHelp()
	return a
}

type chatClientInitMsg struct {
	client *chat.ChatClient
}

type chatNewMessageMsg struct {
	PeerMatchHash string
}

func (a *app) initChatClient() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return nil
		}
		pool := cfg.ActivePool()
		if pool == nil || pool.RelayURL == "" || pool.MatchHash == "" {
			return nil
		}
		pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return nil
		}

		dbPath := filepath.Join(config.Dir(), "conversations.db")
		convoDB, err := chat.OpenConversationDB(dbPath)
		if err != nil {
			return nil
		}

		idHash := pool.IDHash
		if idHash == "" {
			idHash = string(crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID))
		}
		relay := relayclient.NewClient(relayclient.Config{
			RelayURL:  pool.RelayURL,
			PoolURL:   pool.Repo,
			IDHash:    idHash,
			MatchHash: pool.MatchHash,
			Pub:       pub,
			Priv:      priv,
		})

		client := chat.NewChatClient(relay, convoDB)
		return chatClientInitMsg{client: client}
	}
}

func (a app) Init() tea.Cmd {
	cmds := []tea.Cmd{inputInit(), a.initChatClient()}
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
	case screenProfile:
		bindings = a.profile.HelpBindings()
	case screenSettings:
		bindings = a.settings.HelpBindings()
	case screenInbox:
		bindings = a.inbox.HelpBindings()
	case screenPoolOnboard:
		bindings = a.poolOnboard.HelpBindings()
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
		a.poolOnboard, _ = a.poolOnboard.Update(msg)
		a.home = a.home.SetSize(msg.Width, msg.Height)
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
			if a.screen == screenPoolOnboard {
				a.screen = screenPools
				a.updateHelp()
				return a, nil
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

	case screens.InboxAcceptMsg:
		return a, acceptLike(a.pool, msg.IssueNumber)

	case screens.InboxRejectMsg:
		return a, rejectLike(a.pool, msg.IssueNumber)

	case screens.PoolSwitchMsg:
		a.pool = msg.Name
		a.statusBar.Pool = msg.Name
		a.pools.SetActivePool(msg.Name)
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

	case screens.ChatSendMsg:
		if a.chatClient != nil {
			a.chatClient.Send(msg.PeerMatchHash, msg.Text)
		}
		a.chat.AppendMessage(msg.Text, true)
		return a, nil

	case screens.MatchesFetchedMsg:
		// Update matches screen
		a.matches, _ = a.matches.Update(msg)
		// Persist greetings + set peer keys
		if a.chatClient != nil {
			for _, m := range msg.Matches {
				if m.Greeting != "" {
					a.chatClient.PersistGreeting(m.MatchHash, m.Greeting)
				}
				a.chatClient.SetPeerKey(m.MatchHash, m.PubKey)
			}
			// Refresh conversations panel
			convos, _ := a.chatClient.Conversations()
			a.home = a.home.SetConversations(convos)
		}
		return a, nil

	case screens.DiscoverLikeMsg:
		return a, sendLike(a.pool, a.registry, msg.TargetMatchHash)

	case screens.ChatUnmatchMsg:
		return a, sendUnmatch(a.pool, msg.TargetMatchHash)

	case chatClientInitMsg:
		a.chatClient = msg.client
		a.chatClient.OnMsg = func(peerMatchHash string) {
			if a.program != nil {
				a.program.Send(chatNewMessageMsg{PeerMatchHash: peerMatchHash})
			}
		}
		// Load existing peer keys + conversations into home screen
		a.chatClient.LoadPeerKeys()
		convos, _ := a.chatClient.Conversations()
		a.home = a.home.SetConversations(convos)
		// Connect relay in background
		return a, func() tea.Msg {
			ctx := context.Background()
			a.chatClient.Relay.Connect(ctx)
			return nil
		}

	case chatNewMessageMsg:
		// Refresh conversations panel
		if a.chatClient != nil {
			convos, _ := a.chatClient.Conversations()
			a.home = a.home.SetConversations(convos)
		}
		// If on chat screen with this peer, show the message + mark read
		if a.screen == screenChat && a.chat.TargetID == msg.PeerMatchHash {
			history, _ := a.chatClient.History(msg.PeerMatchHash)
			if len(history) > 0 {
				latest := history[len(history)-1]
				a.chat.AppendMessage(latest.Body, latest.IsMe)
			}
			a.chatClient.MarkRead(msg.PeerMatchHash)
		} else {
			// Not on chat with this peer — show toast notification
			sender := msg.PeerMatchHash
			if len(sender) > 8 {
				sender = sender[:8] + "..."
			}
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "New message from " + sender, Level: components.ToastInfo}
			}
		}
		return a, nil

	case screens.ConversationOpenMsg:
		a.screen = screenChat
		a.chat = screens.NewChatScreen(msg.PeerMatchHash, a.width, a.height)
		if a.chatClient != nil {
			history, _ := a.chatClient.History(msg.PeerMatchHash)
			a.chat.LoadHistory(history)
			a.chatClient.MarkRead(msg.PeerMatchHash)
		}
		a.updateHelp()
		return a, nil

	case screens.MatchChatMsg:
		a.screen = screenChat
		a.chat = screens.NewChatScreen(msg.MatchHash, a.width, a.height)
		if a.chatClient != nil {
			a.chatClient.SetPeerKey(msg.MatchHash, msg.PubKey)
			history, _ := a.chatClient.History(msg.MatchHash)
			a.chat.LoadHistory(history)
			a.chatClient.MarkRead(msg.MatchHash)
		}
		a.updateHelp()
		return a, nil

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
		// Find pool repo from the pools screen (fetched from registry)
		poolRepo := ""
		for _, p := range a.pools.GetPools() {
			if p.Name == msg.Name {
				poolRepo = p.Repo
				break
			}
		}
		// Fallback: check local config (for rejoin)
		if poolRepo == "" && cfg != nil {
			for _, p := range cfg.Pools {
				if p.Name == msg.Name {
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

		// Load pool.yaml via raw URL (fast), clone repo in background
		rawURL := gitclient.RawURL(poolRepo, "main", "pool.yaml")
		s, err := schema.Load(rawURL)
		if err != nil {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Pool missing pool.yaml: " + err.Error(), Level: components.ToastError}
			}
		}

		// Clone repo in background for later use
		go func() {
			if repo, err := gitclient.Clone(gitclient.EnsureGitURL(poolRepo)); err == nil {
				repo.Sync()
			}
		}()

		a.poolOnboard = screens.NewPoolOnboardScreen(msg.Name, s, a.width, a.height)
		a.screen = screenPoolOnboard
		a.updateHelp()
		return a, tea.Batch(cmds...)

	case screens.PoolOnboardDoneMsg:
		// Save profile to disk
		profilePath := schema.ProfilePath(config.Dir(), msg.PoolName)
		if err := schema.SaveProfile(profilePath, msg.Profile); err != nil {
			return a, func() tea.Msg {
				return components.ToastMsg{Text: "Failed to save profile: " + err.Error(), Level: components.ToastError}
			}
		}

		// Add role to profile if set
		if msg.Role != "" {
			msg.Profile["_role"] = msg.Role
		}

		// Ensure pool is in config before submitting (first-time join)
		cfg, _ := config.Load()
		if cfg != nil {
			found := false
			for _, p := range cfg.Pools {
				if p.Name == msg.PoolName {
					found = true
					break
				}
			}
			if !found {
				// Add pool from registry entries
				for _, p := range a.pools.GetPools() {
					if p.Name == msg.PoolName {
						cfg.AddPool(config.PoolConfig{
							Name:           msg.PoolName,
							Repo:           p.Repo,
							OperatorPubKey: p.OperatorPubKey,
							RelayURL:       p.RelayURL,
							Status:         "joining",
						})
						cfg.Save()
						break
					}
				}
			}
		}

		// Trigger registration submission
		return a, submitPoolRegistration(msg.PoolName, msg.Profile)

	case poolRegistrationSubmittedMsg:
		// Update pool config with pending status
		cfg, _ := config.Load()
		if cfg != nil {
			// Find pool info from pools screen
			var poolCfg config.PoolConfig
			for _, p := range a.pools.GetPools() {
				if p.Name == msg.poolName {
					poolCfg = config.PoolConfig{
						Name:           msg.poolName,
						Repo:           p.Repo,
						OperatorPubKey: p.OperatorPubKey,
						RelayURL:       p.RelayURL,
						Status:         "pending",
						PendingIssue:   msg.issueNumber,
					}
					break
				}
			}
			if poolCfg.Name == "" {
				// Fallback: check existing config
				for _, p := range cfg.Pools {
					if p.Name == msg.poolName {
						poolCfg = p
						poolCfg.Status = "pending"
						poolCfg.PendingIssue = msg.issueNumber
						break
					}
				}
			}
			if poolCfg.Name != "" {
				cfg.AddPool(poolCfg)
				if cfg.Active == "" {
					cfg.Active = msg.poolName
				}
				cfg.Save()
			}
		}

		a.pool = msg.poolName
		a.statusBar.Pool = msg.poolName

		// Refresh pools screen
		ps, pi := poolStatusesFromConfig()
		a.pools = screens.NewPoolsScreen(a.registry, ps, pi)
		a.pools.Width = a.width
		a.pools.Height = a.height

		a.screen = screenHome
		a.updateHelp()
		return a, func() tea.Msg {
			return components.ToastMsg{
				Text:  fmt.Sprintf("Registration submitted (Issue #%d) — waiting for approval", msg.issueNumber),
				Level: components.ToastSuccess,
			}
		}

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
		a.updateHelp()
		return a, fetchInbox(a.pool, a.registry)
	case "profile":
		dbg.Log("navigate → profile")
		a.profile.SetPool(a.pool)
		a.screen = screenProfile
		a.profile, _ = a.profile.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		return a, screens.LoadProfileCmd(a.pool)
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
			a.profile.SetPool(a.pool)
			a.screen = screenProfile
			a.profile, _ = a.profile.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			a.updateHelp()
			return a, screens.LoadProfileCmd(a.pool)
		case "/profile-edit", "/edit":
			if a.pool == "" {
				return a, func() tea.Msg {
					return components.ToastMsg{Text: "No active pool", Level: components.ToastError}
				}
			}
			// Load existing profile and schema, launch edit screen
			poolRepo := ""
			cfg, _ := config.Load()
			if cfg != nil {
				for _, p := range cfg.Pools {
					if p.Name == a.pool {
						poolRepo = p.Repo
						break
					}
				}
			}
			if poolRepo == "" {
				return a, func() tea.Msg {
					return components.ToastMsg{Text: "Pool not found in config", Level: components.ToastError}
				}
			}
			rawURL := gitclient.RawURL(poolRepo, "main", "pool.yaml")
			s, err := schema.Load(rawURL)
			if err != nil {
				return a, func() tea.Msg {
					return components.ToastMsg{Text: "Failed to load pool schema", Level: components.ToastError}
				}
			}
			profilePath := schema.ProfilePath(config.Dir(), a.pool)
			existing, _ := schema.LoadProfile(profilePath)
			a.poolOnboard = screens.NewPoolEditScreen(a.pool, s, a.width, a.height, existing)
			a.screen = screenPoolOnboard
			a.updateHelp()
			return a, nil
		case "/inbox":
			a.screen = screenInbox
			a.inbox = screens.NewInboxScreen()
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
		case "/unmatch":
			if a.screen == screenChat {
				return a, func() tea.Msg {
					return screens.ChatUnmatchMsg{TargetMatchHash: a.chat.TargetID}
				}
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
	case screenPoolOnboard:
		a.poolOnboard, cmd = a.poolOnboard.Update(msg)
		a.updateHelp()
	case screenProfile:
		a.profile, cmd = a.profile.Update(msg)
	case screenSettings:
		a.settings, cmd = a.settings.Update(msg)
	case screenInbox:
		a.inbox, cmd = a.inbox.Update(msg)
	}

	if cmd != nil {
		// Don't forward to input during onboarding (it steals key events)
		if a.screen == screenOnboarding || a.screen == screenPoolOnboard || a.screen == screenSettings || a.screen == screenInbox || a.screen == screenDiscover {
			return a, cmd
		}
		var inputCmd tea.Cmd
		a.input, inputCmd = a.input.Update(msg)
		return a, tea.Batch(cmd, inputCmd)
	}

	if a.screen == screenOnboarding || a.screen == screenPoolOnboard || a.screen == screenSettings || a.screen == screenInbox || a.screen == screenDiscover {
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
	case screenPoolOnboard:
		content = a.poolOnboard.View()
	case screenProfile:
		content = a.profile.View()
	case screenSettings:
		content = a.settings.View()
	case screenInbox:
		content = a.inbox.View()
	}

	toastView := a.toast.View()

	// During onboarding, hide the command input
	bottom := ""
	if a.screen != screenOnboarding && a.screen != screenPoolOnboard {
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
				// Read bin_hash and match_hash from operator's signed comment
				operatorPubBytes, opErr := hex.DecodeString(p.OperatorPubKey)
				if opErr == nil {
					ghClient := gh.NewCLIOrHTTP(p.Repo, token)
					ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
					content, findErr := gh.FindOperatorReplyInIssue(ctx2, ghClient, p.PendingIssue, "registration", ed25519.PublicKey(operatorPubBytes))
					cancel2()
					if findErr == nil {
						plaintext, decErr := crypto.DecryptSignedBlob(content, priv)
						if decErr == nil {
							var hashes map[string]string
							if msgpack.Unmarshal(plaintext, &hashes) == nil {
								cfg.Pools[i].BinHash = hashes["bin_hash"]
								cfg.Pools[i].MatchHash = hashes["match_hash"]
							}
						}
					}
				}
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
		if pool == nil {
			return screens.InboxFetchedResult{Err: fmt.Errorf("no active pool — join a pool first")}
		}
		if pool.MatchHash == "" {
			return screens.InboxFetchedResult{Err: fmt.Errorf("registration pending — wait for pool to process your request")}
		}

		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return screens.InboxFetchedResult{Err: fmt.Errorf("GitHub auth required")}
		}

		client := gh.NewPoolWithClient(gh.NewCLIOrHTTP(pool.Repo, ghToken))
		issues, err := client.ListInterestsForMeIssues(context.Background(), pool.MatchHash)
		if err != nil {
			return screens.InboxFetchedResult{Err: err}
		}

		var items []screens.InboxLikeItem
		for _, iss := range issues {
			items = append(items, screens.InboxLikeItem{
				Issue:     iss,
				LikerHash: iss.Title, // title is the target match_hash
			})
		}

		return screens.InboxFetchedResult{Likes: items}
	}
}

func acceptLike(poolName string, issueNumber int) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return screens.InboxActionResult{Accepted: true, Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil {
			return screens.InboxActionResult{Accepted: true, Err: fmt.Errorf("no active pool")}
		}
		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return screens.InboxActionResult{Accepted: true, Err: err}
		}
		client := gh.NewCLIOrHTTP(pool.Repo, ghToken)
		err = client.CloseIssue(context.Background(), issueNumber, "completed")
		return screens.InboxActionResult{Accepted: true, Err: err}
	}
}

func rejectLike(poolName string, issueNumber int) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return screens.InboxActionResult{Accepted: false, Err: err}
		}
		pool := cfg.ActivePool()
		if pool == nil {
			return screens.InboxActionResult{Accepted: false, Err: fmt.Errorf("no active pool")}
		}
		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return screens.InboxActionResult{Accepted: false, Err: err}
		}
		client := gh.NewCLIOrHTTP(pool.Repo, ghToken)
		err = client.CloseIssue(context.Background(), issueNumber, "not_planned")
		return screens.InboxActionResult{Accepted: false, Err: err}
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

type poolRegistrationSubmittedMsg struct {
	poolName    string
	issueNumber int
}

func submitPoolRegistration(poolName string, profile map[string]any) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return components.ToastMsg{Text: "Config error: " + err.Error(), Level: components.ToastError}
		}

		// Find pool config
		var pool *config.PoolConfig
		for i := range cfg.Pools {
			if cfg.Pools[i].Name == poolName {
				pool = &cfg.Pools[i]
				break
			}
		}
		if pool == nil {
			return components.ToastMsg{Text: "Pool not found in config: " + poolName, Level: components.ToastError}
		}

		// Load keys
		pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
		if err != nil {
			return components.ToastMsg{Text: "Keys not found: " + err.Error(), Level: components.ToastError}
		}

		// Encode profile as JSON
		profileJSON, err := json.Marshal(profile)
		if err != nil {
			return components.ToastMsg{Text: "Profile encode failed: " + err.Error(), Level: components.ToastError}
		}

		// Encrypt to operator pubkey
		operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
		if err != nil {
			return components.ToastMsg{Text: "Invalid operator key", Level: components.ToastError}
		}
		bin, err := crypto.PackUserBin(pub, ed25519.PublicKey(operatorPubBytes), profileJSON)
		if err != nil {
			return components.ToastMsg{Text: "Encrypt failed: " + err.Error(), Level: components.ToastError}
		}

		// Submit registration issue
		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return components.ToastMsg{Text: "GitHub auth required", Level: components.ToastError}
		}

		ghClient := gh.NewCLIOrHTTP(pool.Repo, ghToken)
		poolClient := gh.NewPoolWithClient(ghClient)

		userHash := crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID)
		pubKeyHex := hex.EncodeToString(pub)

		// Identity proof
		identityProof, _ := crypto.EncryptIdentityProof(pool.OperatorPubKey, cfg.User.Provider, cfg.User.ProviderUserID)
		sig := hex.EncodeToString(ed25519.Sign(priv, []byte(pubKeyHex)))

		issueNumber, err := poolClient.RegisterUserViaIssue(
			context.Background(),
			string(userHash),
			bin,
			pubKeyHex,
			sig,
			identityProof,
		)
		if err != nil {
			return components.ToastMsg{Text: "Registration failed: " + err.Error(), Level: components.ToastError}
		}

		return poolRegistrationSubmittedMsg{
			poolName:    poolName,
			issueNumber: issueNumber,
		}
	}
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

		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return components.ToastMsg{Text: "GitHub auth required", Level: components.ToastError}
		}

		operatorPubBytes, _ := hex.DecodeString(pool.OperatorPubKey)

		// Compute ephemeral title from pool's interest_expiry
		rawURL := gitclient.RawURL(pool.Repo, "main", "pool.yaml")
		s, sErr := schema.Load(rawURL)
		if sErr != nil {
			return components.ToastMsg{Text: "Failed to load pool schema", Level: components.ToastError}
		}
		expiry, eErr := s.ParseInterestExpiry()
		if eErr != nil {
			return components.ToastMsg{Text: "Pool missing interest_expiry", Level: components.ToastError}
		}
		ephemeralTitle := crypto.EphemeralHash(targetMatchHash, expiry)

		client := gh.NewPoolWithClient(gh.NewCLIOrHTTP(pool.Repo, ghToken))
		_, err = client.CreateInterestIssue(
			context.Background(),
			pool.BinHash,
			pool.MatchHash,
			targetMatchHash,
			ephemeralTitle,
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

func sendUnmatch(poolName, targetMatchHash string) tea.Cmd {
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
		if pool == nil || pool.MatchHash == "" {
			return components.ToastMsg{Text: "Not registered in pool", Level: components.ToastError}
		}

		ghToken, err := gh.GetCLIToken()
		if err != nil {
			return components.ToastMsg{Text: "GitHub auth required", Level: components.ToastError}
		}

		operatorPubBytes, _ := hex.DecodeString(pool.OperatorPubKey)
		client := gh.NewPoolWithClient(gh.NewCLIOrHTTP(pool.Repo, ghToken))
		_, err = client.SubmitUnmatchIssue(
			context.Background(),
			pool.MatchHash,
			targetMatchHash,
			ed25519.PublicKey(operatorPubBytes),
		)
		if err != nil {
			return components.ToastMsg{Text: "Error: " + err.Error(), Level: components.ToastError}
		}

		return components.ToastMsg{Text: "Unmatch submitted — match will be dissolved", Level: components.ToastSuccess}
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
