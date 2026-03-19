package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

func genKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 key: %v", err)
	}
	return pub, priv
}

func TestSharedSecret_BothSidesMatch(t *testing.T) {
	pubA, privA := genKeys(t)
	pubB, privB := genKeys(t)

	secretAB, err := SharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("SharedSecret(A->B): %v", err)
	}
	secretBA, err := SharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("SharedSecret(B->A): %v", err)
	}

	if len(secretAB) != 32 {
		t.Fatalf("expected 32-byte secret, got %d", len(secretAB))
	}
	if string(secretAB) != string(secretBA) {
		t.Fatal("shared secrets do not match")
	}
}

func TestSharedSecret_DifferentPairs_DifferentSecrets(t *testing.T) {
	pubB, privA := genKeys(t)
	pubC, _ := genKeys(t)

	secretAB, err := SharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("SharedSecret(A->B): %v", err)
	}
	secretAC, err := SharedSecret(privA, pubC)
	if err != nil {
		t.Fatalf("SharedSecret(A->C): %v", err)
	}

	if string(secretAB) == string(secretAC) {
		t.Fatal("different pairs produced same secret")
	}
}

func TestDeriveConversationKey_UniquePerPair(t *testing.T) {
	shared := make([]byte, 32)
	if _, err := rand.Read(shared); err != nil {
		t.Fatal(err)
	}

	key1, err := DeriveConversationKey(shared, "aaa", "bbb", "https://pool.example")
	if err != nil {
		t.Fatalf("pair1: %v", err)
	}
	key2, err := DeriveConversationKey(shared, "aaa", "ccc", "https://pool.example")
	if err != nil {
		t.Fatalf("pair2: %v", err)
	}

	if string(key1) == string(key2) {
		t.Fatal("different pairs produced same key")
	}
}

func TestDeriveConversationKey_UniquePerPool(t *testing.T) {
	shared := make([]byte, 32)
	if _, err := rand.Read(shared); err != nil {
		t.Fatal(err)
	}

	key1, err := DeriveConversationKey(shared, "aaa", "bbb", "https://pool1.example")
	if err != nil {
		t.Fatalf("pool1: %v", err)
	}
	key2, err := DeriveConversationKey(shared, "aaa", "bbb", "https://pool2.example")
	if err != nil {
		t.Fatalf("pool2: %v", err)
	}

	if string(key1) == string(key2) {
		t.Fatal("different pools produced same key")
	}
}

func TestDeriveConversationKey_SortedHashes(t *testing.T) {
	shared := make([]byte, 32)
	if _, err := rand.Read(shared); err != nil {
		t.Fatal(err)
	}

	keyFwd, err := DeriveConversationKey(shared, "aaa", "zzz", "https://pool.example")
	if err != nil {
		t.Fatalf("forward order: %v", err)
	}
	keyRev, err := DeriveConversationKey(shared, "zzz", "aaa", "https://pool.example")
	if err != nil {
		t.Fatalf("reverse order: %v", err)
	}

	if string(keyFwd) != string(keyRev) {
		t.Fatal("key differs depending on hash argument order")
	}
}

func TestDeriveConversationKey_InvalidInput(t *testing.T) {
	shared32 := make([]byte, 32)

	cases := []struct {
		name    string
		shared  []byte
		hashA   string
		hashB   string
		poolURL string
	}{
		{"short shared", make([]byte, 16), "aaa", "bbb", "https://pool.example"},
		{"empty hashA", shared32, "", "bbb", "https://pool.example"},
		{"empty hashB", shared32, "aaa", "", "https://pool.example"},
		{"empty poolURL", shared32, "aaa", "bbb", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DeriveConversationKey(tc.shared, tc.hashA, tc.hashB, tc.poolURL)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestSealMessage_OpenMessage_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := "hello world"
	encoded, err := SealMessage(key, plaintext)
	if err != nil {
		t.Fatalf("SealMessage: %v", err)
	}

	got, err := OpenMessage(key, encoded)
	if err != nil {
		t.Fatalf("OpenMessage: %v", err)
	}
	if got != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, got)
	}
}

func TestSealMessage_UniqueNonce(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	enc1, err := SealMessage(key, "same plaintext")
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := SealMessage(key, "same plaintext")
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Fatal("two seals of the same plaintext produced identical ciphertext")
	}
}

func TestOpenMessage_WrongKey_Fails(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	wrongKey := make([]byte, 32)
	if _, err := rand.Read(wrongKey); err != nil {
		t.Fatal(err)
	}

	encoded, err := SealMessage(key, "secret")
	if err != nil {
		t.Fatal(err)
	}

	_, err = OpenMessage(wrongKey, encoded)
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}
}

func TestOpenMessage_Tampered_Fails(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	encoded, err := SealMessage(key, "secret message")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	tampered := base64.RawStdEncoding.EncodeToString(raw)

	_, err = OpenMessage(key, tampered)
	if err == nil {
		t.Fatal("expected error with tampered ciphertext, got nil")
	}
}

func TestOpenMessage_BadVersion_Fails(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	encoded, err := SealMessage(key, "test")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 0x99
	badVersion := base64.RawStdEncoding.EncodeToString(raw)

	_, err = OpenMessage(key, badVersion)
	if err == nil {
		t.Fatal("expected error with bad version byte, got nil")
	}
}

func TestSealMessage_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	encoded, err := SealMessage(key, "")
	if err != nil {
		t.Fatalf("SealMessage empty: %v", err)
	}

	got, err := OpenMessage(key, encoded)
	if err != nil {
		t.Fatalf("OpenMessage empty: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSealRaw_OpenRaw_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	plaintext := []byte("hello world")
	sealed, err := SealRaw(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if sealed[0] != 0x01 {
		t.Fatalf("version byte")
	}
	if len(sealed) < 1+24+secretbox.Overhead {
		t.Fatal("too short")
	}
	opened, err := OpenRaw(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "hello world" {
		t.Fatalf("got %q", opened)
	}
}

func TestOpenRaw_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)
	sealed, _ := SealRaw(key1, []byte("secret"))
	_, err := OpenRaw(key2, sealed)
	if err == nil {
		t.Fatal("wrong key should fail")
	}
}

func TestSealRaw_InvalidKeyLength(t *testing.T) {
	_, err := SealRaw([]byte("short"), []byte("data"))
	if err == nil {
		t.Fatal("short key should fail")
	}
}

func TestOpenRaw_TooShort(t *testing.T) {
	key := make([]byte, 32)
	_, err := OpenRaw(key, []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("too short should fail")
	}
}

func TestSealRaw_EmptyPlaintext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	sealed, err := SealRaw(key, []byte{})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenRaw(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) != 0 {
		t.Fatal("should be empty")
	}
}
