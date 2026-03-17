package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

func TestConfig_AddPool_NoDuplicates(t *testing.T) {
	cfg := &Config{}
	cfg.AddPool(PoolConfig{Name: "p1", Status: "pending"})
	cfg.AddPool(PoolConfig{Name: "p1", Status: "active"})

	if len(cfg.Pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(cfg.Pools))
	}
	if cfg.Pools[0].Status != "active" {
		t.Errorf("expected updated status, got %s", cfg.Pools[0].Status)
	}
}

func TestConfig_AddPool_Multiple(t *testing.T) {
	cfg := &Config{}
	cfg.AddPool(PoolConfig{Name: "p1"})
	cfg.AddPool(PoolConfig{Name: "p2"})

	if len(cfg.Pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(cfg.Pools))
	}
}

func TestConfig_RemovePool(t *testing.T) {
	cfg := &Config{Active: "p1"}
	cfg.AddPool(PoolConfig{Name: "p1"})
	cfg.AddPool(PoolConfig{Name: "p2"})

	if !cfg.RemovePool("p1") {
		t.Error("expected true")
	}
	if len(cfg.Pools) != 1 {
		t.Errorf("expected 1 pool, got %d", len(cfg.Pools))
	}
	if cfg.Active != "" {
		t.Error("active should be cleared")
	}
	if cfg.RemovePool("nonexistent") {
		t.Error("expected false for nonexistent")
	}
}

func TestConfig_AddRegistry_NoDuplicates(t *testing.T) {
	cfg := &Config{}
	cfg.AddRegistry("reg1")
	cfg.AddRegistry("reg1")

	if len(cfg.Registries) != 1 {
		t.Errorf("expected 1, got %d", len(cfg.Registries))
	}
}

func TestConfig_RemoveRegistry(t *testing.T) {
	cfg := &Config{ActiveRegistry: "reg1"}
	cfg.AddRegistry("reg1")

	if !cfg.RemoveRegistry("reg1") {
		t.Error("expected true")
	}
	if cfg.ActiveRegistry != "" {
		t.Error("active registry should be cleared")
	}
	if cfg.RemoveRegistry("nonexistent") {
		t.Error("expected false")
	}
}

func TestConfig_ActivePool(t *testing.T) {
	cfg := &Config{Active: "p2"}
	cfg.AddPool(PoolConfig{Name: "p1"})
	cfg.AddPool(PoolConfig{Name: "p2"})

	pool := cfg.ActivePool()
	if pool == nil || pool.Name != "p2" {
		t.Error("expected p2")
	}

	cfg.Active = "nonexistent"
	if cfg.ActivePool() != nil {
		t.Error("expected nil")
	}
}

func TestConfig_IsRegistered(t *testing.T) {
	cfg := &Config{}
	if cfg.IsRegistered() {
		t.Error("should not be registered")
	}
	cfg.User.IDHash = "abc"
	if !cfg.IsRegistered() {
		t.Error("should be registered")
	}
}

func TestConfig_HasToken(t *testing.T) {
	cfg := &Config{}
	if cfg.HasToken() {
		t.Error("should not have token")
	}
	cfg.User.EncryptedToken = "deadbeef"
	if !cfg.HasToken() {
		t.Error("should have token")
	}
}

func TestConfig_DecryptToken(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	token := "ghp_test123"
	encrypted, _ := crypto.Encrypt(pub, []byte(token))

	cfg := &Config{}
	cfg.User.EncryptedToken = hex.EncodeToString(encrypted)

	decrypted, err := cfg.DecryptToken(priv)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != token {
		t.Errorf("expected %s, got %s", token, decrypted)
	}
}

func TestConfig_DecryptToken_Empty(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	cfg := &Config{}
	_, err := cfg.DecryptToken(priv)
	if err == nil {
		t.Error("expected error")
	}
}

func TestConfig_DecryptToken_WrongKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)

	encrypted, _ := crypto.Encrypt(pub, []byte("token"))
	cfg := &Config{}
	cfg.User.EncryptedToken = hex.EncodeToString(encrypted)

	_, err := cfg.DecryptToken(wrongPriv)
	if err == nil {
		t.Error("expected error")
	}
}

func TestConfig_ProfilePath(t *testing.T) {
	p := ProfilePath()
	if filepath.Base(p) != "profile.json" {
		t.Errorf("expected profile.json, got %s", filepath.Base(p))
	}
}

func TestConfig_PoolConfig_Fields(t *testing.T) {
	p := PoolConfig{
		Name:         "test",
		Repo:         "owner/test",
		UserHash:     "abc123",
		PendingIssue: 42,
		Status:       "pending",
	}
	if p.UserHash != "abc123" {
		t.Error("UserHash")
	}
	if p.PendingIssue != 42 {
		t.Error("PendingIssue")
	}
}

func TestPoolConfig_BinHashMatchHash_Persist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATING_HOME", dir)
	cfg := &Config{}
	cfg.AddPool(PoolConfig{
		Name:      "test",
		Repo:      "owner/pool",
		BinHash:   "abcd1234abcd1234",
		MatchHash: "efgh5678efgh5678",
	})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	pool := loaded.Pools[0]
	if pool.BinHash != "abcd1234abcd1234" {
		t.Errorf("bin_hash = %q, want abcd1234abcd1234", pool.BinHash)
	}
	if pool.MatchHash != "efgh5678efgh5678" {
		t.Errorf("match_hash = %q, want efgh5678efgh5678", pool.MatchHash)
	}
}
