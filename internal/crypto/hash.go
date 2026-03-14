package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func UserHash(poolSecret, provider, providerUserID string) string {
	mac := hmac.New(sha256.New, []byte(poolSecret))
	mac.Write([]byte(provider + ":" + providerUserID))
	return hex.EncodeToString(mac.Sum(nil))
}
