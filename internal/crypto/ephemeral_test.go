package crypto

import (
	"testing"
	"time"
)

func TestEphemeralHash_Deterministic(t *testing.T) {
	expiry := 3 * 24 * time.Hour // 3 days
	h1 := EphemeralHash("abc123", expiry)
	h2 := EphemeralHash("abc123", expiry)
	if h1 != h2 {
		t.Error("should be deterministic within same window")
	}
	if len(h1) != 16 {
		t.Errorf("expected 16 hex chars, got %d", len(h1))
	}
}

func TestEphemeralHash_DifferentInputs(t *testing.T) {
	expiry := 3 * 24 * time.Hour
	h1 := EphemeralHash("abc123", expiry)
	h2 := EphemeralHash("def456", expiry)
	if h1 == h2 {
		t.Error("different match_hashes should produce different ephemeral hashes")
	}
}

func TestEphemeralHashAt_WindowRotation(t *testing.T) {
	expiry := 3 * 24 * time.Hour
	matchHash := "abc123"

	now := time.Now()
	h1 := EphemeralHashAt(matchHash, expiry, now)

	// Same window
	h2 := EphemeralHashAt(matchHash, expiry, now.Add(1*time.Hour))
	if h1 != h2 {
		t.Error("within same window should produce same hash")
	}

	// Different window (4 days later)
	h3 := EphemeralHashAt(matchHash, expiry, now.Add(4*24*time.Hour))
	if h1 == h3 {
		t.Error("different windows should produce different hashes")
	}
}

func TestEphemeralHash_NotMatchHash(t *testing.T) {
	expiry := 3 * 24 * time.Hour
	matchHash := "abc123"
	ephemeral := EphemeralHash(matchHash, expiry)
	if ephemeral == matchHash {
		t.Error("ephemeral hash should not equal match_hash")
	}
}
