package svc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

type mockGitHub struct {
	issues map[string]struct{ state, reason string } // "repo:number" → state
}

func (m *mockGitHub) GetUser(_ context.Context, _ string) (*GitHubUser, error) { return nil, nil }
func (m *mockGitHub) ResolveToken(_ func(string) string) (string, error)      { return "", nil }
func (m *mockGitHub) CreateIssue(_ context.Context, _, _, _, _ string, _ []string) (int, error) {
	return 0, nil
}
func (m *mockGitHub) GetIssue(_ context.Context, repo, _ string, number int) (string, string, error) {
	key := repo + ":" + string(rune(number+'0'))
	if issue, ok := m.issues[key]; ok {
		return issue.state, issue.reason, nil
	}
	return "open", "", nil
}
func (m *mockGitHub) StarRepo(_ context.Context, _, _ string) error                      { return nil }
func (m *mockGitHub) CreateRepo(_ context.Context, _, _ string, _ bool) error             { return nil }
func (m *mockGitHub) CommitFile(_ context.Context, _, _, _, _ string, _ []byte) error      { return nil }
func (m *mockGitHub) RepoExists(_ context.Context, _, _ string) bool                      { return false }

func TestPolling_PollOnce_NoPending(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mc := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}

	// Save a token so DecryptToken works
	encrypted, _ := crypto.Encrypt(pub, []byte("token"))
	mc.cfg.User.EncryptedToken = hex.EncodeToString(encrypted)

	gh := &mockGitHub{issues: make(map[string]struct{ state, reason string })}
	s := NewPollingService(mc, cr, gh)

	result := s.PollOnce(context.Background())
	if result != nil {
		t.Error("expected nil when no pending pools")
	}
}

func TestPolling_PollOnce_PendingOpen(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mc := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}

	encrypted, _ := crypto.Encrypt(pub, []byte("token"))
	mc.cfg.User.EncryptedToken = hex.EncodeToString(encrypted)
	mc.cfg.AddPool(config.PoolConfig{Name: "pool1", Repo: "owner/pool1", Status: "pending", PendingIssue: 1})

	gh := &mockGitHub{issues: map[string]struct{ state, reason string }{
		"owner/pool1:1": {state: "open"},
	}}
	s := NewPollingService(mc, cr, gh)

	result := s.PollOnce(context.Background())
	if result != nil {
		t.Error("expected nil for still-open issue")
	}
}

func TestPolling_PollOnce_Completed(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mc := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}

	encrypted, _ := crypto.Encrypt(pub, []byte("token"))
	mc.cfg.User.EncryptedToken = hex.EncodeToString(encrypted)
	mc.cfg.AddPool(config.PoolConfig{Name: "pool1", Repo: "owner/pool1", Status: "pending", PendingIssue: 1})

	gh := &mockGitHub{issues: map[string]struct{ state, reason string }{
		"owner/pool1:1": {state: "closed", reason: "completed"},
	}}
	s := NewPollingService(mc, cr, gh)

	result := s.PollOnce(context.Background())
	if result == nil {
		t.Fatal("expected update")
	}
	if result.Status != "active" {
		t.Errorf("expected active, got %s", result.Status)
	}
	if result.PoolName != "pool1" {
		t.Errorf("expected pool1, got %s", result.PoolName)
	}

	// Check persistence updated
	if mc.cfg.Pools[0].Status != "active" {
		t.Errorf("expected config updated to active, got %s", mc.cfg.Pools[0].Status)
	}
}

func TestPolling_PollOnce_Rejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mc := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}

	encrypted, _ := crypto.Encrypt(pub, []byte("token"))
	mc.cfg.User.EncryptedToken = hex.EncodeToString(encrypted)
	mc.cfg.AddPool(config.PoolConfig{Name: "pool1", Repo: "owner/pool1", Status: "pending", PendingIssue: 1})

	gh := &mockGitHub{issues: map[string]struct{ state, reason string }{
		"owner/pool1:1": {state: "closed", reason: "not_planned"},
	}}
	s := NewPollingService(mc, cr, gh)

	result := s.PollOnce(context.Background())
	if result == nil {
		t.Fatal("expected update")
	}
	if result.Status != "rejected" {
		t.Errorf("expected rejected, got %s", result.Status)
	}
	if mc.cfg.Pools[0].Status != "rejected" {
		t.Errorf("expected config updated to rejected")
	}
}
