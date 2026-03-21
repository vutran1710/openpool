package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
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
