package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
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
	Status        string // "active", "pending", "rejected", ""
	IsCurrentPool bool   // true if this is the currently active pool
	PendingIssue  int
	Logo          string // pre-rendered ASCII logo
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
	logo := p.Logo
	if logo == "" {
		logo = PoolLogo()
	}
	headerBlock := renderCardHeader(p)
	headerSection := lipgloss.JoinHorizontal(
		lipgloss.Top,
		logo,
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
	return theme.BoldStyle.Render(p.Name)
}

func renderCardInfo(p PoolCardData) string {
	created := p.CreatedAt
	if len(created) > 10 {
		created = created[:10]
	}

	rows := []table.Row{
		{"Operator", valOrDefault(p.Operator)},
		{"Key", valOrDefault(p.OperatorKey)},
		{"Repo", valOrDefault(p.Repo)},
		{"Website", valOrDefault(p.Website)},
		{"Relay", valOrDefault(p.RelayURL)},
		{"Created", valOrDefault(created)},
	}

	columns := []table.Column{
		{Title: "", Width: 12},
		{Title: "", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(len(rows)),
	)

	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle()
	s.Cell = lipgloss.NewStyle().Foreground(theme.Text)
	s.Selected = lipgloss.NewStyle().Foreground(theme.Text)
	t.SetStyles(s)

	// Style the label column
	return t.View()
}

func valOrDefault(v string) string {
	if v == "" {
		return "unavailable"
	}
	return v
}

func renderCardAction(p PoolCardData) string {
	var line string
	switch p.Status {
	case "active":
		if p.IsCurrentPool {
			line = theme.GreenStyle.Render("✓ Active")
		} else {
			line = theme.GreenStyle.Render("✓ Member") + theme.DimStyle.Render("  ·  Press enter to activate")
		}
	case "pending":
		line = theme.AmberStyle.Render("Registration pending — waiting for pool to process")
	case "rejected":
		line = theme.RedStyle.Render("Registration rejected") + theme.DimStyle.Render("  ·  Press enter to join again")
	default:
		return theme.DimStyle.Render("Press enter to join  ·  op pool join " + p.Name)
	}

	if p.PendingIssue > 0 && p.Repo != "" {
		url := fmt.Sprintf("https://github.com/%s/issues/%d", p.Repo, p.PendingIssue)
		line += "\n" + theme.DimStyle.Render("Issue: ") + theme.AccentStyle.Render(url)
	}
	return line
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
// poolStatus: "" or "active" = joined, "pending" = waiting, "rejected" = rejected, anything else = not joined
func RenderPoolListItem(name, description, poolStatus string, selected bool, maxDescWidth int) string {
	cursor := "  "
	nameStyle := theme.NormalItem
	if selected {
		cursor = theme.Cursor()
		nameStyle = theme.ActiveItem
	}

	var status string
	switch poolStatus {
	case "active":
		status = theme.GreenStyle.Render(" ✓")
	case "pending":
		status = theme.AmberStyle.Render(" ⏱")
	case "rejected":
		status = theme.RedStyle.Render(" ✗")
	default:
		status = theme.DimStyle.Render(" → join")
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
