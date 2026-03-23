# Chain Encryption Library — Implementation Plan (1 of 3)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the chain encryption library — the crypto foundation for rate-limited profile discovery.

**Architecture:** New `internal/chainenc` package. Pure crypto — no database, no schema, no UI. Accepts plaintext profiles + config, produces encrypted chains. Accepts encrypted chains + hints, solves entries. Fully testable in isolation.

**Tech Stack:** Go, `crypto/aes`, `crypto/cipher` (AES-256-GCM), `crypto/sha256`, `encoding/json`

**Spec:** `docs/superpowers/specs/2026-03-23-matching-engine-design.md` (Chain Encryption section)
**Reference:** `docs/concepts/chain-encryption-demo.py` (Python demo)

**Subsequent plans:**
- Plan 2: Indexer — uses this library to build `index.db` from `.bin` files
- Plan 3: Explorer — uses this library to grind chains client-side

---

## File Structure

```
internal/chainenc/
  chainenc.go       — types, constants, key derivation, ordering
  encrypt.go        — chain building (indexer side)
  decrypt.go        — chain solving (explorer side)
  chainenc_test.go  — all tests
```

---

### Task 1: Types and key derivation

**Files:**
- Create: `internal/chainenc/chainenc.go`
- Create: `internal/chainenc/chainenc_test.go`

- [ ] **Step 1: Write tests for key derivation and profile constant**

```go
// internal/chainenc/chainenc_test.go
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
	c2 := ProfileConstant(profile)
	if c != c2 {
		t.Error("ProfileConstant should be deterministic")
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

	// Deterministic
	h2 := ProfileHint(profile)
	if h != h2 {
		t.Error("ProfileHint should be deterministic")
	}
}

func TestDeriveKey(t *testing.T) {
	key1 := DeriveKey(12345678, 9999, 15, Orderings[0])
	key2 := DeriveKey(12345678, 9999, 15, Orderings[0])

	// Deterministic
	if key1 != key2 {
		t.Error("DeriveKey should be deterministic")
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
}

func TestPickOrdering(t *testing.T) {
	o1 := PickOrdering("context_a")
	o2 := PickOrdering("context_a")

	// Deterministic
	if o1 != o2 {
		t.Error("PickOrdering should be deterministic")
	}

	// Different context → likely different ordering
	o3 := PickOrdering("context_b")
	_ = o3 // may or may not differ (6 possible values)
}

func TestSeedHint(t *testing.T) {
	h := SeedHint([]byte("bucket:women:25-30:perm0"))
	if h < 0 || h >= HintSpace {
		t.Errorf("seed hint out of range: %d", h)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/chainenc/ -v`
Expected: compilation errors (package doesn't exist yet)

- [ ] **Step 3: Implement chainenc.go**

```go
// internal/chainenc/chainenc.go
package chainenc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	HintSpace  = 100_000_000 // 10^8 — 8 digits
	ConstSpace = 10_000      // 10^4 — 4 digits
)

// Ordering represents the order of [hint, constant, nonce] in key derivation.
type Ordering [3]int

// All 6 orderings of 3 elements (3! = 6).
var Orderings = [6]Ordering{
	{0, 1, 2}, // hint, constant, nonce
	{0, 2, 1}, // hint, nonce, constant
	{1, 0, 2}, // constant, hint, nonce
	{1, 2, 0}, // constant, nonce, hint
	{2, 0, 1}, // nonce, hint, constant
	{2, 1, 0}, // nonce, constant, hint
}

// Entry is a single encrypted profile in a chain.
type Entry struct {
	Tag        string // match_hash (plaintext, for skip detection)
	Ciphertext []byte // AES-256-GCM encrypted payload
	Nonce      []byte // 12-byte GCM nonce
	Context    string // chain context for ordering derivation
}

// Payload is the decrypted content of an entry.
type Payload struct {
	Profile     map[string]any `json:"profile"`
	MyConstant  int            `json:"my_constant"`
	NextHint    int            `json:"next_hint"`
}

// Chain is a sequence of encrypted entries for one permutation of one bucket.
type Chain struct {
	BucketID    string
	Permutation int
	Seed        []byte
	SeedHintVal int
	NonceSpace  int
	Entries     []Entry
}

// ProfileConstant derives the 4-digit constant from profile content.
func ProfileConstant(profileBytes []byte) int {
	h := sha256.Sum256(profileBytes)
	// Use first 8 bytes as uint64, mod ConstSpace
	val := uint64(0)
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(h[i])
	}
	return int(val % uint64(ConstSpace))
}

// ProfileHint derives the 8-digit hint from profile content.
func ProfileHint(profileBytes []byte) int {
	h := sha256.Sum256(append([]byte("hint:"), profileBytes...))
	val := uint64(0)
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(h[i])
	}
	return int(val % uint64(HintSpace))
}

// SeedHint derives the initial hint from a chain seed.
func SeedHint(seed []byte) int {
	h := sha256.Sum256(seed)
	val := uint64(0)
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(h[i])
	}
	return int(val % uint64(HintSpace))
}

// DeriveKey combines hint, constant, nonce in the given ordering into a 32-byte AES key.
func DeriveKey(hint, constant, nonce int, ordering Ordering) [32]byte {
	parts := [3]string{fmt.Sprintf("%d", hint), fmt.Sprintf("%d", constant), fmt.Sprintf("%d", nonce)}
	material := fmt.Sprintf("%s:%s:%s", parts[ordering[0]], parts[ordering[1]], parts[ordering[2]])
	return sha256.Sum256([]byte(material))
}

// PickOrdering selects one of 6 orderings deterministically from a context string.
func PickOrdering(context string) Ordering {
	h := sha256.Sum256([]byte(context))
	idx := int(h[0]) % 6
	return Orderings[idx]
}

// ChainContext produces a deterministic context string for a chain position.
func ChainContext(seed []byte, position int) string {
	material := fmt.Sprintf("%s:%d", hex.EncodeToString(seed), position)
	h := sha256.Sum256([]byte(material))
	return hex.EncodeToString(h[:8])
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/chainenc/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chainenc/chainenc.go internal/chainenc/chainenc_test.go
git commit -m "feat(chainenc): types, constants, key derivation, ordering"
```

---

### Task 2: Chain building (encrypt)

**Files:**
- Create: `internal/chainenc/encrypt.go`
- Modify: `internal/chainenc/chainenc_test.go`

- [ ] **Step 1: Write tests for chain building**

```go
// Add to chainenc_test.go

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

	// Tags should be plaintext
	if chain.Entries[0].Tag != "hash_alice" {
		t.Errorf("expected hash_alice, got %s", chain.Entries[0].Tag)
	}

	// Ciphertext should not be empty
	for i, e := range chain.Entries {
		if len(e.Ciphertext) == 0 {
			t.Errorf("entry %d: empty ciphertext", i)
		}
		if len(e.Nonce) != 12 {
			t.Errorf("entry %d: nonce should be 12 bytes, got %d", i, len(e.Nonce))
		}
	}

	// Seed hint should be set
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

	// Different seeds → different ciphertexts
	if string(c1.Entries[0].Ciphertext) == string(c2.Entries[0].Ciphertext) {
		t.Error("different seeds should produce different ciphertexts")
	}
}
```

- [ ] **Step 2: Run tests — should fail (BuildChain not defined)**

- [ ] **Step 3: Implement encrypt.go**

```go
// internal/chainenc/encrypt.go
package chainenc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
)

// ProfileEntry is the input to chain building.
type ProfileEntry struct {
	Tag  string // match_hash
	Data []byte // JSON profile content (plaintext)
}

// BuildChain encrypts a list of profiles into a chain.
// nonceSpace is the difficulty knob (number of possible nonce values).
func BuildChain(profiles []ProfileEntry, seed []byte, nonceSpace int) (*Chain, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("empty profile list")
	}

	// Pre-compute constants and hints
	constants := make([]int, len(profiles))
	hints := make([]int, len(profiles))
	for i, p := range profiles {
		constants[i] = ProfileConstant(p.Data)
		hints[i] = ProfileHint(p.Data)
	}

	seedHintVal := SeedHint(seed)
	currentHint := seedHintVal
	entries := make([]Entry, len(profiles))

	for i, p := range profiles {
		ctx := ChainContext(seed, i)
		ordering := PickOrdering(ctx)

		// Random nonce value within nonceSpace
		nonceVal, err := randInt(nonceSpace)
		if err != nil {
			return nil, fmt.Errorf("generating nonce: %w", err)
		}

		// Next hint: next profile's hint, or seed hint for last entry
		nextHint := seedHintVal
		if i+1 < len(profiles) {
			nextHint = hints[i+1]
		}

		// Build payload
		payload := Payload{
			Profile:    make(map[string]any),
			MyConstant: constants[i],
			NextHint:   nextHint,
		}
		json.Unmarshal(p.Data, &payload.Profile)

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling payload: %w", err)
		}

		// Derive AES key and encrypt
		key := DeriveKey(currentHint, constants[i], nonceVal, ordering)
		gcmNonce := make([]byte, 12)
		if _, err := rand.Read(gcmNonce); err != nil {
			return nil, fmt.Errorf("generating GCM nonce: %w", err)
		}

		block, err := aes.NewCipher(key[:])
		if err != nil {
			return nil, fmt.Errorf("creating cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("creating GCM: %w", err)
		}

		ciphertext := gcm.Seal(nil, gcmNonce, payloadBytes, nil)

		entries[i] = Entry{
			Tag:        p.Tag,
			Ciphertext: ciphertext,
			Nonce:      gcmNonce,
			Context:    ctx,
		}

		currentHint = nextHint
	}

	return &Chain{
		Seed:        seed,
		SeedHintVal: seedHintVal,
		NonceSpace:  nonceSpace,
		Entries:     entries,
	}, nil
}

func randInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/chainenc/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chainenc/encrypt.go internal/chainenc/chainenc_test.go
git commit -m "feat(chainenc): chain building with AES-256-GCM encryption"
```

---

### Task 3: Chain solving (decrypt)

**Files:**
- Create: `internal/chainenc/decrypt.go`
- Modify: `internal/chainenc/chainenc_test.go`

- [ ] **Step 1: Write tests for cold and warm solving**

```go
// Add to chainenc_test.go

func TestSolveCold(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"name":"Alice","age":27}`)},
		{Tag: "hash_b", Data: []byte(`{"name":"Bob","age":28}`)},
	}

	chain, err := BuildChain(profiles, []byte("test_seed"), 10)
	if err != nil {
		t.Fatal(err)
	}

	// Solve first entry cold (with seed hint)
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

	// Solve second entry cold (with hint from first)
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

	// First solve cold to learn the constant
	payload, coldAttempts, err := SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)
	if err != nil {
		t.Fatal(err)
	}

	// Build same profile with different seed (simulates different permutation)
	chain2, err := BuildChain(profiles, []byte("other_seed"), 10)
	if err != nil {
		t.Fatal(err)
	}

	// Solve warm using known constant
	payload2, warmAttempts, err := SolveWarm(chain2.Entries[0], chain2.SeedHintVal, payload.MyConstant, chain2.NonceSpace)
	if err != nil {
		t.Fatal(err)
	}
	if payload2.Profile["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", payload2.Profile["name"])
	}

	t.Logf("cold: %d attempts, warm: %d attempts", coldAttempts, warmAttempts)

	// Warm should be significantly fewer attempts
	if warmAttempts >= coldAttempts {
		t.Errorf("warm (%d) should be fewer than cold (%d)", warmAttempts, coldAttempts)
	}
}

func TestSolveWithoutHint_Fails(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "hash_a", Data: []byte(`{"name":"Alice"}`)},
	}

	chain, err := BuildChain(profiles, []byte("seed"), 5)
	if err != nil {
		t.Fatal(err)
	}

	// Try solving with wrong hint — should fail
	_, _, err = SolveCold(chain.Entries[0], 99999999, ConstSpace, chain.NonceSpace)
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

	// Solve entire chain sequentially
	hint := chain.SeedHintVal
	for i, entry := range chain.Entries {
		payload, _, err := SolveCold(entry, hint, ConstSpace, chain.NonceSpace)
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		hint = payload.NextHint
	}
}
```

- [ ] **Step 2: Run tests — should fail**

- [ ] **Step 3: Implement decrypt.go**

```go
// internal/chainenc/decrypt.go
package chainenc

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
)

// SolveCold solves an entry when the profile_constant is unknown.
// Brute-forces: constSpace × nonceSpace × 6 orderings.
// Returns the decrypted payload, number of attempts, or error.
func SolveCold(entry Entry, hint int, constSpace, nonceSpace int) (*Payload, int, error) {
	attempts := 0

	for constant := 0; constant < constSpace; constant++ {
		for nonce := 0; nonce < nonceSpace; nonce++ {
			for _, ordering := range Orderings {
				attempts++
				key := DeriveKey(hint, constant, nonce, ordering)
				payload, err := tryDecrypt(entry, key)
				if err == nil {
					return payload, attempts, nil
				}
			}
		}
	}

	return nil, attempts, fmt.Errorf("cold solve failed after %d attempts", attempts)
}

// SolveWarm solves an entry when the profile_constant is known.
// Brute-forces: nonceSpace × 6 orderings.
func SolveWarm(entry Entry, hint int, knownConstant int, nonceSpace int) (*Payload, int, error) {
	attempts := 0

	for nonce := 0; nonce < nonceSpace; nonce++ {
		for _, ordering := range Orderings {
			attempts++
			key := DeriveKey(hint, knownConstant, nonce, ordering)
			payload, err := tryDecrypt(entry, key)
			if err == nil {
				return payload, attempts, nil
			}
		}
	}

	return nil, attempts, fmt.Errorf("warm solve failed after %d attempts", attempts)
}

func tryDecrypt(entry Entry, key [32]byte) (*Payload, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, entry.Nonce, entry.Ciphertext, nil)
	if err != nil {
		return nil, err // wrong key — GCM auth tag mismatch
	}

	var payload Payload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/chainenc/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chainenc/decrypt.go internal/chainenc/chainenc_test.go
git commit -m "feat(chainenc): cold and warm solving with AES-256-GCM"
```

---

### Task 4: Benchmarks and integration test

**Files:**
- Modify: `internal/chainenc/chainenc_test.go`

- [ ] **Step 1: Add benchmark and permutation-jumping test**

```go
// Add to chainenc_test.go

func TestPermutationJump(t *testing.T) {
	profiles := []ProfileEntry{
		{Tag: "h1", Data: []byte(`{"name":"Alice"}`)},
		{Tag: "h2", Data: []byte(`{"name":"Bob"}`)},
		{Tag: "h3", Data: []byte(`{"name":"Carol"}`)},
	}

	// Build 2 permutations with different seeds
	chain0, _ := BuildChain(profiles, []byte("perm0"), 10)
	// Shuffle for perm1
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

	// Solve chain1 warm — should be much faster
	hint = chain1.SeedHintVal
	totalWarm := 0
	for _, entry := range chain1.Entries {
		constant, ok := known[entry.Tag]
		if !ok {
			t.Fatal("constant not found for", entry.Tag)
		}
		payload, attempts, err := SolveWarm(entry, hint, constant, chain1.NonceSpace)
		if err != nil {
			t.Fatal(err)
		}
		totalWarm += attempts
		hint = payload.NextHint
	}

	t.Logf("warm total: %d attempts for %d profiles", totalWarm, len(profiles))
	// Max warm attempts = len(profiles) * nonceSpace * 6 = 3 * 10 * 6 = 180
	if totalWarm > len(profiles)*chain1.NonceSpace*6 {
		t.Error("warm should not exceed nonceSpace * 6 per entry")
	}
}

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

	// Learn constant
	payload, _, _ := SolveCold(chain.Entries[0], chain.SeedHintVal, ConstSpace, chain.NonceSpace)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SolveWarm(chain.Entries[0], chain.SeedHintVal, payload.MyConstant, chain.NonceSpace)
	}
}
```

- [ ] **Step 2: Run all tests + benchmarks**

Run: `go test ./internal/chainenc/ -v -bench=. -benchmem`
Expected: all tests PASS, benchmarks show cold >> warm

- [ ] **Step 3: Run full test suite**

Run: `make test`
Expected: all packages pass

- [ ] **Step 4: Commit**

```bash
git add internal/chainenc/chainenc_test.go
git commit -m "test(chainenc): permutation jumping, benchmarks for cold vs warm"
```

- [ ] **Step 5: Push**

```bash
git push
```
