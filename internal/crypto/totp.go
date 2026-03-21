package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// TOTPSign computes a TOTP-style signature for relay auth.
// Signs sha256(timeWindow + relayHost) for channel binding.
func TOTPSign(priv ed25519.PrivateKey, relayHost string) string {
	tw := time.Now().Unix() / 300
	return TOTPSignAt(priv, relayHost, tw)
}

// TOTPSignAt computes a TOTP signature for a specific time window (for testing).
func TOTPSignAt(priv ed25519.PrivateKey, relayHost string, timeWindow int64) string {
	msg := totpMessage(relayHost, timeWindow)
	sig := ed25519.Sign(priv, msg)
	return hex.EncodeToString(sig)
}

// TOTPVerify checks a TOTP signature against current ± 1 time windows.
func TOTPVerify(sigHex string, pub ed25519.PublicKey, relayHost string) bool {
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	now := time.Now().Unix() / 300
	for _, tw := range []int64{now, now - 1, now + 1} {
		msg := totpMessage(relayHost, tw)
		if ed25519.Verify(pub, msg, sigBytes) {
			return true
		}
	}
	return false
}

func totpMessage(relayHost string, tw int64) []byte {
	h := sha256.Sum256([]byte(strconv.FormatInt(tw, 10) + relayHost))
	return h[:]
}
