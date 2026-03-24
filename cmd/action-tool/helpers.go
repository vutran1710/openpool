package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/vutran1710/openpool/internal/github"
)

// sha256Short computes SHA-256 and returns the first 16 hex characters.
func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}

// envOrArg looks for --flag in os.Args[2:], falls back to env var.
func envOrArg(flag, envVar string) string {
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

// writeError prints an error message to stderr and exits with code 1.
func writeError(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}

// rejectIssue closes + locks an issue and exits with error.
// Used when an issue is invalid (bad format, unregistered user, etc.)
func rejectIssue(msg string) {
	repo := os.Getenv("REPO")
	issueNumStr := os.Getenv("ISSUE_NUMBER")
	issueNum, _ := strconv.Atoi(issueNumStr)

	if repo != "" && issueNum > 0 {
		gh, err := github.NewCLI(repo)
		if err == nil {
			ctx := context.Background()
			gh.CloseIssue(ctx, issueNum, "not_planned")
			gh.LockIssue(ctx, issueNum, "spam")
		}
	}

	fmt.Fprintf(os.Stderr, "rejected: %s\n", msg)
	os.Exit(1)
}
