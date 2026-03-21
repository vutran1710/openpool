// regcrypt computes registration hashes and encrypts them for delivery.
// Used by GitHub Actions registration workflows.
//
// Usage:
//
//	regcrypt --pool-url owner/repo --user-id 12345 --salt secret --user-pubkey hex
//
// Output (stdout):
//
//	Line 1: bin_hash (16 hex chars)
//	Line 2: base64 NaCl box encrypted {bin_hash, match_hash}
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/message"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "sign":
			cmdSign()
			return
		case "register":
			cmdRegister()
			return
		}
	}

	poolURL := flag.String("pool-url", "", "pool repo (owner/repo)")
	provider := flag.String("provider", "github", "auth provider")
	userID := flag.String("user-id", "", "provider user ID")
	salt := flag.String("salt", "", "pool salt")
	userPubHex := flag.String("user-pubkey", "", "user's ed25519 public key (hex)")
	flag.Parse()

	if *poolURL == "" || *userID == "" || *salt == "" || *userPubHex == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Compute hashes
	idH := crypto.UserHash(*poolURL, *provider, *userID).String()
	binH := sha256Short(*salt + ":" + idH)
	matchH := sha256Short(*salt + ":" + binH)

	// Decode user pubkey
	userPub, err := hex.DecodeString(*userPubHex)
	if err != nil || len(userPub) != ed25519.PublicKeySize {
		log.Fatal("invalid user pubkey: must be 64 hex chars (32 bytes)")
	}

	// Encrypt {bin_hash, match_hash} to user's pubkey (ephemeral NaCl box)
	payload, err := msgpack.Marshal(map[string]string{
		"bin_hash":   binH,
		"match_hash": matchH,
	})
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}

	encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}

	// Output: line 1 = bin_hash, line 2 = match_hash, line 3 = base64 encrypted blob
	fmt.Println(binH)
	fmt.Println(matchH)
	fmt.Println(base64.StdEncoding.EncodeToString(encrypted))
}

func cmdRegister() {
	issueBody := os.Getenv("ISSUE_BODY")
	issueAuthorID := os.Getenv("ISSUE_AUTHOR_ID")
	issueAuthor := os.Getenv("ISSUE_AUTHOR")
	repo := os.Getenv("REPO")
	poolSalt := os.Getenv("POOL_SALT")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	issueNumberStr := os.Getenv("ISSUE_NUMBER")

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
}

func cmdSign() {
	operatorKeyHex := envOrArgSign("--operator-key", "OPERATOR_PRIVATE_KEY")
	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key (expected 128 hex chars / 64 bytes)")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin: %v", err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), data)
	fmt.Print(hex.EncodeToString(sig))
}

// envOrArgSign is like envOrArg but searches from os.Args[2:] (skip "sign" subcommand)
func envOrArgSign(flag, envVar string) string {
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return ""
}

func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}
