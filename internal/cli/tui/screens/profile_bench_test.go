package screens

import (
	"encoding/base64"
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
	Showcase:     base64.StdEncoding.EncodeToString([]byte("# Alice\n\nRust developer\n\n## Stats\n\nSome stats here")),
}

func BenchmarkRenderProfileFull(b *testing.B) {
	for i := 0; i < b.N; i++ {
		components.RenderProfile(testProfile, 60, components.ProfileFull)
	}
}

func BenchmarkRenderProfileShort(b *testing.B) {
	for i := 0; i < b.N; i++ {
		components.RenderProfile(testProfile, 50, components.ProfileShort)
	}
}

func BenchmarkRenderProfileCompact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		components.RenderProfile(testProfile, 80, components.ProfileCompact)
	}
}

func BenchmarkRenderShowcase(b *testing.B) {
	for i := 0; i < b.N; i++ {
		renderShowcaseClean(testProfile, 60)
	}
}

func BenchmarkRenderShowcaseVsRaw(b *testing.B) {
	b.Run("glamour", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			renderShowcaseClean(testProfile, 60)
		}
	})
	b.Run("raw_decode_only", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			components.DecodeShowcase(testProfile)
		}
	})
}
