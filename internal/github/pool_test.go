package github

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/message"
)

func TestDecryptSignedBlob_RegistrationFlow(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub := userPriv.Public().(ed25519.PublicKey)

	payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "abc123", "match_hash": "def456"})
	ciphertext, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, ciphertext)
	signedBlob := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)
	comment := message.Format("registration", signedBlob)

	// Simulate FindOperatorReplyInIssue extracting content
	_, content, err := message.Parse(comment)
	if err != nil {
		t.Fatal(err)
	}

	// Verify signature matches operator
	if !verifyBlobSignature(content, operatorPub) {
		t.Fatal("signature should verify")
	}

	// Decrypt via DecryptSignedBlob
	plaintext, err := crypto.DecryptSignedBlob(content, userPriv)
	if err != nil {
		t.Fatal(err)
	}

	var hashes map[string]string
	if err := msgpack.Unmarshal(plaintext, &hashes); err != nil {
		t.Fatal(err)
	}
	if hashes["bin_hash"] != "abc123" || hashes["match_hash"] != "def456" {
		t.Fatalf("got %v", hashes)
	}
}

func TestDecryptSignedBlob_TamperedCiphertext(t *testing.T) {
	operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub := userPriv.Public().(ed25519.PublicKey)

	payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "abc", "match_hash": "def"})
	ciphertext, _ := crypto.Encrypt(userPub, payload)
	sig := ed25519.Sign(operatorPriv, ciphertext)
	ciphertext[0] ^= 0xFF // tamper
	signedBlob := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)
	comment := message.Format("registration", signedBlob)

	_, content, err := message.Parse(comment)
	if err != nil {
		t.Fatal(err)
	}

	// Tampered ciphertext won't pass signature verification
	if verifyBlobSignature(content, operatorPub) {
		t.Fatal("tampered ciphertext should fail signature verification")
	}

	// Even if we bypass sig check, decryption should fail
	_, err = crypto.DecryptSignedBlob(content, userPriv)
	if err == nil {
		t.Fatal("tampered ciphertext should fail decryption")
	}
}
