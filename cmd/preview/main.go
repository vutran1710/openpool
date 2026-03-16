// Preview renders mock profiles in all three modes.
// Run: go run ./cmd/preview
package main

import (
	"fmt"

	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func main() {
	profiles := gh.MockProfiles()
	alice := profiles[0]
	eve := profiles[4]

	section("COMPACT MODE (list items)")
	for i, p := range profiles {
		prefix := "  "
		if i == 0 {
			prefix = "❯ "
		}
		fmt.Println(prefix + components.RenderProfile(p, 80, components.ProfileCompact))
	}

	section("SHORT MODE (discovery card)")
	fmt.Println(components.RenderProfile(alice, 50, components.ProfileShort))

	section("FULL MODE (page 1 — profile brochure)")
	fmt.Println(components.RenderProfile(alice, 60, components.ProfileFull))

	section("SHOWCASE (page 2 — rendered markdown)")
	if components.HasShowcase(alice) {
		fmt.Println(components.RenderShowcase(alice, 60))
	}

	section("MINIMAL PROFILE (eve)")
	fmt.Println("Compact:", components.RenderProfile(eve, 80, components.ProfileCompact))
	fmt.Println()
	fmt.Println(components.RenderProfile(eve, 50, components.ProfileShort))
	fmt.Println()
	fmt.Println(components.RenderProfile(eve, 60, components.ProfileFull))
}

func section(title string) {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println(" ", title)
	fmt.Println("══════════════════════════════════════════════════")
	fmt.Println()
}
