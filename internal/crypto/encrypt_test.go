package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	plaintext := []byte("hello world")
	encrypted, err := Encrypt(pub, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := Decrypt(priv, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(decrypted) != "hello world" {
		t.Errorf("expected 'hello world', got %q", decrypted)
	}
}

func TestEncryptDecrypt_DifferentKeys_Fails(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	_, priv2, _ := ed25519.GenerateKey(rand.Reader)

	encrypted, _ := Encrypt(pub1, []byte("secret"))
	_, err := Decrypt(priv2, encrypted)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	encrypted, err := Encrypt(pub, []byte{})
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(priv, encrypted)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("expected empty, got %d bytes", len(decrypted))
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := Decrypt(priv, []byte("short"))
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestPackUserBin_UnpackUserBin_RoundTrip(t *testing.T) {
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	opPub, opPriv, _ := ed25519.GenerateKey(rand.Reader)

	profile := map[string]string{
		"display_name": "Alice",
		"bio":          "engineer",
	}
	plaintext, _ := json.Marshal(profile)

	bin, err := PackUserBin(userPub, opPub, plaintext)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	// First 32 bytes should be user pubkey
	if len(bin) < ed25519.PublicKeySize {
		t.Fatal("bin too short")
	}
	for i := 0; i < ed25519.PublicKeySize; i++ {
		if bin[i] != userPub[i] {
			t.Fatalf("pubkey mismatch at byte %d", i)
		}
	}

	// Unpack with operator key
	decrypted, err := UnpackUserBin(opPriv, bin)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}

	var result map[string]string
	json.Unmarshal(decrypted, &result)
	if result["display_name"] != "Alice" {
		t.Errorf("expected Alice, got %s", result["display_name"])
	}
}

func TestPackUserBin_WrongKey_Fails(t *testing.T) {
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	opPub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)

	bin, _ := PackUserBin(userPub, opPub, []byte("secret"))
	_, err := UnpackUserBin(wrongPriv, bin)
	if err == nil {
		t.Error("expected error with wrong operator key")
	}
}

func TestUnpackUserBin_TooShort(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, err := UnpackUserBin(priv, []byte("short"))
	if err == nil {
		t.Error("expected error for short bin")
	}
}

func TestReEncryptForRecipient(t *testing.T) {
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	opPub, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	recipientPub, recipientPriv, _ := ed25519.GenerateKey(rand.Reader)

	plaintext := []byte("profile data")
	bin, _ := PackUserBin(userPub, opPub, plaintext)

	reEncrypted, err := ReEncryptForRecipient(opPriv, bin, recipientPub)
	if err != nil {
		t.Fatalf("re-encrypt: %v", err)
	}

	decrypted, err := Decrypt(recipientPriv, reEncrypted)
	if err != nil {
		t.Fatalf("decrypt re-encrypted: %v", err)
	}

	if string(decrypted) != "profile data" {
		t.Errorf("expected 'profile data', got %q", decrypted)
	}
}

func TestSign_Verify(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)

	message := []byte("test message")
	sig := Sign(priv, message)

	if !Verify(pub, message, sig) {
		t.Error("signature should verify")
	}

	if Verify(pub, []byte("wrong message"), sig) {
		t.Error("signature should not verify for wrong message")
	}
}

func TestVerify_InvalidHex(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	if Verify(pub, []byte("test"), "not-hex") {
		t.Error("should fail for invalid hex signature")
	}
}

func TestUserHash_Deterministic(t *testing.T) {
	h1 := UserHash("repo", "github", "123")
	h2 := UserHash("repo", "github", "123")
	if h1 != h2 {
		t.Error("same inputs should produce same hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 chars (full SHA256 hex), got %d", len(h1))
	}
}

func TestUserHash_DifferentInputs(t *testing.T) {
	h1 := UserHash("repo1", "github", "123")
	h2 := UserHash("repo2", "github", "123")
	if h1 == h2 {
		t.Error("different repos should produce different hashes")
	}
}
