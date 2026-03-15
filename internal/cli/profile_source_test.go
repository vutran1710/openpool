package cli

import (
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestParseDatingReadme_WithFrontmatter(t *testing.T) {
	content := `---
interests: [rust, hiking, coffee]
looking_for: [dating, friendship]
---

# About

Looking for someone to debug life with.
`
	result, err := parseDatingReadme([]byte(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(result.Interests) != 3 {
		t.Errorf("expected 3 interests, got %d", len(result.Interests))
	}
	if result.Interests[0] != "rust" {
		t.Errorf("expected rust, got %s", result.Interests[0])
	}
	if len(result.LookingFor) != 2 {
		t.Errorf("expected 2 looking_for, got %d", len(result.LookingFor))
	}
	if result.About != "Looking for someone to debug life with." {
		t.Errorf("unexpected about: %q", result.About)
	}
}

func TestParseDatingReadme_NoFrontmatter(t *testing.T) {
	content := "Just some text about me."
	result, err := parseDatingReadme([]byte(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.About != "Just some text about me." {
		t.Errorf("unexpected about: %q", result.About)
	}
	if len(result.Interests) != 0 {
		t.Errorf("expected no interests")
	}
}

func TestParseDatingReadme_EmptyFrontmatter(t *testing.T) {
	content := `---
---

Hello world
`
	result, err := parseDatingReadme([]byte(content))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.About != "Hello world" {
		t.Errorf("unexpected about: %q", result.About)
	}
}

func TestMergeProfiles(t *testing.T) {
	github := &gh.DatingProfile{
		DisplayName: "Alice",
		Bio:         "engineer",
		Location:    "Berlin",
		AvatarURL:   "https://avatar.com/alice",
	}

	dating := &gh.DatingProfile{
		Interests:  []string{"rust", "hiking"},
		LookingFor: []gh.LookingFor{"dating"},
		About:      "looking for fun",
	}

	merged := MergeProfiles(github, dating)

	if merged.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", merged.DisplayName)
	}
	if merged.Location != "Berlin" {
		t.Errorf("expected Berlin, got %s", merged.Location)
	}
	if len(merged.Interests) != 2 {
		t.Errorf("expected 2 interests, got %d", len(merged.Interests))
	}
	if merged.About != "looking for fun" {
		t.Errorf("expected about from dating, got %s", merged.About)
	}
}

func TestMergeProfiles_LaterOverrides(t *testing.T) {
	first := &gh.DatingProfile{Bio: "original"}
	second := &gh.DatingProfile{Bio: "updated"}

	merged := MergeProfiles(first, second)
	if merged.Bio != "updated" {
		t.Errorf("expected updated, got %s", merged.Bio)
	}
}

func TestMergeProfiles_NilSafe(t *testing.T) {
	result := MergeProfiles(nil, nil)
	if result.DisplayName != "" {
		t.Error("expected empty profile")
	}
}

func TestGenerateDatingReadme(t *testing.T) {
	readme := GenerateDatingReadme(
		[]string{"rust", "coffee"},
		[]gh.LookingFor{"dating", "friendship"},
		"Looking for someone cool",
	)

	if !containsStr(readme, "interests: [rust, coffee]") {
		t.Error("missing interests")
	}
	if !containsStr(readme, "looking_for: [dating, friendship]") {
		t.Error("missing looking_for")
	}
	if !containsStr(readme, "# About") {
		t.Error("missing about heading")
	}
	if !containsStr(readme, "Looking for someone cool") {
		t.Error("missing about text")
	}
}

func TestGenerateDatingReadme_Empty(t *testing.T) {
	readme := GenerateDatingReadme(nil, nil, "")
	if !containsStr(readme, "---") {
		t.Error("missing frontmatter delimiters")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
