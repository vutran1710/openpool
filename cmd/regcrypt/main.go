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
	"log"
	"os"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func main() {
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

func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}
