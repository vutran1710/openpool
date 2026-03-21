package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestDecryptSignedBlob_Valid(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	plaintext := []byte("secret data")
	encrypted, _ := Encrypt(pub, plaintext)
	sig := ed25519.Sign(priv, encrypted)
	blob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

	result, err := DecryptSignedBlob(blob, priv)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != "secret data" {
		t.Fatalf("got %q", result)
	}
}

func TestDecryptSignedBlob_InvalidFormat(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := DecryptSignedBlob("no-dot-separator", priv)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptSignedBlob_InvalidBase64(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := DecryptSignedBlob("not-base64!!!.aabbcc", priv)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecryptSignedBlob_WrongKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
	encrypted, _ := Encrypt(pub, []byte("data"))
	blob := base64.StdEncoding.EncodeToString(encrypted) + ".aabb"

	_, err := DecryptSignedBlob(blob, wrongPriv)
	if err == nil {
		t.Fatal("expected error with wrong key")
	}
}
