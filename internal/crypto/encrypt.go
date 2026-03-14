package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

func PackUserBin(pub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	encrypted, err := Encrypt(pub, plaintext)
	if err != nil {
		return nil, err
	}

	bin := make([]byte, 0, ed25519.PublicKeySize+len(encrypted))
	bin = append(bin, pub...)
	bin = append(bin, encrypted...)
	return bin, nil
}

func UnpackUserBin(priv ed25519.PrivateKey, bin []byte) ([]byte, error) {
	if len(bin) < ed25519.PublicKeySize {
		return nil, fmt.Errorf("bin too short")
	}
	encrypted := bin[ed25519.PublicKeySize:]
	return Decrypt(priv, encrypted)
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
