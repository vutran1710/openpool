package svc

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// SubmitProfileToPool encrypts the profile and creates a registration issue
// on the pool repo. Used by both initial registration (join) and profile updates.
// Reuses RegisterUserViaIssue — same title, same labels, same Action processing.
func SubmitProfileToPool(ctx context.Context, poolRepo, operatorPubKeyHex, token string,
	profile any, userHash string,
	pub ed25519.PublicKey, priv ed25519.PrivateKey) (int, error) {

	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return 0, fmt.Errorf("serializing profile: %w", err)
	}

	operatorPub, err := hex.DecodeString(operatorPubKeyHex)
	if err != nil {
		return 0, fmt.Errorf("decoding operator key: %w", err)
	}

	bin, err := crypto.PackUserBin(pub, operatorPub, profileJSON)
	if err != nil {
		return 0, fmt.Errorf("packing profile: %w", err)
	}

	pubKeyHex := hex.EncodeToString(pub)

	// Sign the registration payload
	payload, err := json.Marshal(map[string]string{
		"action":    "register",
		"user_hash": userHash,
	})
	if err != nil {
		return 0, fmt.Errorf("marshaling payload: %w", err)
	}
	signature := crypto.Sign(priv, payload)

	// Create identity proof
	identityProof, err := crypto.EncryptIdentityProof(operatorPubKeyHex, "github", "")
	if err != nil {
		identityProof = "" // non-fatal for profile updates
	}

	pool := gh.NewPool(poolRepo, token)
	return pool.RegisterUserViaIssue(ctx, userHash, bin, pubKeyHex, signature, identityProof)
}
