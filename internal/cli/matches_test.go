package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

func TestDecryptMatchComment_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{
		"matched_bin_hash": "target_bin_hash1",
		"greeting":         "Hello!",
	})
	encrypted, _ := crypto.Encrypt(pub, payload)
	body := base64.StdEncoding.EncodeToString(encrypted)

	m, err := decryptMatchComment(body, priv)
	if err != nil {
		t.Fatal(err)
	}
	if m.BinHash != "target_bin_hash1" {
		t.Errorf("bin_hash = %q", m.BinHash)
	}
	if m.Greeting != "Hello!" {
		t.Errorf("greeting = %q", m.Greeting)
	}
}

func TestDecryptMatchComment_WrongKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "x"})
	encrypted, _ := crypto.Encrypt(pub, payload)
	body := base64.StdEncoding.EncodeToString(encrypted)

	_, err := decryptMatchComment(body, wrongPriv)
	if err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestDecryptMatchComment_InvalidBase64(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := decryptMatchComment("not-base64!!!", priv)
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptMatchComment_MissingBinHash(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"greeting": "hi"})
	encrypted, _ := crypto.Encrypt(pub, payload)
	body := base64.StdEncoding.EncodeToString(encrypted)

	_, err := decryptMatchComment(body, priv)
	if err == nil {
		t.Fatal("expected error for missing bin_hash")
	}
}

func TestDecryptMatchComment_EmptyGreeting(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "abc", "greeting": ""})
	encrypted, _ := crypto.Encrypt(pub, payload)
	body := base64.StdEncoding.EncodeToString(encrypted)

	m, err := decryptMatchComment(body, priv)
	if err != nil {
		t.Fatal(err)
	}
	if m.BinHash != "abc" {
		t.Errorf("bin_hash = %q", m.BinHash)
	}
	if m.Greeting != "" {
		t.Errorf("greeting should be empty, got %q", m.Greeting)
	}
}
