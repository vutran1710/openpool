package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// PublicID is a client-computed pseudonymous identity. Full SHA256 (64 hex chars), never truncated.
// Conversion to string is explicit: string(id) at storage boundaries.
type PublicID string

// String implements fmt.Stringer for natural use in fmt.Sprintf etc.
func (id PublicID) String() string { return string(id) }

// Short returns a display-friendly truncated form with "..." suffix.
func (id PublicID) Short() string { return ShortHash(string(id)) }

// UserHash computes the PublicID for a user in a pool. Returns full SHA256 (64 hex chars).
func UserHash(poolRepo, provider, providerUserID string) PublicID {
	h := sha256.Sum256([]byte(poolRepo + ":" + provider + ":" + providerUserID))
	return PublicID(hex.EncodeToString(h[:]))
}

// ShortHash returns a display-friendly truncated hash with "..." suffix.
func ShortHash(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "..."
}
