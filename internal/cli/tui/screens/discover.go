package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

type DiscoverScreen struct {
	Profile *gh.DatingProfile
	Loading bool
	Empty   bool
	Width   int
}

func NewDiscoverScreen() DiscoverScreen {
	// Mock profile for preview
	mock := gh.MockProfiles()
	return DiscoverScreen{
		Loading: false,
		Empty:   false,
		Profile: &mock[0],
	}
}

func (s DiscoverScreen) Update(msg tea.Msg) (DiscoverScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "l":
			if s.Profile != nil {
				return s, func() tea.Msg {
					return components.ToastMsg{
						Text:  fmt.Sprintf("Liked %s", s.Profile.DisplayName),
						Level: components.ToastSuccess,
					}
				}
			}
		case "s", "n":
			return s, func() tea.Msg {
				return components.ToastMsg{
					Text:  "Skipped — fetching next...",
					Level: components.ToastInfo,
				}
			}
		}
	}
	return s, nil
}

func (s DiscoverScreen) View() string {
	if s.Loading {
		return "\n  " + theme.DimStyle.Render("Fetching profiles...") + "\n"
	}

	if s.Profile == nil {
		return "\n  " + theme.DimStyle.Render("No more profiles. Check back later.") + "\n"
	}

	card := components.RenderProfile(*s.Profile, s.Width, components.ProfileShort)

	actions := fmt.Sprintf(
		"\n  %s  %s  %s",
		theme.BrandStyle.Render("[l]")+theme.TextStyle.Render(" like"),
		theme.DimStyle.Render("[s]")+theme.TextStyle.Render(" skip"),
		theme.DimStyle.Render("[v]")+theme.TextStyle.Render(" view full"),
	)

	return "\n  " + card + actions + "\n"
}

func (s DiscoverScreen) HelpBindings() []components.KeyBind {
	return []components.KeyBind{
		{Key: "l", Desc: "like"},
		{Key: "s", Desc: "skip"},
		{Key: "v", Desc: "view"},
		{Key: "esc", Desc: "back"},
	}
}
