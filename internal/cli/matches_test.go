package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/message"
)

// signedComment creates a comment in the new format wrapped in message.Format
func signedComment(t *testing.T, operatorPriv ed25519.PrivateKey, userPub ed25519.PublicKey, payload []byte) string {
	t.Helper()
	encrypted, err := crypto.Encrypt(userPub, payload)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(operatorPriv, encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)
	return message.Format("match", signedBlob)
}

func TestDecryptMatchComment_Valid(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{
		"matched_bin_hash": "target_bin_hash1",
		"greeting":         "Hello!",
	})
	body := signedComment(t, operatorPriv, userPub, payload)

	m, err := decryptMatchComment(body, operatorPub, priv)
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

func TestDecryptMatchComment_ForgedSignature(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, attackerPriv, _ := ed25519.GenerateKey(rand.Reader)

	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "evil"})
	body := signedComment(t, attackerPriv, userPub, payload) // signed by attacker, not operator

	_, err := decryptMatchComment(body, operatorPub, priv)
	if err == nil {
		t.Fatal("forged signature should be rejected")
	}
}

func TestDecryptMatchComment_UnsignedComment(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	_, err := decryptMatchComment("c29tZWJhc2U2NA==", operatorPub, priv)
	if err == nil {
		t.Fatal("unsigned comment should be rejected")
	}
}

func TestDecryptMatchComment_MissingBinHash(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"greeting": "hi"})
	body := signedComment(t, operatorPriv, userPub, payload)

	_, err := decryptMatchComment(body, operatorPub, priv)
	if err == nil {
		t.Fatal("expected error for missing bin_hash")
	}
}

func TestDecryptMatchComment_EmptyGreeting(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "abc", "greeting": ""})
	body := signedComment(t, operatorPriv, userPub, payload)

	m, err := decryptMatchComment(body, operatorPub, priv)
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
