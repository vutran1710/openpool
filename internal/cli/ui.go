package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	brand    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	success  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7CDB8A"))
	warning  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD93D"))
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	dim      = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	bold     = lipgloss.NewStyle().Bold(true)

	profileBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF6B9D")).
			Padding(1, 2).
			Width(40)
)

func printBrand(msg string) {
	fmt.Println(brand.Render(msg))
}

func printSuccess(msg string) {
	fmt.Println(success.Render("  " + msg))
}

func printWarning(msg string) {
	fmt.Println(warning.Render("  " + msg))
}

func printError(msg string) {
	fmt.Println(errStyle.Render("  " + msg))
}

func printDim(msg string) {
	fmt.Println(dim.Render(msg))
}

func printHeader() {
	fmt.Println()
	printBrand("  dating v0.1.0")
	fmt.Println()
}
