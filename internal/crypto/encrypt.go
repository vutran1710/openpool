package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// PackUserBin packs a user profile: [user_pubkey_32][profile_encrypted_to_operator].
// The profile is encrypted to the operator's pubkey so only the operator/relay can read it.
func PackUserBin(userPub ed25519.PublicKey, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	encrypted, err := Encrypt(operatorPub, plaintext)
	if err != nil {
		return nil, err
	}

	bin := make([]byte, 0, ed25519.PublicKeySize+len(encrypted))
	bin = append(bin, userPub...)
	bin = append(bin, encrypted...)
	return bin, nil
}

// UnpackUserBin extracts the user pubkey and decrypts the profile using the operator's private key.
func UnpackUserBin(operatorPriv ed25519.PrivateKey, bin []byte) ([]byte, error) {
	if len(bin) < ed25519.PublicKeySize {
		return nil, fmt.Errorf("bin too short")
	}
	encrypted := bin[ed25519.PublicKeySize:]
	return Decrypt(operatorPriv, encrypted)
}

// ReEncryptForRecipient decrypts a .bin with the operator key and re-encrypts the profile
// for a specific recipient's pubkey. Returns the re-encrypted profile (no pubkey prefix).
func ReEncryptForRecipient(operatorPriv ed25519.PrivateKey, bin []byte, recipientPub ed25519.PublicKey) ([]byte, error) {
	plaintext, err := UnpackUserBin(operatorPriv, bin)
	if err != nil {
		return nil, fmt.Errorf("decrypting profile: %w", err)
	}
	return Encrypt(recipientPub, plaintext)
}

func Encrypt(recipientPub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	ephPub, ephPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral key: %w", err)
	}

	recipientCurve := ed25519PubToCurve25519(recipientPub)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	encrypted := box.Seal(nil, plaintext, &nonce, &recipientCurve, ephPriv)

	result := make([]byte, 0, 32+24+len(encrypted))
	result = append(result, ephPub[:]...)
	result = append(result, nonce[:]...)
	result = append(result, encrypted...)
	return result, nil
}

func Decrypt(privKey ed25519.PrivateKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 32+24+box.Overhead {
		return nil, fmt.Errorf("ciphertext too short")
	}

	var senderPub [32]byte
	copy(senderPub[:], ciphertext[:32])

	var nonce [24]byte
	copy(nonce[:], ciphertext[32:56])

	encrypted := ciphertext[56:]
	recipientCurve := ed25519PrivToCurve25519(privKey)

	plaintext, ok := box.Open(nil, encrypted, &nonce, &senderPub, &recipientCurve)
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}

	return plaintext, nil
}

func ed25519PubToCurve25519(pub ed25519.PublicKey) [32]byte {
	h := sha256.Sum256(pub)
	return h
}

func ed25519PrivToCurve25519(priv ed25519.PrivateKey) [32]byte {
	h := sha256.Sum256(priv.Seed())
	return h
}
