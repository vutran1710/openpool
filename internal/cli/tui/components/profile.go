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
	ProfileCompact ProfileMode = iota // name, location, bio (slogan)
	ProfileNormal                     // card: header, interests, intent, about, links
)

// RenderProfile renders a DatingProfile in the specified mode.
func RenderProfile(p gh.DatingProfile, width int, mode ProfileMode) string {
	switch mode {
	case ProfileCompact:
		return renderCompact(p, width)
	case ProfileNormal:
		return renderNormal(p, width)
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
	name := theme.BoldStyle.Render(p.DisplayName)
	if p.DisplayName == "" {
		name = theme.DimStyle.Render("anonymous")
	}

	var lines []string
	lines = append(lines, name)

	if p.Location != "" {
		lines = append(lines, theme.DimStyle.Render("📍 "+p.Location))
	}

	if p.Bio != "" {
		bio := p.Bio
		if len(bio) > width-4 {
			bio = bio[:width-7] + "..."
		}
		lines = append(lines, theme.TextStyle.Render(bio))
	}

	return strings.Join(lines, "\n")
}

// ── Normal: card with all visible info ──

func renderNormal(p gh.DatingProfile, width int) string {
	if width < 30 {
		width = 44
	}

	card := lipgloss.NewStyle().
		Padding(1, 2)

	inner := width - 4
	var sections []string

	sections = append(sections, profileHeader(p, inner))

	if len(p.Interests) > 0 {
		sections = append(sections, profileInterests(p.Interests))
	}

	if badges := profileBadges(p.Intent, p.GenderTarget); badges != "" {
		sections = append(sections, badges)
	}

	if p.About != "" {
		lines := WordWrap(p.About, inner)
		sections = append(sections, theme.DimStyle.Render("About")+"\n"+theme.TextStyle.Render(strings.Join(lines, "\n")))
	}

	if links := profileLinks(p.Website, p.Social); links != "" {
		sections = append(sections, links)
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

// WordWrap wraps text to the given width, preserving newlines.
// Force-breaks words longer than width.
func WordWrap(s string, width int) []string {
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
			// Force-break words longer than width
			for len(w) > width {
				if line != "" {
					result = append(result, line)
					line = ""
				}
				result = append(result, w[:width])
				w = w[width:]
			}
			if w == "" {
				continue
			}
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
