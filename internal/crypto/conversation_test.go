package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

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
