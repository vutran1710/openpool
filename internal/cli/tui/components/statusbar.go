package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type StatusBar struct {
	User     string
	UserHash string
	Pool     string
	Registry string
	Width    int
	Heart    HeartBeat
}

func NewStatusBar() StatusBar {
	return StatusBar{
		Heart: NewHeartBeat(),
	}
}

func (s StatusBar) View() string {
	rightAlign := lipgloss.NewStyle().Align(lipgloss.Right)
	pad := lipgloss.NewStyle().Padding(0, 2)

	// Row 1: ♥ dating.dev ... ⬡ User hash
	heart := s.Heart.Inline()
	title := lipgloss.NewStyle().Foreground(theme.Pink).Bold(true).Render("dating.dev")
	leftR1 := heart + " " + title

	rightR1 := ""
	if s.User != "" {
		rightR1 = theme.DimStyle.Render("⬡ ") + theme.AccentStyle.Render(s.User)
		if s.UserHash != "" {
			rightR1 += theme.DimStyle.Render("  ") + theme.DimStyle.Render(s.UserHash)
		}
	}

	gap1 := s.Width - lipgloss.Width(leftR1) - lipgloss.Width(rightR1) - 4
	if gap1 < 1 {
		gap1 = 1
	}
	row1 := pad.Width(s.Width).Render(leftR1 + spaces(gap1) + rightR1)

	// Row 2: (empty left) ... ◈ pool  ⊞ registry
	poolPart := theme.DimStyle.Render("◈ no pool")
	if s.Pool != "" {
		poolPart = theme.DimStyle.Render("◈ ") + theme.GreenStyle.Render(s.Pool)
	}
	regPart := theme.DimStyle.Render("⊞ no registry")
	if s.Registry != "" {
		regPart = theme.DimStyle.Render("⊞ ") + theme.DimStyle.Render(s.Registry)
	}
	rightR2 := poolPart + "  " + regPart

	row2 := pad.Width(s.Width).Render(rightAlign.Width(s.Width - 4).Render(rightR2))

	separator := lipgloss.NewStyle().
		Width(s.Width).
		Foreground(theme.Border).
		Render(Repeat("─", s.Width))

	return row1 + "\n" + row2 + "\n" + separator
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
