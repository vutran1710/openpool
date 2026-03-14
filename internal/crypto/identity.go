package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type IdentityProof struct {
	Provider       string `json:"provider"`
	ProviderUserID string `json:"provider_user_id"`
}

func EncryptIdentityProof(operatorPubKeyHex, provider, providerUserID string) (string, error) {
	pubBytes, err := hex.DecodeString(operatorPubKeyHex)
	if err != nil {
		return "", fmt.Errorf("decoding operator public key: %w", err)
	}

	if len(pubBytes) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid operator public key length")
	}

	proof := IdentityProof{
		Provider:       provider,
		ProviderUserID: providerUserID,
	}

	plaintext, _ := json.Marshal(proof)
	encrypted, err := Encrypt(ed25519.PublicKey(pubBytes), plaintext)
	if err != nil {
		return "", fmt.Errorf("encrypting identity proof: %w", err)
	}

	return hex.EncodeToString(encrypted), nil
}

func DecryptIdentityProof(privKey ed25519.PrivateKey, encryptedHex string) (*IdentityProof, error) {
	encrypted, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return nil, fmt.Errorf("decoding encrypted proof: %w", err)
	}

	plaintext, err := Decrypt(privKey, encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypting identity proof: %w", err)
	}

	var proof IdentityProof
	if err := json.Unmarshal(plaintext, &proof); err != nil {
		return nil, fmt.Errorf("parsing identity proof: %w", err)
	}

	return &proof, nil
}
