package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func GenerateKeyPair(keysDir string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generating key pair: %w", err)
	}

	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("creating keys dir: %w", err)
	}

	pubPath := filepath.Join(keysDir, "identity.pub")
	privPath := filepath.Join(keysDir, "identity.key")

	if err := os.WriteFile(pubPath, []byte(hex.EncodeToString(pub)), 0644); err != nil {
		return nil, nil, fmt.Errorf("writing public key: %w", err)
	}
	if err := os.WriteFile(privPath, []byte(hex.EncodeToString(priv)), 0600); err != nil {
		return nil, nil, fmt.Errorf("writing private key: %w", err)
	}

	return pub, priv, nil
}

func LoadKeyPair(keysDir string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pubHex, err := os.ReadFile(filepath.Join(keysDir, "identity.pub"))
	if err != nil {
		return nil, nil, fmt.Errorf("reading public key: %w", err)
	}

	privHex, err := os.ReadFile(filepath.Join(keysDir, "identity.key"))
	if err != nil {
		return nil, nil, fmt.Errorf("reading private key: %w", err)
	}

	pub, err := hex.DecodeString(string(pubHex))
	if err != nil {
		return nil, nil, fmt.Errorf("decoding public key: %w", err)
	}

	priv, err := hex.DecodeString(string(privHex))
	if err != nil {
		return nil, nil, fmt.Errorf("decoding private key: %w", err)
	}

	return ed25519.PublicKey(pub), ed25519.PrivateKey(priv), nil
}

func Sign(priv ed25519.PrivateKey, message []byte) string {
	sig := ed25519.Sign(priv, message)
	return hex.EncodeToString(sig)
}

func Verify(pub ed25519.PublicKey, message []byte, sigHex string) bool {
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, message, sig)
}

func PublicIDFromKey(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub[:4])[:5]
}
