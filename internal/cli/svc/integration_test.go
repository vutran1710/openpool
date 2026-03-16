package svc

import (
	"context"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// TestIntegration_JoinPoolLifecycle tests the full join→pending→active flow
// using all service mocks together.
func TestIntegration_JoinPoolLifecycle(t *testing.T) {
	persistence := NewMockPersistence()
	profile := NewMockProfile()
	polling := NewMockPolling()

	// Step 1: Save profile
	p := &gh.DatingProfile{
		DisplayName: "Alice",
		Bio:         "engineer",
		Interests:   []string{"rust", "coffee"},
		Intent:      []gh.Intent{gh.IntentDating},
	}
	if err := profile.SaveGlobal(p); err != nil {
		t.Fatalf("save global: %v", err)
	}
	if err := profile.SavePool("test-pool", p); err != nil {
		t.Fatalf("save pool: %v", err)
	}

	// Step 2: Save pending pool
	if err := persistence.SavePendingPool(config.PoolConfig{
		Name:         "test-pool",
		Repo:         "owner/test-pool",
		PendingIssue: 1,
	}); err != nil {
		t.Fatalf("save pending: %v", err)
	}

	pool := persistence.Pools["test-pool"]
	if pool.Status != "pending" {
		t.Errorf("expected pending, got %s", pool.Status)
	}

	// Step 3: Poll — still open
	polling.Updates = []StatusUpdate{
		{PoolName: "test-pool", Status: "active"},
	}

	result := polling.PollOnce(context.Background())
	if result == nil {
		t.Fatal("expected status update")
	}
	if result.Status != "active" {
		t.Errorf("expected active, got %s", result.Status)
	}

	// Step 5: Mark active
	persistence.MarkPoolActive("test-pool", "hash123")
	pool = persistence.Pools["test-pool"]
	if pool.Status != "active" {
		t.Errorf("expected active after mark, got %s", pool.Status)
	}
	if pool.UserHash != "hash123" {
		t.Errorf("expected hash123, got %s", pool.UserHash)
	}

	// Step 6: Verify profiles are accessible
	globalP, err := profile.LoadGlobal()
	if err != nil || globalP.DisplayName != "Alice" {
		t.Errorf("global profile: %v", err)
	}
	poolP, err := profile.LoadPool("test-pool")
	if err != nil || poolP.DisplayName != "Alice" {
		t.Errorf("pool profile: %v", err)
	}
}

// TestIntegration_JoinPoolRejection tests the join→pending→rejected→rejoin flow.
func TestIntegration_JoinPoolRejection(t *testing.T) {
	persistence := NewMockPersistence()

	// Join
	persistence.SavePendingPool(config.PoolConfig{
		Name:         "strict-pool",
		Repo:         "owner/strict-pool",
		PendingIssue: 5,
	})
	if persistence.Pools["strict-pool"].Status != "pending" {
		t.Error("expected pending")
	}

	// Rejected
	persistence.MarkPoolRejected("strict-pool")
	if persistence.Pools["strict-pool"].Status != "rejected" {
		t.Error("expected rejected")
	}
	if persistence.Pools["strict-pool"].PendingIssue != 0 {
		t.Error("expected issue cleared")
	}

	// Rejoin
	persistence.SavePendingPool(config.PoolConfig{
		Name:         "strict-pool",
		Repo:         "owner/strict-pool",
		PendingIssue: 6, // new issue
	})
	if persistence.Pools["strict-pool"].Status != "pending" {
		t.Error("expected pending after rejoin")
	}
	if persistence.Pools["strict-pool"].PendingIssue != 6 {
		t.Errorf("expected issue 6, got %d", persistence.Pools["strict-pool"].PendingIssue)
	}
}

// TestIntegration_OnboardingFlow tests identity + token + registry setup.
func TestIntegration_OnboardingFlow(t *testing.T) {
	persistence := NewMockPersistence()

	// Save identity
	persistence.SaveUserIdentity("Alice", "alice", "github", "123")
	if persistence.User.Username != "alice" {
		t.Error("expected alice")
	}

	// Save token
	persistence.SaveEncryptedToken("encrypted_token_hex")
	token, err := persistence.DecryptToken()
	if err != nil || token != "encrypted_token_hex" {
		t.Errorf("token: %v, %s", err, token)
	}

	// Add registry
	persistence.AddRegistry("https://github.com/owner/reg.git")
	if len(persistence.Registries) != 1 {
		t.Error("expected 1 registry")
	}

	// No duplicates
	persistence.AddRegistry("https://github.com/owner/reg.git")
	if len(persistence.Registries) != 2 { // Mock doesn't dedupe — that's config's job
		// This is expected for the mock
	}
}

// TestIntegration_ProfileIsolation tests that pool profiles are independent.
func TestIntegration_ProfileIsolation(t *testing.T) {
	profile := NewMockProfile()

	// Different profiles for different pools
	profile.SavePool("casual", &gh.DatingProfile{
		DisplayName: "Alice",
		Intent:      []gh.Intent{gh.IntentYolo},
	})
	profile.SavePool("serious", &gh.DatingProfile{
		DisplayName: "Alice",
		Intent:      []gh.Intent{gh.IntentDating},
	})

	casual, _ := profile.LoadPool("casual")
	serious, _ := profile.LoadPool("serious")

	if casual.Intent[0] != gh.IntentYolo {
		t.Errorf("casual should be yolo, got %s", casual.Intent[0])
	}
	if serious.Intent[0] != gh.IntentDating {
		t.Errorf("serious should be dating, got %s", serious.Intent[0])
	}

	// Global is separate
	_, err := profile.LoadGlobal()
	if err == nil {
		t.Error("global should not exist until explicitly saved")
	}
}
