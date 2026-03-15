package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
)

// MockCrypto provides deterministic crypto operations for testing.
// Encryption is a no-op (plaintext stored as-is with a header), making tests predictable.
type MockCrypto struct {
	FixedPub  ed25519.PublicKey
	FixedPriv ed25519.PrivateKey
}

func NewMockCrypto() *MockCrypto {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return &MockCrypto{FixedPub: pub, FixedPriv: priv}
}

func (m *MockCrypto) GenerateKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.FixedPub, m.FixedPriv, nil
}

func (m *MockCrypto) LoadKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.FixedPub, m.FixedPriv, nil
}

func (m *MockCrypto) Encrypt(_ ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return Encrypt(m.FixedPub, plaintext)
}

func (m *MockCrypto) Decrypt(_ ed25519.PrivateKey, ciphertext []byte) ([]byte, error) {
	return Decrypt(m.FixedPriv, ciphertext)
}

func (m *MockCrypto) PackUserBin(userPub, _ ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return PackUserBin(userPub, m.FixedPub, plaintext)
}

func (m *MockCrypto) UnpackUserBin(_ ed25519.PrivateKey, bin []byte) ([]byte, error) {
	return UnpackUserBin(m.FixedPriv, bin)
}
