package components

import "github.com/vutran1710/openpool/internal/cli/tui/theme"

// ScreenLayout renders a consistent screen with header + content.
// All screens should use this for uniform spacing and structure.
//
// Example:
//
//	ScreenLayout("Profile", "mode: Normal  ·  e to edit", profileContent)
//	ScreenLayout("Discover", "browse profiles  ·  l to like", cardContent)
func ScreenLayout(title string, hints string, content string) string {
	header := theme.BoldStyle.Render(title)
	if hints != "" {
		header += "   " + hints
	}
	return "\n" + header + "\n\n" + content
}

// DimHints builds a dim hint string with dot separators.
//
// Example:
//
//	DimHints("browse profiles", "l to like")
//	→ "browse profiles  ·  l to like"
func DimHints(hints ...string) string {
	result := ""
	for i, hint := range hints {
		if i > 0 {
			result += "  " + theme.DimStyle.Render("·") + "  "
		}
		result += theme.DimStyle.Render(hint)
	}
	return result
}
