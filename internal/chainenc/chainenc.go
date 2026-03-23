// Package chainenc implements chain encryption for rate-limited profile discovery.
//
// Profiles are encrypted into sequential chains where each entry reveals the hint
// needed to solve the next. Three security tiers:
//   - Without hint: infeasible (10^8 × 10^4 × N × 6 attempts)
//   - Cold (have hint): seconds (10^4 × N × 6 attempts)
//   - Warm (know constant): instant (N × 6 attempts)
package chainenc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	HintSpace  = 100_000_000 // 10^8 — 8-digit hint range
	ConstSpace = 10_000      // 10^4 — 4-digit profile constant range
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
	Profile    map[string]any `json:"profile"`
	MyConstant int            `json:"my_constant"`
	NextHint   int            `json:"next_hint"`
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

// ProfileEntry is the input to chain building.
type ProfileEntry struct {
	Tag  string // match_hash
	Data []byte // JSON profile content (plaintext)
}

// ProfileConstant derives the 4-digit constant from profile content.
func ProfileConstant(profileBytes []byte) int {
	h := sha256.Sum256(profileBytes)
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
