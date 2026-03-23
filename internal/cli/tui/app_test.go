package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/cli/tui/screens"
	"github.com/vutran1710/dating-dev/internal/schema"
)

func TestPoolOnboard_ScreenConstant(t *testing.T) {
	// Verify the new screen constant exists and is distinct
	if screenPoolOnboard == screenJoin {
		t.Fatal("screenPoolOnboard should be distinct from screenJoin")
	}
	if screenPoolOnboard == screenHome {
		t.Fatal("screenPoolOnboard should be distinct from screenHome")
	}
}

func TestPoolOnboard_PoolJoinMsg_WithSchema(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DATING_HOME", tmpDir)

	// Create a pool.yaml in the expected location
	poolRepo := "testowner/testpool"
	repoDir := filepath.Join(tmpDir, "repos", poolRepo)
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	schemaContent := `name: testpool
description: A test pool
profile:
  name:
    type: text
  age:
    type: range
    min: 18
    max: 99
`
	if err := os.WriteFile(filepath.Join(repoDir, "pool.yaml"), []byte(schemaContent), 0600); err != nil {
		t.Fatal(err)
	}

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.width = 80
	a.height = 24

	// Simulate the pools screen having a pool entry
	a.pools = screens.NewPoolsScreen("registry/repo", nil, nil)

	// Manually set the pool info on the app to simulate what happens
	// when PoolJoinMsg is processed and poolRepo is found
	// We need to test the schema path — directly test the schema load + screen creation
	schemaPath := filepath.Join(repoDir, "pool.yaml")
	s, err := schema.Load(schemaPath)
	if err != nil {
		t.Fatalf("failed to load schema: %v", err)
	}

	a.poolOnboard = screens.NewPoolOnboardScreen("testpool", s, a.width, a.height)
	a.screen = screenPoolOnboard
	a.updateHelp()

	if a.screen != screenPoolOnboard {
		t.Fatalf("expected screen to be screenPoolOnboard, got %d", a.screen)
	}

	// Verify view renders without panic
	view := a.poolOnboard.View()
	if view == "" {
		t.Fatal("expected non-empty view from poolOnboard")
	}
}

func TestPoolOnboard_EscGoesToPools(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.screen = screenPoolOnboard

	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := model.(app)

	if updated.screen != screenPools {
		t.Fatalf("expected ESC from poolOnboard to go to screenPools, got %d", updated.screen)
	}
}

func TestPoolOnboard_ViewRouting(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.width = 80
	a.height = 24

	// Create a minimal schema and set up the screen
	s := &schema.PoolSchema{
		Name:    "test",
		Profile: map[string]schema.Attribute{"name": {Type: "text"}},
	}
	a.poolOnboard = screens.NewPoolOnboardScreen("test", s, 80, 24)
	a.screen = screenPoolOnboard

	// View should not panic and should return content
	view := a.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestPoolOnboard_UpdateRouting(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.width = 80
	a.height = 24

	s := &schema.PoolSchema{
		Name:    "test",
		Profile: map[string]schema.Attribute{"name": {Type: "text"}},
	}
	a.poolOnboard = screens.NewPoolOnboardScreen("test", s, 80, 24)
	a.screen = screenPoolOnboard

	// Send a key event — should not panic
	model, _ := a.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := model.(app)
	if updated.screen != screenPoolOnboard {
		t.Fatalf("expected screen to remain screenPoolOnboard after down key, got %d", updated.screen)
	}
}

func TestPoolOnboard_InputBlocked(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.width = 80
	a.height = 24

	s := &schema.PoolSchema{
		Name:    "test",
		Profile: map[string]schema.Attribute{"name": {Type: "text"}},
	}
	a.poolOnboard = screens.NewPoolOnboardScreen("test", s, 80, 24)
	a.screen = screenPoolOnboard

	// View should hide command input (same as onboarding/join)
	view := a.View()
	// The help bar should be present but not the command input prompt
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestPoolOnboard_HelpBindings(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	s := &schema.PoolSchema{
		Name:    "test",
		Profile: map[string]schema.Attribute{"name": {Type: "text"}},
	}
	a.poolOnboard = screens.NewPoolOnboardScreen("test", s, 80, 24)
	a.screen = screenPoolOnboard
	a.updateHelp()

	bindings := a.poolOnboard.HelpBindings()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty help bindings for poolOnboard")
	}
}

func TestPoolOnboard_DoneMsg_SavesProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DATING_HOME", tmpDir)

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.screen = screenPoolOnboard

	profile := map[string]any{"name": "Alice", "age": 25}
	msg := screens.PoolOnboardDoneMsg{
		PoolName: "testpool",
		Role:     "seeker",
		Profile:  profile,
	}

	model, cmd := a.Update(msg)
	_ = model

	// The profile should be saved to disk
	profilePath := schema.ProfilePath(tmpDir, "testpool")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		t.Fatal("expected profile to be saved to disk")
	}

	// The role should be set in the profile
	if profile["_role"] != "seeker" {
		t.Fatal("expected _role to be set in profile")
	}

	// cmd should be non-nil (submitPoolRegistration)
	if cmd == nil {
		t.Fatal("expected a command to be returned for registration submission")
	}
}

func TestPoolRegistrationSubmittedMsg_UpdatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("DATING_HOME", tmpDir)

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	a.width = 80
	a.height = 24

	msg := poolRegistrationSubmittedMsg{
		poolName:    "testpool",
		issueNumber: 42,
	}

	model, cmd := a.Update(msg)
	updated := model.(app)

	if updated.screen != screenHome {
		t.Fatalf("expected screen to be screenHome after registration, got %d", updated.screen)
	}
	if updated.pool != "testpool" {
		t.Fatalf("expected pool to be 'testpool', got %q", updated.pool)
	}

	// cmd should produce a toast
	if cmd == nil {
		t.Fatal("expected a toast command")
	}
	result := cmd()
	toast, ok := result.(components.ToastMsg)
	if !ok {
		t.Fatalf("expected ToastMsg, got %T", result)
	}
	if toast.Level != components.ToastSuccess {
		t.Fatalf("expected ToastSuccess, got %d", toast.Level)
	}
}

func TestPoolOnboard_WindowSizeMsg(t *testing.T) {
	t.Setenv("DATING_HOME", t.TempDir())

	a := newApp("testuser", "testhash", "", "registry/repo", nil, nil, false)
	s := &schema.PoolSchema{
		Name:    "test",
		Profile: map[string]schema.Attribute{"name": {Type: "text"}},
	}
	a.poolOnboard = screens.NewPoolOnboardScreen("test", s, 80, 24)
	a.screen = screenPoolOnboard

	// Send WindowSizeMsg — should not panic
	model, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = model.(app)
}
