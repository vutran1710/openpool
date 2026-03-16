package svc

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

type mockConfig struct {
	cfg *config.Config
}

func (m *mockConfig) Load() (*config.Config, error) { return m.cfg, nil }
func (m *mockConfig) Save(cfg *config.Config) error  { m.cfg = cfg; return nil }
func (m *mockConfig) Dir() string                    { return "/mock" }
func (m *mockConfig) KeysDir() string                { return "/mock/keys" }

type mockCrypto struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
}

func (m *mockCrypto) GenerateKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.pub, m.priv, nil
}
func (m *mockCrypto) LoadKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.pub, m.priv, nil
}
func (m *mockCrypto) Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(pub, plaintext)
}
func (m *mockCrypto) Decrypt(priv ed25519.PrivateKey, ct []byte) ([]byte, error) {
	return crypto.Decrypt(priv, ct)
}
func (m *mockCrypto) PackUserBin(a, b ed25519.PublicKey, p []byte) ([]byte, error) {
	return crypto.PackUserBin(a, b, p)
}
func (m *mockCrypto) Sign(priv ed25519.PrivateKey, msg []byte) string {
	return crypto.Sign(priv, msg)
}

func newTestPersistence() (PersistenceService, *mockConfig) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	cfg := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}
	return NewPersistenceService(cfg, cr), cfg
}

func TestPersistence_SavePendingPool(t *testing.T) {
	p, mc := newTestPersistence()

	err := p.SavePendingPool(config.PoolConfig{Name: "pool1", Repo: "owner/pool1"})
	if err != nil {
		t.Fatal(err)
	}

	if len(mc.cfg.Pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(mc.cfg.Pools))
	}
	if mc.cfg.Pools[0].Status != "pending" {
		t.Errorf("expected pending, got %s", mc.cfg.Pools[0].Status)
	}
	if mc.cfg.Active != "pool1" {
		t.Errorf("expected active pool1, got %s", mc.cfg.Active)
	}
}

func TestPersistence_MarkPoolActive(t *testing.T) {
	p, mc := newTestPersistence()
	p.SavePendingPool(config.PoolConfig{Name: "pool1", PendingIssue: 5})

	err := p.MarkPoolActive("pool1", "hash123")
	if err != nil {
		t.Fatal(err)
	}

	if mc.cfg.Pools[0].Status != "active" {
		t.Errorf("expected active, got %s", mc.cfg.Pools[0].Status)
	}
	if mc.cfg.Pools[0].UserHash != "hash123" {
		t.Errorf("expected hash123, got %s", mc.cfg.Pools[0].UserHash)
	}
	if mc.cfg.Pools[0].PendingIssue != 0 {
		t.Errorf("expected issue cleared, got %d", mc.cfg.Pools[0].PendingIssue)
	}
}

func TestPersistence_MarkPoolRejected(t *testing.T) {
	p, mc := newTestPersistence()
	p.SavePendingPool(config.PoolConfig{Name: "pool1", PendingIssue: 5})

	p.MarkPoolRejected("pool1")

	if mc.cfg.Pools[0].Status != "rejected" {
		t.Errorf("expected rejected, got %s", mc.cfg.Pools[0].Status)
	}
}

func TestPersistence_TokenRoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	mc := &mockConfig{cfg: &config.Config{}}
	cr := &mockCrypto{pub: pub, priv: priv}
	p := NewPersistenceService(mc, cr)

	// Encrypt and save token
	encrypted, _ := crypto.Encrypt(pub, []byte("ghp_test"))
	p.SaveEncryptedToken(hex.EncodeToString(encrypted))

	// Decrypt
	token, err := p.DecryptToken()
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if token != "ghp_test" {
		t.Errorf("expected ghp_test, got %s", token)
	}
}

func TestPersistence_DecryptToken_Empty(t *testing.T) {
	p, _ := newTestPersistence()
	_, err := p.DecryptToken()
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestPersistence_SaveUserIdentity(t *testing.T) {
	p, mc := newTestPersistence()

	p.SaveUserIdentity("Alice", "alice", "github", "123")

	if mc.cfg.User.DisplayName != "Alice" {
		t.Errorf("expected Alice")
	}
	if mc.cfg.User.Username != "alice" {
		t.Errorf("expected alice")
	}
}

func TestPersistence_AddRegistry(t *testing.T) {
	p, mc := newTestPersistence()

	p.AddRegistry("https://example.com/reg.git")

	if len(mc.cfg.Registries) != 1 {
		t.Errorf("expected 1 registry")
	}
	if mc.cfg.ActiveRegistry != "https://example.com/reg.git" {
		t.Errorf("expected active registry set")
	}

	// No duplicates
	p.AddRegistry("https://example.com/reg.git")
	if len(mc.cfg.Registries) != 1 {
		t.Errorf("expected no duplicate")
	}
}
