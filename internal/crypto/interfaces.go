package crypto

import "crypto/ed25519"

// CryptoOps defines the interface for cryptographic operations.
// Implementations: package-level functions (real), MockCrypto (test).
type CryptoOps interface {
	GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	Encrypt(recipientPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Decrypt(privKey ed25519.PrivateKey, ciphertext []byte) ([]byte, error)
	PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	UnpackUserBin(operatorPriv ed25519.PrivateKey, bin []byte) ([]byte, error)
}
