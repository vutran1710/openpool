package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// EphemeralHash computes a short-lived hash from a match_hash and expiry duration.
// The hash rotates every `expiry` duration based on the current time window.
//
//	window = unix_timestamp / expiry_seconds
//	ephemeral = sha256(match_hash + ":" + window)[:16]
func EphemeralHash(matchHash string, expiry time.Duration) string {
	window := time.Now().Unix() / int64(expiry.Seconds())
	return ephemeralHashAtWindow(matchHash, window)
}

// EphemeralHashAt computes the ephemeral hash for a specific unix timestamp.
func EphemeralHashAt(matchHash string, expiry time.Duration, ts time.Time) string {
	window := ts.Unix() / int64(expiry.Seconds())
	return ephemeralHashAtWindow(matchHash, window)
}

func ephemeralHashAtWindow(matchHash string, window int64) string {
	material := fmt.Sprintf("%s:%d", matchHash, window)
	h := sha256.Sum256([]byte(material))
	return hex.EncodeToString(h[:])[:16]
}
