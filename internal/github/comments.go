package github

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/vutran1710/dating-dev/internal/message"
)

// FindOperatorReplyInIssue scans issue comments newest-first for the first one
// matching the block type with a valid operator signature.
// Returns the raw signed content (base64.hexsig) — caller handles decryption.
func FindOperatorReplyInIssue(
	ctx context.Context,
	client GitHubClient,
	issueNumber int,
	blockType string,
	operatorPub ed25519.PublicKey,
) (string, error) {
	comments, err := client.ListIssueComments(ctx, issueNumber)
	if err != nil {
		return "", fmt.Errorf("listing comments: %w", err)
	}
	for i := len(comments) - 1; i >= 0; i-- {
		bt, content, parseErr := message.Parse(comments[i].Body)
		if parseErr != nil || bt != blockType {
			continue
		}
		if verifyBlobSignature(content, operatorPub) {
			return content, nil
		}
	}
	return "", fmt.Errorf("no valid %s reply found", blockType)
}

// verifyBlobSignature checks if a "base64.hexsig" content has a valid operator signature.
func verifyBlobSignature(content string, operatorPub ed25519.PublicKey) bool {
	parts := strings.SplitN(content, ".", 2)
	if len(parts) != 2 {
		return false
	}
	ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sigBytes, err := hex.DecodeString(parts[1])
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(operatorPub, ciphertext, sigBytes)
}
