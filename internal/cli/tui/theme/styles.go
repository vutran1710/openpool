package theme

import "github.com/charmbracelet/lipgloss"

var (
	Pink    = lipgloss.Color("#FF6B9D")
	Violet  = lipgloss.Color("#A78BFA")
	Dim     = lipgloss.Color("#5C566E")
	Text    = lipgloss.Color("#E8E4F0")
	Surface = lipgloss.Color("#1A1528")
	Border  = lipgloss.Color("#2A2240")
	Green   = lipgloss.Color("#7CDB8A")
	Amber   = lipgloss.Color("#FBBF24")
	Red     = lipgloss.Color("#FF6B6B")

	BrandStyle = lipgloss.NewStyle().Foreground(Pink)
	DimStyle   = lipgloss.NewStyle().Foreground(Dim)
	TextStyle  = lipgloss.NewStyle().Foreground(Text)
	AccentStyle = lipgloss.NewStyle().Foreground(Violet)
	GreenStyle = lipgloss.NewStyle().Foreground(Green)
	AmberStyle = lipgloss.NewStyle().Foreground(Amber)
	RedStyle   = lipgloss.NewStyle().Foreground(Red)
	BoldStyle  = lipgloss.NewStyle().Foreground(Text).Bold(true)

	ActiveItem = lipgloss.NewStyle().Foreground(Pink).Bold(true)
	NormalItem = lipgloss.NewStyle().Foreground(Text)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border)

	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Violet)
)

func Logo() string {
	return BrandStyle.Render("♥") + " " + BoldStyle.Render("dating") + DimStyle.Render(".dev")
}

func Cursor() string {
	return BrandStyle.Render("❯ ")
}

// AccentPresets maps accent names to colors.
var AccentPresets = map[string]lipgloss.Color{
	"pink":   lipgloss.Color("#FF6B9D"),
	"violet": lipgloss.Color("#A78BFA"),
	"green":  lipgloss.Color("#7CDB8A"),
	"blue":   lipgloss.Color("#60A5FA"),
	"amber":  lipgloss.Color("#FBBF24"),
}

// SetAccent updates the brand/accent colors based on a preset name.
func SetAccent(name string) {
	color, ok := AccentPresets[name]
	if !ok {
		return
	}
	Pink = color
	BrandStyle = lipgloss.NewStyle().Foreground(color)
	ActiveItem = lipgloss.NewStyle().Foreground(color).Bold(true)
}
