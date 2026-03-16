package components

import (
	"encoding/base64"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// ProfileMode determines how much detail to render.
type ProfileMode int

const (
	ProfileCompact ProfileMode = iota // single line: name · bio · location
	ProfileShort                      // card: header, interests, intent, about (truncated)
	ProfileFull                       // page 1 of brochure: everything except showcase
)

// RenderProfile renders a DatingProfile in the specified mode.
func RenderProfile(p gh.DatingProfile, width int, mode ProfileMode) string {
	switch mode {
	case ProfileCompact:
		return renderCompact(p, width)
	case ProfileShort:
		return renderShort(p, width)
	case ProfileFull:
		return renderFull(p, width)
	}
	return ""
}

// RenderShowcase renders the showcase (page 2) as styled markdown in a string.
// Returns empty if no showcase data.
func RenderShowcase(p gh.DatingProfile, width int) string {
	if p.Showcase == "" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(p.Showcase)
	if err != nil {
		return theme.DimStyle.Render("(showcase decode error)")
	}

	mdWidth := width - 6
	if mdWidth < 30 {
		mdWidth = 40
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(mdWidth),
	)
	if err != nil {
		return theme.TextStyle.Render(string(decoded))
	}

	rendered, err := renderer.Render(string(decoded))
	if err != nil {
		return theme.TextStyle.Render(string(decoded))
	}

	return strings.TrimSpace(rendered)
}

// HasShowcase returns true if the profile has showcase data.
func HasShowcase(p gh.DatingProfile) bool {
	return p.Showcase != ""
}

// DecodeShowcase returns the raw markdown from the base64-encoded showcase.
func DecodeShowcase(p gh.DatingProfile) string {
	if p.Showcase == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(p.Showcase)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// ── Compact: single line ──

func renderCompact(p gh.DatingProfile, width int) string {
	name := theme.TextStyle.Render(p.DisplayName)
	if name == "" {
		name = theme.DimStyle.Render("anonymous")
	}

	parts := []string{name}
	if p.Bio != "" {
		parts = append(parts, theme.DimStyle.Render(truncate(p.Bio, 35)))
	}
	if p.Location != "" {
		parts = append(parts, theme.DimStyle.Render("· "+p.Location))
	}

	line := strings.Join(parts, "  ")
	if len(line) > width {
		line = line[:width-3] + "..."
	}
	return line
}

// ── Short: card without showcase/links ──

func renderShort(p gh.DatingProfile, width int) string {
	if width < 30 {
		width = 44
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(1, 2).
		Width(width)

	inner := width - 6
	var sections []string

	sections = append(sections, profileHeader(p, inner))

	if len(p.Interests) > 0 {
		sections = append(sections, profileInterests(p.Interests))
	}

	if badges := profileBadges(p.Intent, p.GenderTarget); badges != "" {
		sections = append(sections, badges)
	}

	if p.About != "" {
		about := truncate(p.About, inner*3)
		sections = append(sections, theme.DimStyle.Render("About")+"\n"+theme.TextStyle.Render(about))
	}

	return card.Render(strings.Join(sections, "\n\n"))
}

// ── Full: page 1 of brochure ──

func renderFull(p gh.DatingProfile, width int) string {
	if width < 30 {
		width = 56
	}

	card := lipgloss.NewStyle().
		Padding(1, 2).
		Width(width)

	inner := width - 6
	var sections []string

	// Header with avatar placeholder
	sections = append(sections, profileHeader(p, inner))

	// Interests
	if len(p.Interests) > 0 {
		sections = append(sections, profileInterests(p.Interests))
	}

	// Intent + gender
	if badges := profileBadges(p.Intent, p.GenderTarget); badges != "" {
		sections = append(sections, badges)
	}

	// Full about (word wrapped)
	if p.About != "" {
		lines := wordWrap(p.About, inner)
		sections = append(sections, theme.DimStyle.Render("About")+"\n"+theme.TextStyle.Render(strings.Join(lines, "\n")))
	}

	// Links
	if links := profileLinks(p.Website, p.Social); links != "" {
		sections = append(sections, links)
	}

	// Showcase indicator
	if p.Showcase != "" {
		sections = append(sections, theme.DimStyle.Render("→ Page 2: Showcase available"))
	}

	return card.Render(strings.Join(sections, "\n\n"))
}

// ── Shared section renderers ──

func profileHeader(p gh.DatingProfile, width int) string {
	name := theme.BrandStyle.Bold(true).Render(p.DisplayName)
	if p.DisplayName == "" {
		name = theme.DimStyle.Render("anonymous")
	}

	var lines []string
	lines = append(lines, name)

	if p.Bio != "" {
		b := p.Bio
		if len(b) > width {
			b = b[:width-3] + "..."
		}
		lines = append(lines, theme.TextStyle.Render(b))
	}

	if p.Location != "" {
		lines = append(lines, theme.DimStyle.Render("📍 "+p.Location))
	}

	return strings.Join(lines, "\n")
}

func profileInterests(interests []string) string {
	var tags []string
	for _, t := range interests {
		tag := lipgloss.NewStyle().
			Foreground(theme.Violet).
			Padding(0, 1).
			Render("#" + t)
		tags = append(tags, tag)
	}
	return strings.Join(tags, " ")
}

func profileBadges(intent []gh.Intent, genderTarget []gh.GenderTarget) string {
	var parts []string

	if len(intent) > 0 {
		var badges []string
		for _, i := range intent {
			icon := "♥"
			switch i {
			case gh.IntentFriendship:
				icon = "🤝"
			case gh.IntentYolo:
				icon = "🎲"
			case gh.IntentNetworking:
				icon = "🔗"
			}
			badges = append(badges, theme.BrandStyle.Render(icon+" "+i))
		}
		parts = append(parts, strings.Join(badges, "  "))
	}

	if len(genderTarget) > 0 {
		var targets []string
		for _, g := range genderTarget {
			if g == "dev" {
				targets = append(targets, "dev 💻")
			} else {
				targets = append(targets, string(g))
			}
		}
		parts = append(parts, theme.DimStyle.Render("→ ")+theme.TextStyle.Render(strings.Join(targets, ", ")))
	}

	return strings.Join(parts, "    ")
}

func profileLinks(website string, social []string) string {
	var lines []string
	if website != "" {
		lines = append(lines, theme.AccentStyle.Render("🌐 "+website))
	}
	for _, s := range social {
		lines = append(lines, theme.DimStyle.Render("🔗 "+s))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// ── Helpers ──

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func wordWrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var result []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			result = append(result, "")
			continue
		}
		words := strings.Fields(paragraph)
		line := ""
		for _, w := range words {
			if line == "" {
				line = w
			} else if len(line)+1+len(w) <= width {
				line += " " + w
			} else {
				result = append(result, line)
				line = w
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
