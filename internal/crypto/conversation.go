package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/secretbox"
)

const e2eVersion byte = 0x01

// SharedSecret performs ECDH using the caller's ed25519 private key and
// the peer's ed25519 public key, returning a 32-byte shared secret.
func SharedSecret(myPriv ed25519.PrivateKey, theirPub ed25519.PublicKey) ([]byte, error) {
	myCurve := ed25519PrivToCurve25519(myPriv)
	theirCurve, err := ed25519PubToCurve25519(theirPub)
	if err != nil {
		return nil, fmt.Errorf("converting peer public key: %w", err)
	}
	secret, err := curve25519.X25519(myCurve[:], theirCurve[:])
	if err != nil {
		return nil, fmt.Errorf("curve25519 ECDH: %w", err)
	}
	return secret, nil
}

// DeriveConversationKey derives a 32-byte symmetric key for the conversation
// identified by the two user hashes and the pool URL.
// hashA and hashB are sorted so the same key is derived regardless of argument order.
func DeriveConversationKey(shared []byte, hashA, hashB, poolURL string) ([]byte, error) {
	if len(shared) != 32 {
		return nil, fmt.Errorf("shared secret must be 32 bytes, got %d", len(shared))
	}
	if hashA == "" {
		return nil, fmt.Errorf("hashA must not be empty")
	}
	if hashB == "" {
		return nil, fmt.Errorf("hashB must not be empty")
	}
	if poolURL == "" {
		return nil, fmt.Errorf("poolURL must not be empty")
	}

	if hashA > hashB {
		hashA, hashB = hashB, hashA
	}

	info := "dating-e2e:" + hashA + ":" + hashB + ":" + poolURL
	hk := hkdf.New(sha256.New, shared, nil, []byte(info))

	key := make([]byte, 32)
	if _, err := io.ReadFull(hk, key); err != nil {
		return nil, fmt.Errorf("deriving key: %w", err)
	}
	return key, nil
}

// SealMessage encrypts plaintext with a 32-byte symmetric key using NaCl secretbox.
// The returned string is base64 (RawStd) encoded: version(1) || nonce(24) || ciphertext.
func SealMessage(key []byte, plaintext string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	var k [32]byte
	copy(k[:], key)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := secretbox.Seal(nil, []byte(plaintext), &nonce, &k)

	out := make([]byte, 0, 1+24+len(ciphertext))
	out = append(out, e2eVersion)
	out = append(out, nonce[:]...)
	out = append(out, ciphertext...)

	return base64.RawStdEncoding.EncodeToString(out), nil
}

// SealRaw encrypts plaintext with a 32-byte key, returning raw bytes:
// version(1) || nonce(24) || secretbox ciphertext.
func SealRaw(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	var k [32]byte
	copy(k[:], key)
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext := secretbox.Seal(nil, plaintext, &nonce, &k)
	out := make([]byte, 0, 1+24+len(ciphertext))
	out = append(out, e2eVersion)
	out = append(out, nonce[:]...)
	out = append(out, ciphertext...)
	return out, nil
}

// OpenRaw decrypts raw bytes produced by SealRaw.
func OpenRaw(key, sealed []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	minLen := 1 + 24 + secretbox.Overhead
	if len(sealed) < minLen {
		return nil, fmt.Errorf("message too short: got %d bytes, need at least %d", len(sealed), minLen)
	}
	if sealed[0] != e2eVersion {
		return nil, fmt.Errorf("unsupported version byte: 0x%02x", sealed[0])
	}
	var nonce [24]byte
	copy(nonce[:], sealed[1:25])
	var k [32]byte
	copy(k[:], key)
	plaintext, ok := secretbox.Open(nil, sealed[25:], &nonce, &k)
	if !ok {
		return nil, fmt.Errorf("decryption failed: authentication tag mismatch")
	}
	return plaintext, nil
}

// OpenMessage decrypts a message produced by SealMessage.
func OpenMessage(key []byte, encoded string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	minLen := 1 + 24 + secretbox.Overhead
	if len(raw) < minLen {
		return "", fmt.Errorf("message too short: got %d bytes, need at least %d", len(raw), minLen)
	}

	if raw[0] != e2eVersion {
		return "", fmt.Errorf("unsupported version byte: 0x%02x", raw[0])
	}

	var nonce [24]byte
	copy(nonce[:], raw[1:25])
	ciphertext := raw[25:]

	var k [32]byte
	copy(k[:], key)

	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &k)
	if !ok {
		return "", fmt.Errorf("decryption failed: authentication tag mismatch")
	}

	return string(plaintext), nil
}
