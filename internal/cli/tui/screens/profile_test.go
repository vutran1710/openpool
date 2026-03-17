package screens

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestProfileScreen_InitialState(t *testing.T) {
	s := NewProfileScreen()
	if !s.IsLoaded() {
		t.Error("should be loaded after NewProfileScreen")
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

func TestProfileScreen_NormalModeAfterResize(t *testing.T) {
	s := NewProfileScreen()
	p := &gh.DatingProfile{
		DisplayName: "Alice",
		Bio:         "engineer",
		Location:    "Berlin",
		Interests:   []string{"rust", "go"},
		Intent:      []gh.Intent{"dating"},
		About:       "Hello world",
		Website:     "https://alice.dev",
	}
	s.SetProfile(p)
	s, _ = s.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	view := s.View()
	if !containsStr(view, "Alice") {
		t.Error("Normal mode View should contain name")
	}
	if !containsStr(view, "Profile") {
		t.Error("View should have Profile header")
	}
}

func TestProfileScreen_ModeCycle(t *testing.T) {
	s := NewProfileScreen()
	s.SetProfile(&gh.DatingProfile{DisplayName: "Bob"})
	s, _ = s.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	// Starts at Normal (1)
	if s.mode != components.ProfileNormal {
		t.Errorf("expected Normal, got %d", s.mode)
	}

	// Tab → Compact
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.mode != components.ProfileCompact {
		t.Errorf("expected Compact, got %d", s.mode)
	}

	// Tab → Normal
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.mode != components.ProfileNormal {
		t.Errorf("expected Normal, got %d", s.mode)
	}
}

func TestProfileScreen_AllModesHaveContent(t *testing.T) {
	s := NewProfileScreen()
	s.SetProfile(&gh.DatingProfile{
		DisplayName: "Bob",
		Bio:         "dev",
		Location:    "NYC",
		Interests:   []string{"go"},
		Intent:      []gh.Intent{"friendship"},
		About:       "Testing!",
	})
	s, _ = s.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	// Normal
	view := s.View()
	if !containsStr(view, "Bob") {
		t.Error("Normal mode: missing name")
	}

	// Compact
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = s.View()
	if !containsStr(view, "Bob") {
		t.Error("Compact mode: missing name")
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
	if len(bindings) != 4 {
		t.Errorf("expected 4 bindings, got %d", len(bindings))
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
