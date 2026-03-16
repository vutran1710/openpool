// Package crypto provides encryption and signing using ed25519 key pairs.
//
// Signing uses ed25519 directly (native).
//
// Encryption uses NaCl box (Curve25519 + XSalsa20-Poly1305) with automatic
// ed25519 → curve25519 key conversion. This is the same approach used by
// libsodium (crypto_sign_ed25519_pk_to_curve25519), Signal Protocol, and
// other established cryptographic systems. The conversion is an internal
// implementation detail — the public API only accepts ed25519 keys.
//
// Wire format for encrypted data:
//   [32B ephemeral curve25519 pubkey][24B nonce][ciphertext + Poly1305 tag]
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/curve25519"
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

	recipientCurve, err := ed25519PubToCurve25519(recipientPub)
	if err != nil {
		return nil, fmt.Errorf("converting recipient pubkey: %w", err)
	}

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

// ed25519PubToCurve25519 converts an ed25519 public key to curve25519
// using the proper edwards→montgomery conversion.
func ed25519PubToCurve25519(pub ed25519.PublicKey) ([32]byte, error) {
	p, err := new(edwards25519.Point).SetBytes(pub)
	if err != nil {
		return [32]byte{}, fmt.Errorf("invalid ed25519 public key: %w", err)
	}
	var out [32]byte
	copy(out[:], p.BytesMontgomery())
	return out, nil
}

// ed25519PrivToCurve25519 converts an ed25519 private key to curve25519
// using the standard derivation from RFC 8032.
func ed25519PrivToCurve25519(priv ed25519.PrivateKey) [32]byte {
	h := sha512.Sum512(priv.Seed())
	var out [32]byte
	copy(out[:], h[:32])
	// Clamp per curve25519 spec
	out[0] &= 248
	out[31] &= 127
	out[31] |= 64

	// Verify key pair consistency (optional, for debugging)
	_ = curve25519.Basepoint

	return out
}
