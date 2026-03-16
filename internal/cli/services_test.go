package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func TestMockServices_Config(t *testing.T) {
	svc := MockServices()

	// Load returns empty config
	cfg, err := svc.Config.Load()
	if err != nil || cfg == nil {
		t.Fatal("expected empty config")
	}

	// Save and reload
	cfg.User.DisplayName = "Alice"
	cfg.AddPool(config.PoolConfig{Name: "pool1", Status: "active"})
	svc.Config.Save(cfg)

	loaded, _ := svc.Config.Load()
	if loaded.User.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", loaded.User.DisplayName)
	}
	if len(loaded.Pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(loaded.Pools))
	}

	// Paths
	if svc.Config.Dir() == "" {
		t.Error("Dir should not be empty")
	}
	if svc.Config.KeysDir() == "" {
		t.Error("KeysDir should not be empty")
	}
	if svc.Config.ProfilePath() == "" {
		t.Error("ProfilePath should not be empty")
	}
}

func TestMockServices_Crypto(t *testing.T) {
	svc := MockServices()

	// Generate
	pub, priv, err := svc.Crypto.GenerateKeyPair("/tmp/keys")
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil || priv == nil {
		t.Fatal("expected keys")
	}

	// Load returns same keys
	pub2, priv2, _ := svc.Crypto.LoadKeyPair("/tmp/keys")
	if string(pub) != string(pub2) {
		t.Error("load should return same pub")
	}
	if string(priv) != string(priv2) {
		t.Error("load should return same priv")
	}

	// Encrypt/decrypt round-trip
	encrypted, _ := svc.Crypto.Encrypt(pub, []byte("hello"))
	decrypted, err := svc.Crypto.Decrypt(priv, encrypted)
	if err != nil || string(decrypted) != "hello" {
		t.Errorf("round-trip failed: %v", err)
	}

	// Sign
	sig := svc.Crypto.Sign(priv, []byte("msg"))
	if sig == "" {
		t.Error("expected signature")
	}
}

func TestMockServices_Git(t *testing.T) {
	svc := MockServices()
	git := svc.Git.(*MockGitService)

	// Clone missing repo
	_, err := svc.Git.Clone("https://missing.git")
	if err == nil {
		t.Error("expected error")
	}

	// Add file and clone
	git.AddFile("https://example.com/repo.git", "README.md", []byte("hello"))
	repo, err := svc.Git.Clone("https://example.com/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if repo == nil {
		t.Fatal("expected repo")
	}

	// FetchRaw
	data, err := svc.Git.FetchRaw(context.Background(), "https://example.com/repo.git", "main", "README.md")
	if err != nil || string(data) != "hello" {
		t.Errorf("FetchRaw: %v", err)
	}

	// FetchRaw missing
	_, err = svc.Git.FetchRaw(context.Background(), "https://example.com/repo.git", "main", "missing.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}

	// FileExistsRaw
	if !svc.Git.FileExistsRaw(context.Background(), "https://example.com/repo.git", "main", "README.md") {
		t.Error("expected true")
	}
	if svc.Git.FileExistsRaw(context.Background(), "https://example.com/repo.git", "main", "nope") {
		t.Error("expected false")
	}

	// EnsureGitURL
	url := svc.Git.EnsureGitURL("owner/repo")
	if url != "https://github.com/owner/repo.git" {
		t.Errorf("unexpected: %s", url)
	}
}

func TestMockServices_GitHub(t *testing.T) {
	svc := MockServices()
	gh := svc.GitHub.(*MockGitHubService)

	// No user
	_, err := svc.GitHub.GetUser(context.Background(), "token")
	if err == nil {
		t.Error("expected error without user")
	}

	// Set user
	gh.User = &GitHubIdentity{UserID: "123", Username: "alice", DisplayName: "Alice"}
	user, err := svc.GitHub.GetUser(context.Background(), "token")
	if err != nil || user.Username != "alice" {
		t.Errorf("expected alice: %v", err)
	}

	// Token
	gh.Token = "ghp_test"
	token, err := svc.GitHub.ResolveToken(nil)
	if err != nil || token != "ghp_test" {
		t.Errorf("expected ghp_test: %v", err)
	}

	// Token error
	gh.TokenErr = fmt.Errorf("no gh")
	_, err = svc.GitHub.ResolveToken(nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDefaultServices_NotNil(t *testing.T) {
	svc := DefaultServices()
	if svc.Config == nil || svc.Crypto == nil || svc.Git == nil || svc.GitHub == nil {
		t.Error("all services should be non-nil")
	}
}
