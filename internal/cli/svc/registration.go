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
// Returns the issue number.
func SubmitProfileToPool(ctx context.Context, poolRepo, operatorPubKeyHex, token string,
	profile *gh.DatingProfile, pub ed25519.PublicKey, priv ed25519.PrivateKey,
	title string, labels []string) (int, error) {

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

	blobHex := hex.EncodeToString(bin)

	client := gh.NewClient(poolRepo, token)
	number, err := client.CreateIssue(ctx, title, blobHex, labels)
	if err != nil {
		return 0, fmt.Errorf("creating issue: %w", err)
	}

	return number, nil
}
