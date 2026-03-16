package screens

import (
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

var testProfile = gh.DatingProfile{
	DisplayName:  "Alice",
	Bio:          "Backend engineer",
	Location:     "Berlin",
	Interests:    []string{"rust", "hiking", "coffee"},
	Intent:       []gh.Intent{gh.IntentDating, gh.IntentFriendship},
	GenderTarget: []gh.GenderTarget{gh.GenderDev},
	About:        "Looking for someone to debug life with",
	Website:      "https://alice.dev",
	Social:       []string{"https://twitter.com/alice"},
}

func BenchmarkRenderProfileNormal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		components.RenderProfile(testProfile, 60, components.ProfileNormal)
	}
}

func BenchmarkRenderProfileCompact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		components.RenderProfile(testProfile, 80, components.ProfileCompact)
	}
}
