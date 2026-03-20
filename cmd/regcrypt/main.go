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
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "sign" {
		cmdSign()
		return
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
