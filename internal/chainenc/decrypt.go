package chainenc

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
)

// SolveCold solves an entry when the profile_constant is unknown.
// Brute-forces: constSpace × nonceSpace × 6 orderings.
func SolveCold(entry Entry, hint int, constSpace, nonceSpace int) (*Payload, int, error) {
	attempts := 0

	for constant := 0; constant < constSpace; constant++ {
		for nonce := 0; nonce < nonceSpace; nonce++ {
			for _, ordering := range Orderings {
				attempts++
				key := DeriveKey(hint, constant, nonce, ordering)
				payload, err := tryDecrypt(entry, key)
				if err == nil {
					return payload, attempts, nil
				}
			}
		}
	}

	return nil, attempts, fmt.Errorf("cold solve failed after %d attempts", attempts)
}

// SolveWarm solves an entry when the profile_constant is known.
// Brute-forces: nonceSpace × 6 orderings.
func SolveWarm(entry Entry, hint int, knownConstant int, nonceSpace int) (*Payload, int, error) {
	attempts := 0

	for nonce := 0; nonce < nonceSpace; nonce++ {
		for _, ordering := range Orderings {
			attempts++
			key := DeriveKey(hint, knownConstant, nonce, ordering)
			payload, err := tryDecrypt(entry, key)
			if err == nil {
				return payload, attempts, nil
			}
		}
	}

	return nil, attempts, fmt.Errorf("warm solve failed after %d attempts", attempts)
}

func tryDecrypt(entry Entry, key [32]byte) (*Payload, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, entry.Nonce, entry.Ciphertext, nil)
	if err != nil {
		return nil, err // wrong key — GCM auth tag mismatch
	}

	var payload Payload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
