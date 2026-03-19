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
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/vutran1710/dating-dev/internal/crypto"
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
