package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

type ProfileData struct {
	PublicID    string
	DisplayName string
	Bio         string
	City        string
	Interests   []string
	LookingFor  string
	Status      string
}

func RenderProfileCard(p ProfileData, width int) string {
	if width < 20 {
		width = 40
	}
	cardWidth := width - 8
	if cardWidth > 50 {
		cardWidth = 50
	}

	header := fmt.Sprintf(
		"%s  %s  %s",
		theme.BoldStyle.Render(p.PublicID),
		theme.DimStyle.Render(p.City),
		statusBadge(p.Status),
	)

	name := theme.AccentStyle.Render(p.DisplayName)

	bio := theme.TextStyle.Render(p.Bio)
	if len(p.Bio) > cardWidth-4 {
		bio = theme.TextStyle.Render(p.Bio[:cardWidth-7] + "...")
	}

	interests := theme.DimStyle.Render("interests: ") +
		theme.TextStyle.Render(strings.Join(p.Interests, ", "))

	looking := theme.DimStyle.Render("looking for: ") +
		theme.TextStyle.Render(p.LookingFor)

	content := fmt.Sprintf(
		"%s\n%s\n\n%s\n\n%s\n%s",
		header, name, bio, interests, looking,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		Width(cardWidth).
		Render(content)
}

func statusBadge(status string) string {
	switch status {
	case "open":
		return theme.GreenStyle.Render("● open")
	case "committed":
		return theme.BrandStyle.Render("♥ committed")
	default:
		return theme.DimStyle.Render("○ " + status)
	}
}
