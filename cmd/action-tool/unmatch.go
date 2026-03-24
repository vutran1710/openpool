package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/limits"
	"github.com/vutran1710/openpool/internal/message"
)

func cmdUnmatch() {
	issueBody := os.Getenv("ISSUE_BODY")
	issueNumberStr := os.Getenv("ISSUE_NUMBER")
	repo := os.Getenv("REPO")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")

	if len(issueBody) > limits.MaxMessageContent {
		rejectIssue(fmt.Sprintf("issue body too large: %d bytes (max %d)", len(issueBody), limits.MaxMessageContent))
	}

	issueNumber, err := strconv.Atoi(issueNumberStr)
	if err != nil {
		rejectIssue("invalid ISSUE_NUMBER")
	}

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		rejectIssue("invalid OPERATOR_PRIVATE_KEY")
	}

	// Parse issue body
	blockType, content, err := message.Parse(issueBody)
	if err != nil {
		rejectIssue("invalid issue body format: " + err.Error())
	}
	if blockType != "unmatch" {
		rejectIssue("unexpected block type: " + blockType)
	}

	// Decrypt
	blob, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		rejectIssue("invalid base64 in issue body")
	}

	plaintext, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), blob)
	if err != nil {
		rejectIssue("decryption failed")
	}

	var data struct {
		AuthorMatchHash string `json:"author_match_hash"`
		TargetMatchHash string `json:"target_match_hash"`
	}
	if err := json.Unmarshal(plaintext, &data); err != nil {
		rejectIssue("invalid unmatch payload")
	}

	if data.AuthorMatchHash == "" || data.TargetMatchHash == "" {
		rejectIssue("missing author_match_hash or target_match_hash")
	}

	// Compute pair_hash
	a, b := data.AuthorMatchHash, data.TargetMatchHash
	if a > b {
		a, b = b, a
	}
	pairHash := sha256Short(a + ":" + b)
	matchFile := "matches/" + pairHash + ".json"

	// Check match exists
	if _, err := os.Stat(matchFile); os.IsNotExist(err) {
		// Already unmatched — just close the issue
		gh, ghErr := github.NewCLI(repo)
		if ghErr == nil {
			ctx := context.Background()
			gh.CloseIssue(ctx, issueNumber, "not_planned")
			gh.LockIssue(ctx, issueNumber, "resolved")
		}
		fmt.Fprintf(os.Stderr, "match %s does not exist (already unmatched)\n", pairHash)
		os.Exit(0)
	}

	// Delete match file
	if err := os.Remove(matchFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: removing match file: %v\n", err)
		os.Exit(1)
	}

	// Commit + push
	gh, err := github.NewCLI(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating github client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	origDir, _ := os.Getwd()
	if err := gh.AddCommitPush([]string{"matches/"}, "Unmatch: "+pairHash); err != nil {
		_ = origDir
		fmt.Fprintf(os.Stderr, "error: committing unmatch: %v\n", err)
		os.Exit(1)
	}

	// Close + lock issue
	gh.CloseIssue(ctx, issueNumber, "completed")
	gh.LockIssue(ctx, issueNumber, "resolved")

	fmt.Printf("unmatched %s (pair_hash=%s)\n", data.TargetMatchHash, pairHash)
}
