package relay

import "crypto/ed25519"

// verifyEd25519 wraps ed25519.Verify for testability.
func verifyEd25519(pub ed25519.PublicKey, message, sig []byte) bool {
	return ed25519.Verify(pub, message, sig)
}
