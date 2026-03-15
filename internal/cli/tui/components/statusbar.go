package components

import (
	figure "github.com/common-nighthawk/go-figure"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type StatusBar struct {
	User  string
	Pool  string
	Width int
	Heart HeartBeat
}

func NewStatusBar() StatusBar {
	return StatusBar{
		Heart: NewHeartBeat(),
	}
}

func (s StatusBar) View() string {
	// Big ASCII title
	fig := figure.NewFigure("dating.dev", "mini", true)
	title := lipgloss.NewStyle().Foreground(theme.Pink).Render(fig.String())

	// Pulsing heart next to title
	heart := s.Heart.Inline()

	// User/pool info on the right
	info := ""
	if s.User != "" {
		info += theme.DimStyle.Render("⬡ ") + theme.AccentStyle.Render(s.User)
	}
	if s.Pool != "" {
		info += theme.DimStyle.Render("  ◈ ") + theme.GreenStyle.Render(s.Pool)
	}

	// Layout: heart + title on left, info on right
	leftBlock := lipgloss.JoinHorizontal(lipgloss.Center, heart, "  ", title)

	gap := s.Width - lipgloss.Width(leftBlock) - lipgloss.Width(info) - 4
	if gap < 1 {
		gap = 1
	}

	bar := lipgloss.NewStyle().
		Padding(0, 2).
		Background(theme.Surface).
		Width(s.Width).
		Render(leftBlock + spaces(gap) + info)

	separator := lipgloss.NewStyle().
		Width(s.Width).
		Foreground(theme.Border).
		Render(Repeat("─", s.Width))

	return bar + "\n" + separator
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func Repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
