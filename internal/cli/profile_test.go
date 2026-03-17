package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func TestProfileCreate_BaseTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cfg := &config.Config{}
	cfg.User.DisplayName = "Test User"
	cfg.Save()

	cmd := newProfileCreateCmd()
	cmd.SetArgs([]string{"test-pool"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	path := config.PoolProfilePath("test-pool")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("profile not created: %v", err)
	}

	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if profile["display_name"] != "Test User" {
		t.Errorf("display_name = %q, want 'Test User'", profile["display_name"])
	}
}

func TestProfileCreate_FromFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cfg := &config.Config{}
	cfg.Save()

	// Create a source profile
	srcProfile := map[string]any{
		"display_name": "Flagged User",
		"bio":          "from flag",
		"interests":    []string{"testing"},
	}
	srcData, _ := json.Marshal(srcProfile)
	srcPath := filepath.Join(dir, "custom-profile.json")
	os.WriteFile(srcPath, srcData, 0600)

	cmd := newProfileCreateCmd()
	cmd.SetArgs([]string{"test-pool", "--profile", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(config.PoolProfilePath("test-pool"))
	var profile map[string]any
	json.Unmarshal(data, &profile)

	if profile["display_name"] != "Flagged User" {
		t.Errorf("display_name = %q, want 'Flagged User'", profile["display_name"])
	}
	if profile["bio"] != "from flag" {
		t.Errorf("bio = %q, want 'from flag'", profile["bio"])
	}
}

func TestProfileCreate_DuplicateFromExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cfg := &config.Config{}
	cfg.AddPool(config.PoolConfig{Name: "existing-pool"})
	cfg.Save()

	// Create profile for existing pool
	existingProfile := map[string]any{
		"display_name": "Existing User",
		"bio":          "duplicated",
	}
	existingData, _ := json.Marshal(existingProfile)
	existingPath := config.PoolProfilePath("existing-pool")
	os.MkdirAll(filepath.Dir(existingPath), 0700)
	os.WriteFile(existingPath, existingData, 0600)

	cmd := newProfileCreateCmd()
	cmd.SetArgs([]string{"new-pool"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(config.PoolProfilePath("new-pool"))
	var profile map[string]any
	json.Unmarshal(data, &profile)

	if profile["display_name"] != "Existing User" {
		t.Errorf("display_name = %q, want 'Existing User'", profile["display_name"])
	}
}

func TestProfileCreate_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cfg := &config.Config{}
	cfg.Save()

	srcPath := filepath.Join(dir, "bad.json")
	os.WriteFile(srcPath, []byte("not json"), 0600)

	cmd := newProfileCreateCmd()
	cmd.SetArgs([]string{"test-pool", "--profile", srcPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestProfileShow_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cmd := newProfileShowCmd()
	cmd.SetArgs([]string{"nonexistent"})
	// Should not error — prints hint
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProfileEdit_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	cmd := newProfileEditCmd()
	cmd.SetArgs([]string{"nonexistent"})
	// Should not error — prints hint
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProfileEdit_Exists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)

	path := config.PoolProfilePath("test-pool")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte(`{"display_name":"test"}`), 0600)

	cmd := newProfileEditCmd()
	cmd.SetArgs([]string{"test-pool"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteProfileFile_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "profile.json")

	err := writeProfileFile(path, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
