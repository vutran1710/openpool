package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// IDHash is a client-computed pseudonymous identity: sha256(pool_url:provider:user_id).
// Full SHA256 (64 hex chars), never truncated.
type IDHash string

// String implements fmt.Stringer for natural use in fmt.Sprintf etc.
func (id IDHash) String() string { return string(id) }

// Short returns a display-friendly truncated form with "..." suffix.
func (id IDHash) Short() string { return ShortHash(string(id)) }

// UserHash computes the IDHash for a user in a pool. Returns full SHA256 (64 hex chars).
func UserHash(poolRepo, provider, providerUserID string) IDHash {
	h := sha256.Sum256([]byte(poolRepo + ":" + provider + ":" + providerUserID))
	return IDHash(hex.EncodeToString(h[:]))
}

// ShortHash returns a display-friendly truncated hash with "..." suffix.
func ShortHash(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "..."
}
