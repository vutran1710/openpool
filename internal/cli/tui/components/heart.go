package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// Compact heart logo — 7 chars wide, 4 lines tall.
const heartArt = "" +
	" ## ## \n" +
	" #   # \n" +
	"  # #  \n" +
	"   #   "

// PoolLogo renders the heart inside a small bordered box.
func PoolLogo() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Foreground(theme.Pink).
		Padding(0, 1).
		Render(heartArt)
}

// HeartBeat is a color-pulsing heart for the status bar header.
// Same shape every frame — only the color changes, so height is stable.
type HeartBeat struct {
	frame int
}

var pulseColors = []lipgloss.Color{
	lipgloss.Color("#992244"),
	lipgloss.Color("#CC2255"),
	lipgloss.Color("#FF6B9D"),
	lipgloss.Color("#FF9DBF"),
	lipgloss.Color("#FF6B9D"),
	lipgloss.Color("#CC2255"),
}

type HeartTickMsg struct{}

func NewHeartBeat() HeartBeat {
	return HeartBeat{}
}

func (h HeartBeat) Tick() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return HeartTickMsg{}
	})
}

func (h HeartBeat) Update(msg tea.Msg) (HeartBeat, tea.Cmd) {
	switch msg.(type) {
	case HeartTickMsg:
		h.frame = (h.frame + 1) % len(pulseColors)
		return h, h.Tick()
	}
	return h, nil
}

// Inline returns a small inline pulsing heart for the header.
func (h HeartBeat) Inline() string {
	color := pulseColors[h.frame%len(pulseColors)]
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render("♥")
}
