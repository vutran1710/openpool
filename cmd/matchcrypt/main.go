// matchcrypt handles crypto operations for the mutual match GitHub Action.
//
// Decrypt interest PR body:
//   matchcrypt decrypt --operator-key <hex> --body <base64>
//   -> stdout: JSON {author_bin_hash, author_match_hash, greeting}
//
// Encrypt match notification for a user:
//   matchcrypt encrypt --user-pubkey <hex> --match-hash <hash> --peer-pubkey <hex> --greeting "message"
//   -> stdout: base64 encrypted blob containing {matched_match_hash, greeting, pubkey}
//
// Read pubkey from .bin file:
//   matchcrypt pubkey --operator-key <hex> --bin-file <path>
//   -> stdout: hex pubkey
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/limits"
	"github.com/vutran1710/dating-dev/internal/message"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: matchcrypt <decrypt|encrypt|pubkey> [flags]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "decrypt":
		cmdDecrypt()
	case "encrypt":
		cmdEncrypt()
	case "pubkey":
		cmdPubkey()
	case "sign":
		cmdSign()
	case "match":
		cmdMatch()
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func cmdDecrypt() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
	body := envOrArg("--body", "")

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key")
	}

	blobBytes, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		log.Fatalf("base64 decode: %v", err)
	}

	plaintext, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), blobBytes)
	if err != nil {
		log.Fatalf("decrypt: %v", err)
	}

	fmt.Print(string(plaintext))
}

func cmdEncrypt() {
	userPubHex := envOrArg("--user-pubkey", "")
	matchHash := envOrArg("--match-hash", "")
	peerPubHex := envOrArg("--peer-pubkey", "")
	greeting := envOrArg("--greeting", "")

	userPub, err := hex.DecodeString(userPubHex)
	if err != nil || len(userPub) != ed25519.PublicKeySize {
		log.Fatal("invalid user pubkey")
	}

	payload, _ := json.Marshal(map[string]string{
		"matched_match_hash": matchHash,
		"greeting":           greeting,
		"pubkey":             peerPubHex,
	})

	encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}

	fmt.Print(base64.StdEncoding.EncodeToString(encrypted))
}

func cmdPubkey() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
	binFile := envOrArg("--bin-file", "")

	operatorKey, err := hex.DecodeString(operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key")
	}

	binData, err := os.ReadFile(binFile)
	if err != nil {
		log.Fatalf("reading bin file: %v", err)
	}

	// First 32 bytes of .bin file = user's ed25519 pubkey
	if len(binData) < 32 {
		log.Fatal("bin file too short")
	}
	pubkey := binData[:32]

	fmt.Print(hex.EncodeToString(pubkey))
}

func cmdSign() {
	operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
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

func cmdMatch() {
	issueBody := os.Getenv("ISSUE_BODY")
	issueNumberStr := os.Getenv("ISSUE_NUMBER")
	repo := os.Getenv("REPO")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	poolSalt := os.Getenv("POOL_SALT")
	_ = poolSalt

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

	// Decrypt author's issue body
	_, issueContent, err := message.Parse(issueBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing issue body: %v\n", err)
		os.Exit(1)
	}

	issueBlob, err := base64.StdEncoding.DecodeString(issueContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: base64 decode issue body: %v\n", err)
		os.Exit(1)
	}

	authorPlain, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), issueBlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: decrypting issue body: %v\n", err)
		os.Exit(1)
	}

	var authorData struct {
		AuthorBinHash   string `json:"author_bin_hash"`
		AuthorMatchHash string `json:"author_match_hash"`
		Greeting        string `json:"greeting"`
	}
	if err := json.Unmarshal(authorPlain, &authorData); err != nil {
		fmt.Fprintf(os.Stderr, "error: unmarshal author data: %v\n", err)
		os.Exit(1)
	}

	// List open interest issues to find reciprocal
	gh, err := github.NewCLI(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating github client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	recipIssues, err := gh.ListIssues(ctx, "open", "interest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listing interest issues: %v\n", err)
		os.Exit(1)
	}

	// Find reciprocal issue: title == author_match_hash (someone targeting this author)
	var recipIssue *github.Issue
	for i := range recipIssues {
		if recipIssues[i].Title == authorData.AuthorMatchHash && recipIssues[i].Number != issueNumber {
			recipIssue = &recipIssues[i]
			break
		}
	}

	if recipIssue == nil {
		// No match — exit silently
		os.Exit(0)
	}

	// Decrypt reciprocal issue body
	_, recipContent, err := message.Parse(recipIssue.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing reciprocal issue body: %v\n", err)
		os.Exit(1)
	}

	recipBlob, err := base64.StdEncoding.DecodeString(recipContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: base64 decode reciprocal issue body: %v\n", err)
		os.Exit(1)
	}

	recipPlain, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), recipBlob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: decrypting reciprocal issue body: %v\n", err)
		os.Exit(1)
	}

	var recipData struct {
		AuthorBinHash   string `json:"author_bin_hash"`
		AuthorMatchHash string `json:"author_match_hash"`
		Greeting        string `json:"greeting"`
	}
	if err := json.Unmarshal(recipPlain, &recipData); err != nil {
		fmt.Fprintf(os.Stderr, "error: unmarshal reciprocal data: %v\n", err)
		os.Exit(1)
	}

	// Read pubkeys from users/{bin_hash}.bin (first 32 bytes)
	authorBinData, err := os.ReadFile("users/" + authorData.AuthorBinHash + ".bin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading author bin file: %v\n", err)
		os.Exit(1)
	}
	if len(authorBinData) < 32 {
		fmt.Fprintf(os.Stderr, "error: author bin file too short\n")
		os.Exit(1)
	}
	authorPub := authorBinData[:32]

	recipBinData, err := os.ReadFile("users/" + recipData.AuthorBinHash + ".bin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading reciprocal bin file: %v\n", err)
		os.Exit(1)
	}
	if len(recipBinData) < 32 {
		fmt.Fprintf(os.Stderr, "error: reciprocal bin file too short\n")
		os.Exit(1)
	}
	recipPub := recipBinData[:32]

	// Compute pair_hash = sha256(min(a,b) + ":" + max(a,b))[:12]
	a, b := authorData.AuthorMatchHash, recipData.AuthorMatchHash
	if a > b {
		a, b = b, a
	}
	pairHash := sha256Short(a + ":" + b)

	// Write matches/{pair_hash}.json
	matchData, _ := json.Marshal(map[string]string{
		"pair_hash":  pairHash,
		"match_a":    authorData.AuthorMatchHash,
		"match_b":    recipData.AuthorMatchHash,
		"matched_at": time.Now().UTC().Format(time.RFC3339),
	})
	if err := os.MkdirAll("matches", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkdir matches: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("matches/"+pairHash+".json", matchData, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing match file: %v\n", err)
		os.Exit(1)
	}

	// Encrypt + sign notifications for both users
	authorNotif := encryptAndSign(authorPub, recipData.AuthorMatchHash, hex.EncodeToString(recipPub), recipData.Greeting, operatorKey)
	recipNotif := encryptAndSign(recipPub, authorData.AuthorMatchHash, hex.EncodeToString(authorPub), authorData.Greeting, operatorKey)

	// AddCommitPush — ignore error if file exists from concurrent Action
	_ = gh.AddCommitPush([]string{"matches/"}, "Match: "+pairHash)

	// Close issues — ignore already-closed errors
	_ = gh.CloseIssue(ctx, issueNumber, "completed")
	_ = gh.CloseIssue(ctx, recipIssue.Number, "completed")

	// Comment on issues with match notifications
	if err := gh.CommentIssue(ctx, issueNumber, message.Format("match", authorNotif)); err != nil {
		fmt.Fprintf(os.Stderr, "error: commenting on author issue: %v\n", err)
		os.Exit(1)
	}

	if err := gh.CommentIssue(ctx, recipIssue.Number, message.Format("match", recipNotif)); err != nil {
		fmt.Fprintf(os.Stderr, "error: commenting on reciprocal issue: %v\n", err)
		os.Exit(1)
	}
}

func encryptAndSign(recipientPub []byte, peerMatchHash, peerPubHex, greeting string, operatorKey []byte) string {
	payload, _ := json.Marshal(map[string]string{
		"matched_match_hash": peerMatchHash,
		"greeting":           greeting,
		"pubkey":             peerPubHex,
	})
	encrypted, _ := crypto.Encrypt(ed25519.PublicKey(recipientPub), payload)
	sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), encrypted)
	return base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)
}

func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}

// envOrArg looks for --flag in os.Args, falls back to env var.
func envOrArg(flag, envVar string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
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
