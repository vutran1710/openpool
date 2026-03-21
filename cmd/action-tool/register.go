package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/limits"
	"github.com/vutran1710/dating-dev/internal/message"
)

func cmdRegister() {
	issueBody := os.Getenv("ISSUE_BODY")
	issueAuthorID := os.Getenv("ISSUE_AUTHOR_ID")
	issueAuthor := os.Getenv("ISSUE_AUTHOR")
	repo := os.Getenv("REPO")
	poolSalt := os.Getenv("POOL_SALT")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	issueNumberStr := os.Getenv("ISSUE_NUMBER")

	if len(issueBody) > limits.MaxMessageContent {
		fmt.Fprintf(os.Stderr, "error: issue body too large: %d bytes (max %d)\n", len(issueBody), limits.MaxMessageContent)
		os.Exit(1)
	}

	issueNumber, err := strconv.Atoi(issueNumberStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid ISSUE_NUMBER: %v\n", err)
		os.Exit(1)
	}

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "error: invalid OPERATOR_PRIVATE_KEY\n")
		os.Exit(1)
	}

	// Parse issue body
	blockType, content, err := message.Parse(issueBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing issue body: %v\n", err)
		os.Exit(1)
	}
	if blockType != "registration-request" {
		fmt.Fprintf(os.Stderr, "error: unexpected block type: %s\n", blockType)
		os.Exit(1)
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 5 {
		fmt.Fprintf(os.Stderr, "error: registration request must have 5 lines\n")
		os.Exit(1)
	}

	userHash := lines[0]
	pubkeyHex := lines[1]
	blobHex := lines[2]
	signature := lines[3]
	_ = lines[4] // identity_proof

	_ = userHash
	_ = blobHex
	_ = signature

	// Decode user pubkey
	userPub, err := hex.DecodeString(pubkeyHex)
	if err != nil || len(userPub) != ed25519.PublicKeySize {
		fmt.Fprintf(os.Stderr, "error: invalid user pubkey\n")
		os.Exit(1)
	}

	// Determine provider/user_id
	// Operator uses custom ID from issue body; regular users use github + author ID
	provider := "github"
	userID := issueAuthorID
	if issueAuthor == "github-actions[bot]" {
		provider = "custom"
		userID = userHash
	}

	// Compute hashes
	idH := crypto.UserHash(repo, provider, userID).String()
	binHash := sha256Short(poolSalt + ":" + idH)
	matchHash := sha256Short(poolSalt + ":" + binHash)

	// Encrypt {bin_hash, match_hash} to user's pubkey
	payload, err := msgpack.Marshal(map[string]string{
		"bin_hash":   binHash,
		"match_hash": matchHash,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
		os.Exit(1)
	}

	encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encrypt: %v\n", err)
		os.Exit(1)
	}

	// Sign ciphertext with operator key
	sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), encrypted)
	signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

	// Write users/{bin_hash}.bin
	binData := make([]byte, 0, len(userPub)+len(encrypted))
	binData = append(binData, userPub...)
	binData = append(binData, encrypted...)
	if len(binData) > limits.MaxBinFile {
		fmt.Fprintf(os.Stderr, "error: .bin file too large: %d bytes (max %d)\n", len(binData), limits.MaxBinFile)
		os.Exit(1)
	}
	if err := os.MkdirAll("users", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkdir users: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("users/"+binHash+".bin", binData, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing bin file: %v\n", err)
		os.Exit(1)
	}

	// Git + GitHub operations
	gh, err := github.NewCLI(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating github client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if err := gh.AddCommitPush([]string{"users/"}, "Register user "+binHash); err != nil {
		fmt.Fprintf(os.Stderr, "error: add/commit/push: %v\n", err)
		os.Exit(1)
	}

	if err := gh.CloseIssue(ctx, issueNumber, "completed"); err != nil {
		fmt.Fprintf(os.Stderr, "error: closing issue: %v\n", err)
		os.Exit(1)
	}

	if err := gh.CommentIssue(ctx, issueNumber, message.Format("registration", signedBlob)); err != nil {
		fmt.Fprintf(os.Stderr, "error: commenting on issue: %v\n", err)
		os.Exit(1)
	}

	// Lock issue to prevent re-opening
	gh.LockIssue(ctx, issueNumber, "resolved")
}
