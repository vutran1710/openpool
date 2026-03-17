package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

// LabelHashLen is the length used for GitHub label lookups (like:xxx, propose:xxx).
// Both write and read sides must use this constant.
const LabelHashLen = 12

func UserHash(poolRepo, provider, providerUserID string) string {
	h := sha256.Sum256([]byte(poolRepo + ":" + provider + ":" + providerUserID))
	return hex.EncodeToString(h[:])[:16]
}

// ShortHash returns a display-friendly truncated hash with "..." suffix.
// If the string is already short enough, returns it as-is.
func ShortHash(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "..."
}

// LabelHash returns the hash prefix used for GitHub label lookups.
func LabelHash(s string) string {
	if len(s) <= LabelHashLen {
		return s
	}
	return s[:LabelHashLen]
}
