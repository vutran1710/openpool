package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/limits"
	"github.com/vutran1710/openpool/internal/message"
	"github.com/vutran1710/openpool/internal/schema"
)

func cmdMatch() {
	issueBody := os.Getenv("ISSUE_BODY")
	issueNumberStr := os.Getenv("ISSUE_NUMBER")
	repo := os.Getenv("REPO")
	operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	poolSalt := os.Getenv("POOL_SALT")
	_ = poolSalt

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

	// Decrypt author's issue body
	_, issueContent, err := message.Parse(issueBody)
	if err != nil {
		rejectIssue("invalid issue body format: " + err.Error())
	}

	issueBlob, err := base64.StdEncoding.DecodeString(issueContent)
	if err != nil {
		rejectIssue("invalid base64 in issue body")
	}

	authorPlain, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), issueBlob)
	if err != nil {
		rejectIssue("decryption failed — invalid or forged issue body")
	}

	var authorData struct {
		AuthorBinHash   string `json:"author_bin_hash"`
		AuthorMatchHash string `json:"author_match_hash"`
		TargetMatchHash string `json:"target_match_hash"`
		Greeting        string `json:"greeting"`
	}
	if err := json.Unmarshal(authorPlain, &authorData); err != nil {
		rejectIssue("invalid interest payload")
	}

	// Check if author is registered (has .bin file)
	// Skip this check if issue author is the repo owner (managed account mode)
	issueAuthor := os.Getenv("ISSUE_AUTHOR")
	repoOwner := strings.Split(repo, "/")[0]
	if issueAuthor != repoOwner {
		if _, err := os.Stat("users/" + authorData.AuthorBinHash + ".bin"); os.IsNotExist(err) {
			rejectIssue("unregistered user: " + authorData.AuthorBinHash)
		}
	}

	// Load pool schema for interest_expiry
	schemaPath := envOrArg("--schema", "POOL_SCHEMA")
	if schemaPath == "" {
		schemaPath = "pool.yaml"
	}
	poolSchema, sErr := schema.Load(schemaPath)
	if sErr != nil {
		rejectIssue("loading schema: " + sErr.Error())
	}
	expiry, eErr := poolSchema.ParseInterestExpiry()
	if eErr != nil {
		rejectIssue("invalid interest_expiry: " + eErr.Error())
	}

	// Compute ephemeral hash for the author's match_hash (what a reciprocal issue targeting this author would have as title)
	ephemeralAuthor := crypto.EphemeralHash(authorData.AuthorMatchHash, expiry)

	// List open interest issues to find reciprocal
	gh, err := github.NewCLI(repo)
	if err != nil {
		writeError("creating github client: " + err.Error())
	}

	ctx := context.Background()

	recipIssues, err := gh.ListIssues(ctx, "open", "interest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listing interest issues: %v\n", err)
		os.Exit(1)
	}

	// Find reciprocal issue: title == ephemeral hash of author's match_hash
	var recipIssue *github.Issue
	for i := range recipIssues {
		if recipIssues[i].Title == ephemeralAuthor && recipIssues[i].Number != issueNumber {
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
		TargetMatchHash string `json:"target_match_hash"`
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

	// Check if match already exists — if so, just close + lock + exit
	matchFilePath := "matches/" + pairHash + ".json"
	if _, err := os.Stat(matchFilePath); err == nil {
		log.Printf("match %s already exists — closing + locking issues", pairHash)
		_ = gh.CloseIssue(ctx, issueNumber, "completed")
		_ = gh.CloseIssue(ctx, recipIssue.Number, "completed")
		gh.LockIssue(ctx, issueNumber, "resolved")
		gh.LockIssue(ctx, recipIssue.Number, "resolved")
		return
	}

	// Write matches/{pair_hash}.json (empty — existence is the signal, commit time is the timestamp)
	if err := os.MkdirAll("matches", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkdir matches: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("matches/"+pairHash+".json", []byte("{}"), 0o644); err != nil {
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

	// Lock both issues to prevent re-opening
	gh.LockIssue(ctx, issueNumber, "resolved")
	gh.LockIssue(ctx, recipIssue.Number, "resolved")
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
