package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

func UserHash(pubKeyHex string) string {
	h := sha256.Sum256([]byte(pubKeyHex))
	return hex.EncodeToString(h[:])
}
