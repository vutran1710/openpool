package components

import (
	"github.com/charmbracelet/lipgloss"
	figure "github.com/common-nighthawk/go-figure"
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
	// Blocky ASCII title
	fig := figure.NewFigure("dating.dev", "rectangles", true)
	title := lipgloss.NewStyle().Foreground(theme.Pink).Render(fig.String())

	// Pulsing heart
	heart := s.Heart.Inline()

	left := lipgloss.JoinHorizontal(lipgloss.Center, heart, "  ", title)

	// User/pool info (right-aligned, vertically centered)
	var infoLines []string
	if s.User != "" {
		infoLines = append(infoLines, theme.DimStyle.Render("⬡ ")+theme.AccentStyle.Render(s.User))
	}
	if s.Pool != "" {
		infoLines = append(infoLines, theme.DimStyle.Render("◈ ")+theme.GreenStyle.Render(s.Pool))
	}

	rightWidth := s.Width - lipgloss.Width(left) - 6
	if rightWidth < 10 {
		rightWidth = 10
	}

	right := ""
	for _, l := range infoLines {
		right += l + "\n"
	}
	rightBlock := lipgloss.NewStyle().
		Align(lipgloss.Right).
		Width(rightWidth).
		Render(right)

	bar := lipgloss.NewStyle().
		Padding(0, 2).
		Width(s.Width).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, left, rightBlock))

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
