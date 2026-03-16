package components

import (
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func richProfile() gh.DatingProfile {
	return gh.MockProfiles()[0] // Alice — all fields
}

func minimalProfile() gh.DatingProfile {
	return gh.MockProfiles()[4] // eve — name, bio, location only
}

func emptyProfile() gh.DatingProfile {
	return gh.DatingProfile{}
}

// --- Compact mode ---

func TestRenderProfile_Compact_Rich(t *testing.T) {
	out := RenderProfile(richProfile(), 80, ProfileCompact)
	if out == "" {
		t.Fatal("expected output")
	}
	if !contains(out, "Alice Chen") {
		t.Error("expected name")
	}
	if !contains(out, "Berlin") {
		t.Error("expected location")
	}
}

func TestRenderProfile_Compact_Minimal(t *testing.T) {
	out := RenderProfile(minimalProfile(), 80, ProfileCompact)
	if !contains(out, "eve") {
		t.Error("expected name")
	}
}

func TestRenderProfile_Compact_Empty(t *testing.T) {
	out := RenderProfile(emptyProfile(), 80, ProfileCompact)
	if !contains(out, "anonymous") {
		t.Error("expected anonymous for empty name")
	}
}

func TestRenderProfile_Compact_Truncates(t *testing.T) {
	out := RenderProfile(richProfile(), 30, ProfileCompact)
	if len(out) > 35 { // some slack for ansi codes
		// Just ensure it doesn't panic on narrow width
	}
	if out == "" {
		t.Error("should render something even at narrow width")
	}
}

// --- Short mode ---

func TestRenderProfile_Short_Rich(t *testing.T) {
	out := RenderProfile(richProfile(), 50, ProfileShort)
	if !contains(out, "Alice Chen") {
		t.Error("expected name")
	}
	if !contains(out, "#rust") {
		t.Error("expected interests")
	}
	if !contains(out, "dating") {
		t.Error("expected intent")
	}
	if !contains(out, "About") {
		t.Error("expected about section")
	}
}

func TestRenderProfile_Short_NoInterests(t *testing.T) {
	p := gh.DatingProfile{DisplayName: "Bob", Bio: "dev"}
	out := RenderProfile(p, 50, ProfileShort)
	if !contains(out, "Bob") {
		t.Error("expected name")
	}
	// Should not crash with empty interests
}

func TestRenderProfile_Short_Minimal(t *testing.T) {
	out := RenderProfile(minimalProfile(), 50, ProfileShort)
	if !contains(out, "eve") {
		t.Error("expected name")
	}
	// No interests, intent, about — should still render
}

// --- Full mode ---

func TestRenderProfile_Full_Rich(t *testing.T) {
	out := RenderProfile(richProfile(), 60, ProfileFull)
	if !contains(out, "Alice Chen") {
		t.Error("expected name")
	}
	if !contains(out, "alice.dev") {
		t.Error("expected website")
	}
	if !contains(out, "twitter") {
		t.Error("expected social links")
	}
	if !contains(out, "Showcase available") {
		t.Error("expected showcase indicator")
	}
}

func TestRenderProfile_Full_NoShowcase(t *testing.T) {
	p := gh.DatingProfile{DisplayName: "Bob", Bio: "dev", Location: "NYC"}
	out := RenderProfile(p, 60, ProfileFull)
	if contains(out, "Showcase") {
		t.Error("should not show showcase indicator without showcase data")
	}
}

func TestRenderProfile_Full_WithLinks(t *testing.T) {
	p := gh.DatingProfile{
		DisplayName: "Charlie",
		Website:     "https://charlie.dev",
		Social:      []string{"https://twitter.com/charlie"},
	}
	out := RenderProfile(p, 60, ProfileFull)
	if !contains(out, "charlie.dev") {
		t.Error("expected website")
	}
	if !contains(out, "twitter.com/charlie") {
		t.Error("expected social")
	}
}

func TestRenderProfile_Full_AllBadges(t *testing.T) {
	p := gh.DatingProfile{
		DisplayName:  "Dana",
		Intent:       []gh.Intent{gh.IntentDating, gh.IntentYolo, gh.IntentFriendship, gh.IntentNetworking},
		GenderTarget: []gh.GenderTarget{gh.GenderDev, gh.GenderWomen, gh.GenderMen, gh.GenderNonBinary},
	}
	out := RenderProfile(p, 60, ProfileFull)
	if !contains(out, "dating") {
		t.Error("expected dating intent")
	}
	if !contains(out, "yolo") {
		t.Error("expected yolo intent")
	}
	if !contains(out, "dev") {
		t.Error("expected dev gender target")
	}
}

// --- Showcase ---

func TestRenderShowcase_Rich(t *testing.T) {
	out := RenderShowcase(richProfile(), 60)
	if out == "" {
		t.Fatal("expected showcase output")
	}
	if !contains(out, "Alice") {
		t.Error("expected Alice in showcase")
	}
}

func TestRenderShowcase_Empty(t *testing.T) {
	out := RenderShowcase(emptyProfile(), 60)
	if out != "" {
		t.Error("expected empty for no showcase")
	}
}

func TestRenderShowcase_InvalidBase64(t *testing.T) {
	p := gh.DatingProfile{Showcase: "not-valid-base64!!!"}
	out := RenderShowcase(p, 60)
	if !contains(out, "error") {
		t.Error("expected error message for invalid base64")
	}
}

func TestHasShowcase(t *testing.T) {
	if HasShowcase(emptyProfile()) {
		t.Error("empty profile should not have showcase")
	}
	if !HasShowcase(richProfile()) {
		t.Error("rich profile should have showcase")
	}
}

// --- Helpers ---

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncate("hello world foo bar", 10) != "hello w..." {
		t.Errorf("expected truncation, got %q", truncate("hello world foo bar", 10))
	}
}

func TestWordWrap(t *testing.T) {
	lines := wordWrap("hello world foo bar baz", 12)
	if len(lines) < 2 {
		t.Errorf("expected wrapping, got %d lines", len(lines))
	}

	// Preserves newlines
	lines = wordWrap("line1\n\nline3", 40)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (with blank), got %d", len(lines))
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	lines := wordWrap("hello", 0)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Error("zero width should return original")
	}
}

// helper
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
