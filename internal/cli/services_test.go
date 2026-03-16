package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/svc"
)

func TestMockServices_Config(t *testing.T) {
	s := MockServices()

	cfg, err := s.Config.Load()
	if err != nil || cfg == nil {
		t.Fatal("expected empty config")
	}

	cfg.User.DisplayName = "Alice"
	cfg.AddPool(config.PoolConfig{Name: "pool1", Status: "active"})
	s.Config.Save(cfg)

	loaded, _ := s.Config.Load()
	if loaded.User.DisplayName != "Alice" {
		t.Errorf("expected Alice, got %s", loaded.User.DisplayName)
	}
	if len(loaded.Pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(loaded.Pools))
	}
}

func TestMockServices_Crypto(t *testing.T) {
	s := MockServices()

	pub, priv, err := s.Crypto.GenerateKeyPair("/tmp")
	if err != nil || pub == nil || priv == nil {
		t.Fatal("expected keys")
	}

	encrypted, _ := s.Crypto.Encrypt(pub, []byte("hello"))
	decrypted, err := s.Crypto.Decrypt(priv, encrypted)
	if err != nil || string(decrypted) != "hello" {
		t.Errorf("round-trip failed: %v", err)
	}

	sig := s.Crypto.Sign(priv, []byte("msg"))
	if sig == "" {
		t.Error("expected signature")
	}
}

func TestMockServices_Git(t *testing.T) {
	s := MockServices()
	git := s.Git.(*MockGitService)

	_, err := s.Git.Clone("https://missing.git")
	if err == nil {
		t.Error("expected error for missing repo")
	}

	git.AddFile("https://example.com/repo.git", "README.md", []byte("hello"))
	repo, err := s.Git.Clone("https://example.com/repo.git")
	if err != nil || repo == nil {
		t.Fatal("expected repo")
	}

	data, err := s.Git.FetchRaw(context.Background(), "https://example.com/repo.git", "main", "README.md")
	if err != nil || string(data) != "hello" {
		t.Errorf("FetchRaw: %v", err)
	}

	_, err = s.Git.FetchRaw(context.Background(), "https://example.com/repo.git", "main", "missing")
	if err == nil {
		t.Error("expected error for missing file")
	}

	if !s.Git.FileExistsRaw(context.Background(), "https://example.com/repo.git", "main", "README.md") {
		t.Error("expected true")
	}
}

func TestMockServices_GitHub(t *testing.T) {
	s := MockServices()
	gh := s.GitHub.(*MockGitHubService)

	// No user
	_, err := s.GitHub.GetUser(context.Background(), "token")
	if err == nil {
		t.Error("expected error without user")
	}

	// Set user
	gh.User = &svc.GitHubUser{UserID: "123", Username: "alice", DisplayName: "Alice"}
	user, err := s.GitHub.GetUser(context.Background(), "token")
	if err != nil || user.Username != "alice" {
		t.Errorf("expected alice: %v", err)
	}

	// Token
	gh.Token = "ghp_test"
	token, err := s.GitHub.ResolveToken(nil)
	if err != nil || token != "ghp_test" {
		t.Errorf("expected ghp_test: %v", err)
	}

	// Token error
	gh.TokenErr = fmt.Errorf("no gh")
	_, err = s.GitHub.ResolveToken(nil)
	if err == nil {
		t.Error("expected error")
	}

	// CreateIssue
	num, err := s.GitHub.CreateIssue(context.Background(), "repo", "token", "title", "body", nil)
	if err != nil || num == 0 {
		t.Errorf("create issue: %v", err)
	}

	// GetIssue
	state, _, err := s.GitHub.GetIssue(context.Background(), "repo", "token", num)
	if err != nil || state != "open" {
		t.Errorf("get issue: %v, state: %s", err, state)
	}

	// CloseIssue
	gh.CloseIssue(num, "completed")
	state, reason, _ := s.GitHub.GetIssue(context.Background(), "repo", "token", num)
	if state != "closed" || reason != "completed" {
		t.Errorf("expected closed/completed, got %s/%s", state, reason)
	}

	// GetIssue not found
	_, _, err = s.GitHub.GetIssue(context.Background(), "repo", "token", 999)
	if err == nil {
		t.Error("expected error for missing issue")
	}
}

func TestDefaultServices_NotNil(t *testing.T) {
	s := DefaultServices()
	if s.Config == nil || s.Crypto == nil || s.Git == nil || s.GitHub == nil {
		t.Error("all services should be non-nil")
	}
}
