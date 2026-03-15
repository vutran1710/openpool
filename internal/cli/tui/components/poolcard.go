package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	figure "github.com/common-nighthawk/go-figure"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
)

// PoolCardData holds all data needed to render a pool detail card.
type PoolCardData struct {
	Name          string
	Description   string
	About         string
	Operator      string
	OperatorKey   string
	Repo          string
	RelayURL      string
	Website       string
	CreatedAt     string
	Tags          []string
	Members       int
	Matches       int
	Relationships int
	Joined        bool
}

// RenderPoolCard renders a detailed pool info card.
func RenderPoolCard(p PoolCardData, width int, focused bool) string {
	borderColor := theme.Border
	if focused {
		borderColor = theme.Violet
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width).
		Padding(1, 2)

	// Logo + header side by side
	headerBlock := renderCardHeader(p)
	headerSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		PoolLogo(),
		"  ",
		headerBlock,
	)

	content := headerSection + "\n"

	// Short description
	if p.Description != "" {
		content += "\n" + theme.TextStyle.Render(p.Description) + "\n"
	}

	// Stats bar
	content += "\n" + RenderStatsBar(p.Members, p.Matches, p.Relationships) + "\n"

	// Tags
	if len(p.Tags) > 0 {
		content += "\n" + RenderTags(p.Tags) + "\n"
	}

	// About (longer description)
	if p.About != "" {
		content += "\n" + theme.DimStyle.Render("About") + "\n"
		content += theme.TextStyle.Render(p.About) + "\n"
	}

	// Info table
	content += "\n" + renderCardInfo(p)

	// Action
	content += "\n" + renderCardAction(p)

	return cardStyle.Render(content)
}

func renderCardHeader(p PoolCardData) string {
	// Pool name in ASCII art
	fig := figure.NewFigure(p.Name, "mini", true)
	title := theme.BrandStyle.Render(fig.String())

	statusBadge := theme.DimStyle.Render("→ join")
	if p.Joined {
		statusBadge = theme.GreenStyle.Render("✓ joined")
	}

	return title + statusBadge
}

func renderCardInfo(p PoolCardData) string {
	labelStyle := theme.DimStyle.Copy().Width(12)

	info := ""
	info += labelStyle.Render("Operator") + infoValue(p.Operator) + "\n"
	info += labelStyle.Render("Key") + infoValue(p.OperatorKey) + "\n"
	info += labelStyle.Render("Repo") + infoValue(p.Repo) + "\n"
	info += labelStyle.Render("Website") + infoValueAccent(p.Website) + "\n"
	info += labelStyle.Render("Relay") + infoValue(p.RelayURL) + "\n"

	created := p.CreatedAt
	if len(created) > 10 {
		created = created[:10]
	}
	info += labelStyle.Render("Created") + infoValue(created) + "\n"

	return info
}

func infoValue(v string) string {
	if v == "" {
		return theme.DimStyle.Render("unavailable")
	}
	return theme.TextStyle.Render(v)
}

func infoValueAccent(v string) string {
	if v == "" {
		return theme.DimStyle.Render("unavailable")
	}
	return theme.AccentStyle.Render(v)
}

func renderCardAction(p PoolCardData) string {
	if p.Joined {
		return theme.GreenStyle.Render("You are a member of this pool")
	}
	return theme.DimStyle.Render("Press enter to join  ·  dating pool join " + p.Name)
}

// RenderStatsBar renders a horizontal stats bar with icons.
func RenderStatsBar(members, matches, relationships int) string {
	return fmt.Sprintf(
		"%s  %s  %s",
		theme.AccentStyle.Render(fmt.Sprintf("👤 %d members", members)),
		theme.BrandStyle.Render(fmt.Sprintf("♥ %d matches", matches)),
		theme.GreenStyle.Render(fmt.Sprintf("💍 %d committed", relationships)),
	)
}

// RenderTags renders a row of hashtags.
func RenderTags(tags []string) string {
	var parts []string
	for _, t := range tags {
		parts = append(parts, theme.AccentStyle.Render("#"+t))
	}
	return strings.Join(parts, "  ")
}

// RenderPoolListItem renders a single pool item for a list.
func RenderPoolListItem(name, description string, joined, selected bool, maxDescWidth int) string {
	cursor := "  "
	nameStyle := theme.NormalItem
	if selected {
		cursor = theme.Cursor()
		nameStyle = theme.ActiveItem
	}

	status := theme.DimStyle.Render(" → join")
	if joined {
		status = theme.GreenStyle.Render(" ✓")
	}

	line := fmt.Sprintf("%s%s%s\n", cursor, nameStyle.Render(name), status)

	if description != "" {
		desc := description
		if maxDescWidth > 0 && len(desc) > maxDescWidth {
			desc = desc[:maxDescWidth-3] + "..."
		}
		line += fmt.Sprintf("    %s\n", theme.DimStyle.Render(desc))
	}

	return line
}
