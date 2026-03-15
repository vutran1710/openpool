package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/screens"
)

type activeScreen int

const (
	screenHome activeScreen = iota
	screenDiscover
	screenMatches
	screenChat
	screenPools
)

type app struct {
	screen    activeScreen
	prevScreen activeScreen
	width     int
	height    int
	quitting  bool

	statusBar components.StatusBar
	input     components.Input
	toast     components.Toast
	helpBar   components.HelpBar

	home     screens.HomeScreen
	discover screens.DiscoverScreen
	matches  screens.MatchesScreen
	chat     screens.ChatScreen
	pools    screens.PoolsScreen

	user     string
	pool     string
	registry string
}

func newApp(user, pool, registry string, joinedPools []string) app {
	a := app{
		screen:    screenHome,
		user:      user,
		pool:      pool,
		registry:  registry,
		statusBar: components.NewStatusBar(),
		input:     components.NewInput("Type / for commands..."),
		toast:     components.NewToast(),
		home:      screens.NewHomeScreen(),
		discover:  screens.NewDiscoverScreen(),
		matches:   screens.NewMatchesScreen(),
		pools:     screens.NewPoolsScreen(registry, joinedPools),
	}
	a.statusBar.User = user
	a.statusBar.Pool = pool
	a.updateHelp()
	return a
}

func (a app) Init() tea.Cmd {
	return tea.Batch(inputInit(), a.statusBar.Heart.Tick())
}

func (a *app) updateHelp() {
	var bindings []components.KeyBind
	switch a.screen {
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
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		case "esc":
			if a.screen != screenHome {
				a.screen = screenHome
				a.updateHelp()
				return a, nil
			}
		}

	case components.MenuSelectMsg:
		return a.handleMenuSelect(msg.Key)

	case components.SubmitMsg:
		return a.handleSubmit(msg)

	case components.ToastMsg:
		var cmd tea.Cmd
		a.toast, cmd = a.toast.Update(msg)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)

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
	case "inbox":
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Inbox coming soon", Level: components.ToastInfo}
		}
	case "profile":
		return a, func() tea.Msg {
			return components.ToastMsg{Text: "Run: dating profile edit", Level: components.ToastInfo}
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
	}

	if cmd != nil {
		var inputCmd tea.Cmd
		a.input, inputCmd = a.input.Update(msg)
		return a, tea.Batch(cmd, inputCmd)
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
	}

	toastView := a.toast.View()
	bottom := a.helpBar.View() + "\n" + a.input.View()

	contentHeight := a.height - 6 - countLines(bottom)
	if toastView != "" {
		contentHeight -= countLines(toastView)
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	content = padToHeight(content, contentHeight)

	out := top + "\n" + content
	if toastView != "" {
		out += toastView
	}
	out += "\n" + bottom

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

func padToHeight(s string, height int) string {
	lines := countLines(s)
	for lines < height {
		s += "\n"
		lines++
	}
	return s
}
