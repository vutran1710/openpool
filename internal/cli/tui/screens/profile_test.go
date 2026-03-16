package screens

import (
	"encoding/base64"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestProfileScreen_InitialState(t *testing.T) {
	s := NewProfileScreen()
	if s.IsLoaded() {
		t.Error("should not be loaded initially")
	}
	if s.profile != nil {
		t.Error("profile should be nil")
	}
}

func TestProfileScreen_SetProfile(t *testing.T) {
	s := NewProfileScreen()
	p := &gh.DatingProfile{DisplayName: "Alice"}
	s.SetProfile(p)

	if !s.IsLoaded() {
		t.Error("should be loaded after SetProfile")
	}
	if s.profile.DisplayName != "Alice" {
		t.Error("expected Alice")
	}
}

func TestProfileScreen_ModeCycle(t *testing.T) {
	s := NewProfileScreen()
	s.SetProfile(&gh.DatingProfile{DisplayName: "Bob"})
	s.Width = 80
	s.Height = 40
	s.leftVP.Width = 40
	s.rightVP.Width = 40

	// Starts at ProfileFull (2)
	if s.mode != 2 {
		t.Errorf("expected mode 2 (Full), got %d", s.mode)
	}

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.mode != 0 { // → Compact
		t.Errorf("expected mode 0 (Compact), got %d", s.mode)
	}

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.mode != 1 { // → Short
		t.Errorf("expected mode 1 (Short), got %d", s.mode)
	}

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.mode != 2 { // → Full
		t.Errorf("expected mode 2 (Full), got %d", s.mode)
	}
}

func TestProfileScreen_ProfileLoaded(t *testing.T) {
	s := NewProfileScreen()
	p := &gh.DatingProfile{DisplayName: "Charlie", Bio: "dev"}

	s, _ = s.Update(profileLoadedMsg{profile: p})
	if !s.IsLoaded() {
		t.Error("should be loaded")
	}
	if s.profile.DisplayName != "Charlie" {
		t.Error("expected Charlie")
	}
}

func TestProfileScreen_ProfileLoadError(t *testing.T) {
	s := NewProfileScreen()
	s, _ = s.Update(profileLoadedMsg{err: fmt.Errorf("bad json")})
	if s.err != "bad json" {
		t.Errorf("expected error, got %q", s.err)
	}
}

func TestProfileScreen_HelpBindings(t *testing.T) {
	s := NewProfileScreen()
	bindings := s.HelpBindings()
	if len(bindings) != 3 {
		t.Errorf("expected 3 bindings, got %d", len(bindings))
	}
}

func TestRenderShowcaseClean_StripsImages(t *testing.T) {
	md := "# Title\n\n![badge](https://long-url.com/badge.svg)\n\nSome text"
	p := gh.DatingProfile{
		Showcase: base64.StdEncoding.EncodeToString([]byte(md)),
	}
	result := renderShowcaseClean(p, 60)

	if containsStr(result, "long-url.com/badge.svg") {
		t.Error("should strip image URLs")
	}
	if !containsStr(result, "Title") {
		t.Error("should keep headings")
	}
	if !containsStr(result, "Some text") {
		t.Error("should keep text")
	}
}

func TestRenderShowcaseClean_ShortensLinks(t *testing.T) {
	md := "[My Site](https://example.com/very/long/path/that/overflows)"
	p := gh.DatingProfile{
		Showcase: base64.StdEncoding.EncodeToString([]byte(md)),
	}
	result := renderShowcaseClean(p, 60)

	if containsStr(result, "https://example.com/very/long") {
		t.Error("should strip inline link URLs, keep text only")
	}
	if !containsStr(result, "My Site") {
		t.Error("should keep link text")
	}
}

func TestRenderShowcaseClean_Empty(t *testing.T) {
	p := gh.DatingProfile{}
	result := renderShowcaseClean(p, 60)
	if result != "" {
		t.Error("expected empty for no showcase")
	}
}

func TestMatchDetailScreen_PageFlip(t *testing.T) {
	md := "# Showcase\n\nHello world"
	p := &gh.DatingProfile{
		DisplayName: "Alice",
		Showcase:    base64.StdEncoding.EncodeToString([]byte(md)),
	}
	s := NewMatchDetailScreen(p, 60, 30)

	if s.page != 0 {
		t.Error("should start on page 0")
	}

	// Flip right
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.page != 1 {
		t.Errorf("expected page 1, got %d", s.page)
	}

	// Flip left
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if s.page != 0 {
		t.Errorf("expected page 0, got %d", s.page)
	}

	// Tab flip
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.page != 1 {
		t.Errorf("expected page 1 after tab, got %d", s.page)
	}
}

func TestMatchDetailScreen_NoShowcase_NoFlip(t *testing.T) {
	p := &gh.DatingProfile{DisplayName: "Bob"}
	s := NewMatchDetailScreen(p, 60, 30)

	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
	if s.page != 0 {
		t.Error("should not flip without showcase")
	}
}

func TestMatchDetailScreen_HelpBindings(t *testing.T) {
	p := &gh.DatingProfile{
		DisplayName: "Alice",
		Showcase:    base64.StdEncoding.EncodeToString([]byte("# Hi")),
	}
	s := NewMatchDetailScreen(p, 60, 30)
	bindings := s.HelpBindings()

	hasFlip := false
	for _, b := range bindings {
		if b.Key == "← →" {
			hasFlip = true
		}
	}
	if !hasFlip {
		t.Error("should have flip binding when showcase exists")
	}

	// Without showcase
	s2 := NewMatchDetailScreen(&gh.DatingProfile{DisplayName: "Bob"}, 60, 30)
	for _, b := range s2.HelpBindings() {
		if b.Key == "← →" {
			t.Error("should not have flip binding without showcase")
		}
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
