package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type StatusBar struct {
	User     string
	Pool     string
	Width    int
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}

func (s StatusBar) View() string {
	left := theme.Logo()

	right := ""
	if s.User != "" {
		right += theme.DimStyle.Render("⬡ ") + theme.AccentStyle.Render(s.User)
	}
	if s.Pool != "" {
		right += theme.DimStyle.Render("  ◈ ") + theme.GreenStyle.Render(s.Pool)
	}

	gap := s.Width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}

	bar := lipgloss.NewStyle().
		Padding(0, 2).
		Background(theme.Surface).
		Width(s.Width).
		Render(left + spaces(gap) + right)

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
