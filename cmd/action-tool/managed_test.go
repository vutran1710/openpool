package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/schema"
)

func TestHashChain(t *testing.T) {
	salt := "testsalt123"
	pool := "owner/pool"
	provider := "google"
	userid := "user@example.com"

	idHash := string(crypto.UserHash(pool, provider, userid))
	binHash := sha256Short(salt + ":" + idHash)
	matchHash := sha256Short(salt + ":" + binHash)

	// Deterministic
	if id2 := string(crypto.UserHash(pool, provider, userid)); idHash != id2 {
		t.Error("UserHash should be deterministic")
	}

	// Lengths
	if len(idHash) != 64 {
		t.Errorf("id_hash should be 64 hex chars, got %d", len(idHash))
	}
	if len(binHash) != 16 {
		t.Errorf("bin_hash should be 16 hex chars, got %d", len(binHash))
	}
	if len(matchHash) != 16 {
		t.Errorf("match_hash should be 16 hex chars, got %d", len(matchHash))
	}

	// Different inputs → different hashes
	if id3 := string(crypto.UserHash(pool, provider, "other@example.com")); idHash == id3 {
		t.Error("different userids should produce different hashes")
	}
}

func TestBundleKeyGeneration(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")

	pub, _, err := crypto.GenerateKeyPair(keysDir)
	if err != nil {
		t.Fatal(err)
	}

	// Files exist and are hex-encoded
	pubHex, err := os.ReadFile(filepath.Join(keysDir, "identity.pub"))
	if err != nil {
		t.Fatal("identity.pub missing")
	}
	decoded, err := hex.DecodeString(string(pubHex))
	if err != nil {
		t.Fatal("identity.pub not valid hex")
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Errorf("pubkey wrong size: %d", len(decoded))
	}

	privHex, err := os.ReadFile(filepath.Join(keysDir, "identity.key"))
	if err != nil {
		t.Fatal("identity.key missing")
	}
	privDecoded, err := hex.DecodeString(string(privHex))
	if err != nil {
		t.Fatal("identity.key not valid hex")
	}
	if len(privDecoded) != ed25519.PrivateKeySize {
		t.Errorf("privkey wrong size: %d", len(privDecoded))
	}

	// PackUserBin works with generated keys
	opPub, _, _ := ed25519.GenerateKey(rand.Reader)
	binData, err := crypto.PackUserBin(pub, opPub, []byte(`{"age":25}`))
	if err != nil {
		t.Fatal("PackUserBin failed:", err)
	}
	if len(binData) < ed25519.PublicKeySize {
		t.Error("bin data too small")
	}
}

func TestProfileValidation(t *testing.T) {
	schemaYAML := `name: test
profile:
  age:
    type: range
    min: 18
    max: 100
  interests:
    type: multi
    values: hiking, coding, music
`
	schemaFile := filepath.Join(t.TempDir(), "pool.yaml")
	os.WriteFile(schemaFile, []byte(schemaYAML), 0600)

	s, err := schema.Load(schemaFile)
	if err != nil {
		t.Fatal(err)
	}

	// Valid
	valid := map[string]any{"age": 25, "interests": []any{"hiking"}}
	if errs := s.ValidateProfile(valid); len(errs) > 0 {
		t.Errorf("valid profile rejected: %v", errs)
	}

	// Invalid: age out of range
	invalid := map[string]any{"age": 150, "interests": []any{"hiking"}}
	if errs := s.ValidateProfile(invalid); len(errs) == 0 {
		t.Error("age=150 should be rejected")
	}

	// Invalid: bad interest
	invalid2 := map[string]any{"age": 25, "interests": []any{"swimming"}}
	if errs := s.ValidateProfile(invalid2); len(errs) == 0 {
		t.Error("invalid interest should be rejected")
	}
}
