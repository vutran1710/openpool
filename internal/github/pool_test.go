package github

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func TestTryVerifyAndDecrypt_ValidSignature(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub := userPriv.Public().(ed25519.PublicKey)

	payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "abc123", "match_hash": "def456"})
	ciphertext, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, ciphertext)
	comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

	bin, match, err := tryDecryptComment(comment, operatorPub, userPriv)
	if err != nil {
		t.Fatal(err)
	}
	if bin != "abc123" || match != "def456" {
		t.Fatalf("got %s %s", bin, match)
	}
}

func TestTryVerifyAndDecrypt_ForgedSignature(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub := userPriv.Public().(ed25519.PublicKey)
	_, attackerPriv, _ := ed25519.GenerateKey(rand.Reader)

	payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "evil", "match_hash": "evil"})
	ciphertext, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(attackerPriv, ciphertext)
	comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

	_, _, err := tryDecryptComment(comment, operatorPub, userPriv)
	if err == nil {
		t.Fatal("forged signature should be rejected")
	}
}

func TestTryVerifyAndDecrypt_UnsignedComment(t *testing.T) {
	operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)

	_, _, err := tryDecryptComment("c29tZWJhc2U2NA==", operatorPub, userPriv)
	if err == nil {
		t.Fatal("unsigned should be rejected")
	}
}

func TestTryVerifyAndDecrypt_TamperedCiphertext(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub := userPriv.Public().(ed25519.PublicKey)

	payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "abc", "match_hash": "def"})
	ciphertext, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, ciphertext)
	ciphertext[0] ^= 0xFF // tamper
	comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

	_, _, err := tryDecryptComment(comment, operatorPub, userPriv)
	if err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}
