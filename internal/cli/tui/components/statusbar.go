package components

import (
	"github.com/charmbracelet/lipgloss"
	figure "github.com/common-nighthawk/go-figure"
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
	// Blocky ASCII title
	fig := figure.NewFigure("dating.dev", "rectangles", true)
	title := lipgloss.NewStyle().Foreground(theme.Pink).Render(fig.String())

	// Pulsing heart
	heart := s.Heart.Inline()

	left := lipgloss.JoinHorizontal(lipgloss.Center, heart, "  ", title)

	// User info (right-aligned)
	var infoLines []string
	if s.User != "" {
		userLine := theme.DimStyle.Render("⬡ ") + theme.AccentStyle.Render(s.User)
		if s.UserHash != "" {
			userLine += theme.DimStyle.Render("  ") + theme.DimStyle.Render(s.UserHash)
		}
		infoLines = append(infoLines, userLine)
	}

	// Row 2: pool + registry
	var row2Parts []string
	if s.Pool != "" {
		row2Parts = append(row2Parts, theme.DimStyle.Render("◈ ")+theme.GreenStyle.Render(s.Pool))
	} else {
		row2Parts = append(row2Parts, theme.DimStyle.Render("◈ no pool"))
	}
	if s.Registry != "" {
		row2Parts = append(row2Parts, theme.DimStyle.Render("⊞ ")+theme.DimStyle.Render(s.Registry))
	} else {
		row2Parts = append(row2Parts, theme.DimStyle.Render("⊞ no registry"))
	}
	infoLines = append(infoLines, lipgloss.JoinHorizontal(lipgloss.Center, row2Parts[0], "  ", row2Parts[1]))

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
