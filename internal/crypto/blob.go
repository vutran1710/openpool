package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
)

// DecryptSignedBlob decrypts a "base64.hexsig" blob.
// Signature is assumed already verified by the caller.
// Returns raw plaintext bytes — caller unmarshals.
func DecryptSignedBlob(signedBlob string, priv ed25519.PrivateKey) ([]byte, error) {
	parts := strings.SplitN(signedBlob, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid signed blob format: no separator")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	return Decrypt(priv, ciphertext)
}
