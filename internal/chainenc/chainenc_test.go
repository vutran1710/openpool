package chainenc

import (
	"testing"
)

// === Solve tests ===

func TestSolveCold(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"name":"Alice","age":27}`)},
		{Tag: "hash_b", Data: []byte(`{"name":"Bob","age":28}`)},
	}

	chain, err := BuildChain(profiles, []byte("test_seed"), 10)
	if err != nil {
		t.Fatal(err)
	}

	// Solve first entry cold
	payload, attempts, err := SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Profile["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", payload.Profile["name"])
	}
	if attempts == 0 {
		t.Error("attempts should be > 0")
	}
	t.Logf("cold solve: %d attempts", attempts)

	// Solve second entry with hint from first
	payload2, _, err := SolveCold(chain.Entries[1], payload.NextHint, ConstSpace, chain.NonceSpace)
	if err != nil {
		t.Fatal(err)
	}
	if payload2.Profile["name"] != "Bob" {
		t.Errorf("expected Bob, got %v", payload2.Profile["name"])
	}
}

func TestSolveWarm(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"name":"Alice","age":27}`)},
	}

	chain, err := BuildChain(profiles, []byte("test_seed"), 10)
	if err != nil {
		t.Fatal(err)
	}

	// Cold solve to learn constant
	payload, coldAttempts, _ := SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)

	// Build same profile with different seed (different permutation)
	chain2, _ := BuildChain(profiles, []byte("other_seed"), 10)

	// Warm solve
	payload2, warmAttempts, err := SolveWarm(chain2.Entries[0], chain2.SeedHintVal, payload.MyConstant, chain2.NonceSpace)
	if err != nil {
		t.Fatal(err)
	}
	if payload2.Profile["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", payload2.Profile["name"])
	}

	t.Logf("cold: %d attempts, warm: %d attempts", coldAttempts, warmAttempts)
	if warmAttempts >= coldAttempts {
		t.Errorf("warm (%d) should be fewer than cold (%d)", warmAttempts, coldAttempts)
	}
}

func TestSolveWithoutHint_Fails(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"name":"Alice"}`)},
	}

	chain, _ := BuildChain(profiles, []byte("seed"), 5)

	_, _, err := SolveCold(chain.Entries[0], 99999999, ConstSpace, chain.NonceSpace)
	if err == nil {
		t.Error("should fail with wrong hint")
	}
}

func TestFullChainSolve(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "h1", Data: []byte(`{"name":"A"}`)},
		{Tag: "h2", Data: []byte(`{"name":"B"}`)},
		{Tag: "h3", Data: []byte(`{"name":"C"}`)},
	}

	chain, _ := BuildChain(profiles, []byte("seed"), 5)

	hint := chain.SeedHintVal
	for i, entry := range chain.Entries {
		payload, _, err := SolveCold(entry, hint, ConstSpace, chain.NonceSpace)
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		hint = payload.NextHint
	}
}

// === Permutation jumping test ===

func TestPermutationJump(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "h1", Data: []byte(`{"name":"Alice"}`)},
		{Tag: "h2", Data: []byte(`{"name":"Bob"}`)},
		{Tag: "h3", Data: []byte(`{"name":"Carol"}`)},
	}

	chain0, _ := BuildChain(profiles, []byte("perm0"), 10)
	// Different order for perm1
	perm1 := []ProfileEntry{profiles[2], profiles[0], profiles[1]}
	chain1, _ := BuildChain(perm1, []byte("perm1"), 10)

	// Solve chain0 cold — learn all constants
	known := make(map[string]int)
	hint := chain0.SeedHintVal
	for _, entry := range chain0.Entries {
		payload, _, err := SolveCold(entry, hint, ConstSpace, chain0.NonceSpace)
		if err != nil {
			t.Fatal(err)
		}
		known[entry.Tag] = payload.MyConstant
		hint = payload.NextHint
	}

	// Solve chain1 warm
	hint = chain1.SeedHintVal
	totalWarm := 0
	for _, entry := range chain1.Entries {
		constant := known[entry.Tag]
		payload, attempts, err := SolveWarm(entry, hint, constant, chain1.NonceSpace)
		if err != nil {
			t.Fatal(err)
		}
		totalWarm += attempts
		hint = payload.NextHint
	}

	t.Logf("warm total: %d attempts for %d profiles", totalWarm, len(profiles))
	maxWarm := len(profiles) * chain1.NonceSpace * 6
	if totalWarm > maxWarm {
		t.Errorf("warm should not exceed %d, got %d", maxWarm, totalWarm)
	}
}

// === Benchmarks ===

func BenchmarkSolveCold(b *testing.B) {
	profile := ProfileEntry{Tag: "bench", Data: []byte(`{"name":"Bench","age":25}`)}
	chain, _ := BuildChain([]ProfileEntry{profile}, []byte("bench_seed"), 20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)
	}
}

func BenchmarkSolveWarm(b *testing.B) {
	profile := ProfileEntry{Tag: "bench", Data: []byte(`{"name":"Bench","age":25}`)}
	chain, _ := BuildChain([]ProfileEntry{profile}, []byte("bench_seed"), 20)
	payload, _, _ := SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SolveWarm(chain.Entries[0], chain.SeedHintVal, payload.MyConstant, chain.NonceSpace)
	}
}

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
