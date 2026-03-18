package components

import "github.com/vutran1710/dating-dev/internal/cli/tui/theme"

// ScreenHeader renders a consistent screen title with hint text.
// Example: "Profile   mode: Normal  tab to switch  ·  e to edit"
func ScreenHeader(title string, hints ...string) string {
	header := theme.BoldStyle.Render(title)
	for i, hint := range hints {
		if i > 0 {
			header += "  " + theme.DimStyle.Render("·")
		}
		header += "  " + theme.DimStyle.Render(hint)
	}
	return header
}
