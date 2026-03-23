package chainenc

import (
	"testing"
)

// === BuildChain tests ===

func TestBuildChain(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_alice", Data: []byte(`{"name":"Alice","age":27}`)},
		{Tag: "hash_bob", Data: []byte(`{"name":"Bob","age":28}`)},
		{Tag: "hash_carol", Data: []byte(`{"name":"Carol","age":26}`)},
	}

	chain, err := BuildChain(profiles, []byte("test_seed"), 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(chain.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(chain.Entries))
	}

	if chain.Entries[0].Tag != "hash_alice" {
		t.Errorf("expected hash_alice, got %s", chain.Entries[0].Tag)
	}

	for i, e := range chain.Entries {
		if len(e.Ciphertext) == 0 {
			t.Errorf("entry %d: empty ciphertext", i)
		}
		if len(e.Nonce) != 12 {
			t.Errorf("entry %d: nonce should be 12 bytes, got %d", i, len(e.Nonce))
		}
	}

	if chain.SeedHintVal < 0 || chain.SeedHintVal >= HintSpace {
		t.Errorf("seed hint out of range: %d", chain.SeedHintVal)
	}
}

func TestBuildChain_DifferentSeeds(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"x":1}`)},
	}

	c1, _ := BuildChain(profiles, []byte("seed_1"), 10)
	c2, _ := BuildChain(profiles, []byte("seed_2"), 10)

	if string(c1.Entries[0].Ciphertext) == string(c2.Entries[0].Ciphertext) {
		t.Error("different seeds should produce different ciphertexts")
	}
}

func TestBuildChain_Empty(t *testing.T) {
	_, err := BuildChain(nil, []byte("seed"), 10)
	if err == nil {
		t.Error("should fail with empty profiles")
	}
}

func TestProfileConstant(t *testing.T) {
	profile := []byte(`{"name":"Alice","age":27}`)

	c := ProfileConstant(profile)
	if c < 0 || c >= ConstSpace {
		t.Errorf("constant out of range: %d", c)
	}

	// Deterministic
	if c2 := ProfileConstant(profile); c != c2 {
		t.Error("should be deterministic")
	}

	// Different input → different constant (usually)
	c3 := ProfileConstant([]byte(`{"name":"Bob","age":28}`))
	if c == c3 {
		t.Log("warning: collision (possible but unlikely)")
	}
}

func TestProfileHint(t *testing.T) {
	profile := []byte(`{"name":"Alice","age":27}`)

	h := ProfileHint(profile)
	if h < 0 || h >= HintSpace {
		t.Errorf("hint out of range: %d", h)
	}

	if h2 := ProfileHint(profile); h != h2 {
		t.Error("should be deterministic")
	}
}

func TestDeriveKey(t *testing.T) {
	key1 := DeriveKey(12345678, 9999, 15, Orderings[0])
	key2 := DeriveKey(12345678, 9999, 15, Orderings[0])
	if key1 != key2 {
		t.Error("should be deterministic")
	}

	// Different ordering → different key
	key3 := DeriveKey(12345678, 9999, 15, Orderings[1])
	if key1 == key3 {
		t.Error("different ordering should produce different key")
	}

	// Different nonce → different key
	key4 := DeriveKey(12345678, 9999, 16, Orderings[0])
	if key1 == key4 {
		t.Error("different nonce should produce different key")
	}

	// Different hint → different key
	key5 := DeriveKey(99999999, 9999, 15, Orderings[0])
	if key1 == key5 {
		t.Error("different hint should produce different key")
	}
}

func TestPickOrdering(t *testing.T) {
	o1 := PickOrdering("context_a")
	o2 := PickOrdering("context_a")
	if o1 != o2 {
		t.Error("should be deterministic")
	}

	// Should produce a valid ordering
	valid := false
	for _, o := range Orderings {
		if o1 == o {
			valid = true
			break
		}
	}
	if !valid {
		t.Error("should produce a valid ordering")
	}
}

func TestSeedHint(t *testing.T) {
	h := SeedHint([]byte("bucket:women:25-30:perm0"))
	if h < 0 || h >= HintSpace {
		t.Errorf("seed hint out of range: %d", h)
	}
}

func TestChainContext(t *testing.T) {
	c1 := ChainContext([]byte("seed"), 0)
	c2 := ChainContext([]byte("seed"), 1)
	if c1 == c2 {
		t.Error("different positions should produce different contexts")
	}

	// Deterministic
	c3 := ChainContext([]byte("seed"), 0)
	if c1 != c3 {
		t.Error("should be deterministic")
	}

	// Non-empty
	if len(c1) == 0 {
		t.Error("context should not be empty")
	}
}
