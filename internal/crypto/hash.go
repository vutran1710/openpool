package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

func UserHash(poolRepo, provider, providerUserID string) string {
	h := sha256.Sum256([]byte(poolRepo + ":" + provider + ":" + providerUserID))
	return hex.EncodeToString(h[:])
}
