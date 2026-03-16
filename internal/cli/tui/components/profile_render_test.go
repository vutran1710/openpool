package components

import (
	"strings"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

// TestRenderFull_WithRealData tests full mode with a real-world profile at realistic width.
func TestRenderFull_WithRealData(t *testing.T) {
	p := gh.DatingProfile{
		DisplayName:  "Vu Tran",
		Bio:          "making softwares for human",
		Location:     "Vietnam, Hanoi",
		Website:      "https://www.linkedin.com/in/vutr/",
		Interests:    []string{"guitar", "rock", "novel"},
		Intent:       []gh.Intent{"dating", "friendship", "yolo", "networking"},
		GenderTarget: []gh.GenderTarget{"men", "women", "non-binary", "dev"},
		About:        "Author of this shit!",
	}

	widths := []int{30, 40, 50, 60, 80, 100}
	for _, w := range widths {
		result := RenderProfile(p, w, ProfileNormal)
		if result == "" {
			t.Errorf("Full mode at width %d: empty output", w)
			continue
		}
		lines := strings.Split(result, "\n")
		if len(lines) < 3 {
			t.Errorf("Full mode at width %d: only %d lines, expected more. Content:\n%s", w, len(lines), result)
		}
		if !contains(result, "Vu Tran") {
			t.Errorf("Full mode at width %d: missing name", w)
		}
		if !contains(result, "guitar") {
			t.Errorf("Full mode at width %d: missing interests", w)
		}
		if !contains(result, "dating") {
			t.Errorf("Full mode at width %d: missing intent", w)
		}
		if !contains(result, "About") {
			t.Errorf("Full mode at width %d: missing about section", w)
		}
		if !contains(result, "linkedin") {
			t.Errorf("Full mode at width %d: missing website", w)
		}
	}
}

// TestRenderAllModes_NonEmpty ensures all modes produce output.
func TestRenderAllModes_NonEmpty(t *testing.T) {
	p := gh.DatingProfile{
		DisplayName:  "Alice",
		Bio:          "engineer",
		Location:     "Berlin",
		Interests:    []string{"rust"},
		Intent:       []gh.Intent{"dating"},
		GenderTarget: []gh.GenderTarget{"dev"},
		About:        "Hello!",
		Website:      "https://alice.dev",
	}

	modes := []ProfileMode{ProfileCompact, ProfileNormal}
	modeNames := []string{"Compact", "Normal"}

	for i, mode := range modes {
		result := RenderProfile(p, 60, mode)
		if result == "" {
			t.Errorf("%s mode: empty output", modeNames[i])
		}
		if !contains(result, "Alice") {
			t.Errorf("%s mode: missing name", modeNames[i])
		}
	}
}

// TestRenderFull_ContentLength verifies full mode has substantially more content than compact.
func TestRenderFull_ContentLength(t *testing.T) {
	p := gh.DatingProfile{
		DisplayName:  "Bob",
		Bio:          "developer",
		Location:     "NYC",
		Interests:    []string{"go", "rust", "hiking"},
		Intent:       []gh.Intent{"dating", "friendship"},
		GenderTarget: []gh.GenderTarget{"women", "dev"},
		About:        "Looking for someone who gets concurrency jokes.",
		Website:      "https://bob.dev",
		Social:       []string{"https://twitter.com/bob"},
	}

	compact := RenderProfile(p, 60, ProfileCompact)
	full := RenderProfile(p, 60, ProfileNormal)

	if len(full) <= len(compact) {
		t.Errorf("Full mode (%d chars) should be longer than Compact (%d chars)", len(full), len(compact))
	}

	// Full should have sections that compact doesn't
	if !contains(full, "About") {
		t.Error("Full mode should have About section")
	}
	if !contains(full, "bob.dev") {
		t.Error("Full mode should have website")
	}
	if !contains(full, "twitter") {
		t.Error("Full mode should have social links")
	}
}
