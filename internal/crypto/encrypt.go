package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

func UserHash(pubKeyHex string) string {
	h := sha256.Sum256([]byte(pubKeyHex))
	return hex.EncodeToString(h[:])
}

func Encrypt(recipientPub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	var pubArray, privArray [32]byte
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral key: %w", err)
	}
	copy(pubArray[:], pub[:])
	copy(privArray[:], priv[:])

	recipientCurve := ed25519PubToCurve25519(recipientPub)

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	encrypted := box.Seal(nil, plaintext, &nonce, &recipientCurve, &privArray)

	result := make([]byte, 0, 32+24+len(encrypted))
	result = append(result, pubArray[:]...)
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
