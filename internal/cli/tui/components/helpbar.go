package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type KeyBind struct {
	Key  string
	Desc string
}

type HelpBar struct {
	Bindings []KeyBind
	Width    int
}

func NewHelpBar(bindings ...KeyBind) HelpBar {
	return HelpBar{Bindings: bindings}
}

func (h HelpBar) View() string {
	parts := make([]string, len(h.Bindings))
	keyStyle := lipgloss.NewStyle().Foreground(theme.Violet).Bold(true)
	descStyle := theme.DimStyle

	for i, b := range h.Bindings {
		parts[i] = keyStyle.Render(b.Key) + " " + descStyle.Render(b.Desc)
	}

	bar := strings.Join(parts, theme.DimStyle.Render("  ·  "))

	return lipgloss.NewStyle().
		Padding(0, 2).
		Width(h.Width).
		Render(bar)
}
