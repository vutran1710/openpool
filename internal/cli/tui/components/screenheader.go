package components

import "github.com/vutran1710/openpool/internal/cli/tui/theme"

// ScreenHeader renders a consistent screen title with hint text.
// Hints are rendered dim. Use ScreenHeaderRaw for pre-styled hints.
// Adds a blank line before the header for consistent spacing from the app header.
func ScreenHeader(title string, hints ...string) string {
	header := theme.BoldStyle.Render(title)
	for i, hint := range hints {
		if i > 0 {
			header += "  " + theme.DimStyle.Render("·")
		}
		header += "  " + theme.DimStyle.Render(hint)
	}
	return "\n" + header
}

// ScreenHeaderRaw renders a title + pre-styled hint string.
// Use this when hints need mixed styling (e.g., "mode: " + accent("Normal")).
func ScreenHeaderRaw(title string, styledHints string) string {
	return "\n" + theme.BoldStyle.Render(title) + "  " + styledHints
}
