package svc

import (
	"os"
	"path/filepath"
	"testing"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

func TestProfileService_GlobalRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	p := &gh.DatingProfile{
		DisplayName: "Alice",
		Bio:         "engineer",
		Interests:   []string{"rust", "coffee"},
	}

	if err := s.SaveGlobal(p); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadGlobal()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", loaded.DisplayName)
	}
	if len(loaded.Interests) != 2 {
		t.Errorf("expected 2 interests, got %d", len(loaded.Interests))
	}
}

func TestProfileService_PoolRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	p := &gh.DatingProfile{
		DisplayName: "Bob",
		Location:    "Berlin",
	}

	if err := s.SavePool("test-pool", p); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadPool("test-pool")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.DisplayName != "Bob" {
		t.Errorf("expected Bob, got %s", loaded.DisplayName)
	}
}

func TestProfileService_PoolIsolation(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	s.SavePool("pool-a", &gh.DatingProfile{DisplayName: "Alice"})
	s.SavePool("pool-b", &gh.DatingProfile{DisplayName: "Bob"})

	a, _ := s.LoadPool("pool-a")
	b, _ := s.LoadPool("pool-b")

	if a.DisplayName != "Alice" || b.DisplayName != "Bob" {
		t.Error("pool profiles should be isolated")
	}
}

func TestProfileService_LoadMissing(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	_, err := s.LoadGlobal()
	if err == nil {
		t.Error("expected error for missing profile")
	}

	_, err = s.LoadPool("nonexistent")
	if err == nil {
		t.Error("expected error for missing pool profile")
	}
}

func TestProfileService_Paths(t *testing.T) {
	s := NewProfileService(func() string { return "/home/user/.dating" })

	if filepath.Base(s.GlobalPath()) != "profile.json" {
		t.Errorf("unexpected global path: %s", s.GlobalPath())
	}

	poolPath := s.PoolPath("test")
	if !contains(poolPath, "pools") || !contains(poolPath, "test") {
		t.Errorf("unexpected pool path: %s", poolPath)
	}
}

func TestProfileService_OverwriteExisting(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	s.SaveGlobal(&gh.DatingProfile{DisplayName: "v1"})
	s.SaveGlobal(&gh.DatingProfile{DisplayName: "v2"})

	loaded, _ := s.LoadGlobal()
	if loaded.DisplayName != "v2" {
		t.Errorf("expected v2, got %s", loaded.DisplayName)
	}
}

func TestProfileService_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	s := NewProfileService(func() string { return tmp })

	path := s.GlobalPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("not json"), 0600)

	_, err := s.LoadGlobal()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
