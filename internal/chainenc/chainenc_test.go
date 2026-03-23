package chainenc

import (
	"testing"
)

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
