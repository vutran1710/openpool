package chainenc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
)

// BuildChain encrypts a list of profiles into a chain.
// nonceSpace is the difficulty knob (number of possible nonce values).
func BuildChain(profiles []ProfileEntry, seed []byte, nonceSpace int) (*Chain, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("empty profile list")
	}

	// Pre-compute constants and hints
	constants := make([]int, len(profiles))
	hints := make([]int, len(profiles))
	for i, p := range profiles {
		constants[i] = ProfileConstant(p.Data)
		hints[i] = ProfileHint(p.Data)
	}

	seedHintVal := SeedHint(seed)
	currentHint := seedHintVal
	entries := make([]Entry, len(profiles))

	for i, p := range profiles {
		ctx := ChainContext(seed, i)
		ordering := PickOrdering(ctx)

		nonceVal, err := randInt(nonceSpace)
		if err != nil {
			return nil, fmt.Errorf("generating nonce: %w", err)
		}

		// Next hint: next profile's hint, or seed hint for last entry
		nextHint := seedHintVal
		if i+1 < len(profiles) {
			nextHint = hints[i+1]
		}

		// Build payload
		payload := Payload{
			Profile:    make(map[string]any),
			MyConstant: constants[i],
			NextHint:   nextHint,
		}
		json.Unmarshal(p.Data, &payload.Profile)

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshaling payload: %w", err)
		}

		// Derive AES key and encrypt
		key := DeriveKey(currentHint, constants[i], nonceVal, ordering)
		gcmNonce := make([]byte, 12)
		if _, err := rand.Read(gcmNonce); err != nil {
			return nil, fmt.Errorf("generating GCM nonce: %w", err)
		}

		block, err := aes.NewCipher(key[:])
		if err != nil {
			return nil, fmt.Errorf("creating cipher: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("creating GCM: %w", err)
		}

		ciphertext := gcm.Seal(nil, gcmNonce, payloadBytes, nil)

		entries[i] = Entry{
			Tag:        p.Tag,
			Ciphertext: ciphertext,
			Nonce:      gcmNonce,
			Context:    ctx,
		}

		currentHint = nextHint
	}

	return &Chain{
		Seed:        seed,
		SeedHintVal: seedHintVal,
		NonceSpace:  nonceSpace,
		Entries:     entries,
	}, nil
}

func randInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}
