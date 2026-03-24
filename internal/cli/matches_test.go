package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/message"
)

// signedComment creates a comment in the format wrapped in message.Format
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

func TestDecryptSignedBlob_Valid(t *testing.T) {
	_, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{
		"matched_bin_hash": "target_bin_hash1",
		"greeting":         "Hello!",
	})

	encrypted, err := crypto.Encrypt(userPub, payload)
	if err != nil {
		t.Fatal(err)
	}
	sig := ed25519.Sign(operatorPriv, encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

	plaintext, err := crypto.DecryptSignedBlob(signedBlob, priv)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		MatchedBinHash string `json:"matched_bin_hash"`
		Greeting       string `json:"greeting"`
	}
	if err := json.Unmarshal(plaintext, &data); err != nil {
		t.Fatal(err)
	}
	if data.MatchedBinHash != "target_bin_hash1" {
		t.Errorf("matched_bin_hash = %q", data.MatchedBinHash)
	}
	if data.Greeting != "Hello!" {
		t.Errorf("greeting = %q", data.Greeting)
	}
}

func TestDecryptSignedBlob_WrongKey(t *testing.T) {
	_, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)

	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "abc"})
	encrypted, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

	_, err := crypto.DecryptSignedBlob(signedBlob, wrongPriv)
	if err == nil {
		t.Fatal("should fail with wrong key")
	}
}

func TestDecryptSignedBlob_UnsignedBlob(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)

	_, err := crypto.DecryptSignedBlob("c29tZWJhc2U2NA==", priv)
	if err == nil {
		t.Fatal("unsigned blob should be rejected")
	}
}

func TestDecryptSignedBlob_EmptyGreeting(t *testing.T) {
	_, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload, _ := json.Marshal(map[string]string{"matched_bin_hash": "abc", "greeting": ""})

	encrypted, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

	plaintext, err := crypto.DecryptSignedBlob(signedBlob, priv)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		MatchedBinHash string `json:"matched_bin_hash"`
		Greeting       string `json:"greeting"`
	}
	if err := json.Unmarshal(plaintext, &data); err != nil {
		t.Fatal(err)
	}
	if data.MatchedBinHash != "abc" {
		t.Errorf("matched_bin_hash = %q", data.MatchedBinHash)
	}
	if data.Greeting != "" {
		t.Errorf("greeting should be empty, got %q", data.Greeting)
	}
}
