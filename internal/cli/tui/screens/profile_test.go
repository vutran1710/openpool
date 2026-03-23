package screens

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProfileScreen_InitialState(t *testing.T) {
	s := NewProfileScreen()
	if s.IsLoaded() {
		t.Error("should not be loaded initially")
	}
}

func TestProfileScreen_NoPool(t *testing.T) {
	s := NewProfileScreen()
	view := s.View()
	if !strings.Contains(view, "No active pool") {
		t.Error("should warn about no active pool")
	}
}

func TestProfileScreen_WithPool(t *testing.T) {
	s := NewProfileScreen()
	s.SetPool("test-pool")
	if s.poolName != "test-pool" {
		t.Error("expected test-pool")
	}
	view := s.View()
	if !strings.Contains(view, "Loading") {
		t.Error("should show loading before profile is loaded")
	}
}

func TestProfileScreen_ProfileLoaded(t *testing.T) {
	s := NewProfileScreen()
	s.SetPool("test-pool")
	profile := map[string]any{
		"about":     "Hello world",
		"interests": []any{"hiking", "coding"},
		"age":       float64(25),
	}
	s, _ = s.Update(profileLoadedMsg{profile: profile})
	s, _ = s.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	if !s.IsLoaded() {
		t.Error("should be loaded")
	}
	view := s.View()
	if !strings.Contains(view, "Hello world") {
		t.Error("should contain about text")
	}
	if !strings.Contains(view, "hiking") {
		t.Error("should contain interests")
	}
}

func TestProfileScreen_ProfileLoadError(t *testing.T) {
	s := NewProfileScreen()
	s.SetPool("test-pool")
	s, _ = s.Update(profileLoadedMsg{err: fmt.Errorf("bad json")})
	if s.err != "bad json" {
		t.Errorf("expected error, got %q", s.err)
	}
	view := s.View()
	if !strings.Contains(view, "bad json") {
		t.Error("should show error")
	}
}

func TestProfileScreen_HelpBindings(t *testing.T) {
	s := NewProfileScreen()
	bindings := s.HelpBindings()
	if len(bindings) == 0 {
		t.Error("expected at least 1 binding")
	}
}
