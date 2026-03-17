package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

// TOTPSign computes a TOTP-style signature for relay auth.
// Signs sha256(binHash + matchHash + timeWindow) with the user's private key.
func TOTPSign(binHash, matchHash string, priv ed25519.PrivateKey) string {
	tw := time.Now().Unix() / 300
	return TOTPSignAt(binHash, matchHash, priv, tw)
}

// TOTPSignAt computes a TOTP signature for a specific time window (for testing).
func TOTPSignAt(binHash, matchHash string, priv ed25519.PrivateKey, timeWindow int64) string {
	msg := totpMessage(binHash, matchHash, timeWindow)
	sig := ed25519.Sign(priv, msg)
	return hex.EncodeToString(sig)
}

// TOTPVerify checks a TOTP signature against current ± 1 time windows.
func TOTPVerify(binHash, matchHash, sigHex string, pub ed25519.PublicKey) bool {
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	now := time.Now().Unix() / 300
	for _, tw := range []int64{now, now - 1, now + 1} {
		msg := totpMessage(binHash, matchHash, tw)
		if ed25519.Verify(pub, msg, sigBytes) {
			return true
		}
	}
	return false
}

func totpMessage(binHash, matchHash string, tw int64) []byte {
	h := sha256.Sum256([]byte(binHash + matchHash + strconv.FormatInt(tw, 10)))
	return h[:]
}
