// seedpool generates fake test data for a dating pool repo.
//
// Usage:
//
//	go run ./cmd/seedpool -out /path/to/dating-test-pool
//	go run ./cmd/seedpool -out /path/to/dating-test-pool -repo owner/pool-name
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vutran1710/dating-dev/internal/crypto"
)

type PoolManifest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Version        int    `json:"version"`
	CreatedAt      string `json:"created_at"`
	OperatorPubKey string `json:"operator_public_key"`
	RelayURL       string `json:"relay_url"`
}

var testUsers = []struct {
	Provider       string
	ProviderUserID string
	DisplayName    string
	Bio            string
	City           string
	Interests      []string
}{
	{
		Provider:       "github",
		ProviderUserID: "10001",
		DisplayName:    "alice",
		Bio:            "Backend engineer who loves distributed systems and coffee",
		City:           "Berlin",
		Interests:      []string{"rust", "hiking", "coffee", "open-source"},
	},
	{
		Provider:       "github",
		ProviderUserID: "10002",
		DisplayName:    "bob",
		Bio:            "Full-stack dev, weekend climber, terrible at cooking",
		City:           "Berlin",
		Interests:      []string{"typescript", "climbing", "board-games", "vim"},
	},
	{
		Provider:       "google",
		ProviderUserID: "20001",
		DisplayName:    "charlie",
		Bio:            "DevOps by day, DJ by night. Looking for someone to debug life with",
		City:           "Munich",
		Interests:      []string{"kubernetes", "vinyl", "electronic-music", "cycling"},
	},
	{
		Provider:       "github",
		ProviderUserID: "10003",
		DisplayName:    "diana",
		Bio:            "Security researcher. I break things so you don't have to",
		City:           "Hamburg",
		Interests:      []string{"ctf", "reverse-engineering", "cats", "manga"},
	},
	{
		Provider:       "google",
		ProviderUserID: "20002",
		DisplayName:    "eve",
		Bio:            "Data scientist exploring the intersection of ML and art",
		City:           "Berlin",
		Interests:      []string{"python", "generative-art", "photography", "yoga"},
	},
}

func main() {
	outDir := flag.String("out", ".", "output directory (the pool repo root)")
	poolRepo := flag.String("repo", "vutran1710/dating-test-pool", "pool repo identifier for user hash computation")
	flag.Parse()

	// Generate operator keypair
	operatorPub, operatorPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal("generating operator keys: %v", err)
	}

	// Write pool.json
	manifest := PoolManifest{
		Name:           "test-pool",
		Description:    "A test dating pool for development and testing",
		Version:        1,
		CreatedAt:      "2026-03-15T00:00:00Z",
		OperatorPubKey: hex.EncodeToString(operatorPub),
		RelayURL:       "ws://localhost:8081",
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	manifestJSON = append(manifestJSON, '\n')
	writeFile(filepath.Join(*outDir, "pool.json"), manifestJSON, 0644)
	fmt.Printf("pool.json  operator_public_key=%s\n", manifest.OperatorPubKey)

	// Save operator private key
	operatorKeyPath := filepath.Join(*outDir, "operator.key")
	writeFile(operatorKeyPath, []byte(hex.EncodeToString(operatorPriv)), 0600)
	fmt.Println("operator.key written (for OPERATOR_PRIVATE_KEY env var)")

	// Create users
	usersDir := filepath.Join(*outDir, "users")
	os.MkdirAll(usersDir, 0755)

	for _, u := range testUsers {
		// Generate user keypair
		userPub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fatal("generating user keys: %v", err)
		}

		profile := map[string]any{
			"display_name": u.DisplayName,
			"bio":          u.Bio,
			"city":         u.City,
			"interests":    u.Interests,
			"public_key":   hex.EncodeToString(userPub),
			"status":       "open",
		}
		plaintext, _ := json.Marshal(profile)

		bin, err := crypto.PackUserBin(userPub, operatorPub, plaintext)
		if err != nil {
			fatal("packing profile for %s: %v", u.DisplayName, err)
		}

		hash := crypto.UserHash(*poolRepo, u.Provider, u.ProviderUserID)
		binPath := filepath.Join(usersDir, hash+".bin")
		writeFile(binPath, bin, 0644)
		fmt.Printf("users/%s.bin  %s (%s, %s)\n", hash[:12]+"...", u.DisplayName, u.City, u.Provider)
	}

	fmt.Printf("\nDone. %d test users seeded into %s\n", len(testUsers), *outDir)
}

func writeFile(path string, data []byte, perm os.FileMode) {
	if err := os.WriteFile(path, data, perm); err != nil {
		fatal("writing %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seedpool: "+format+"\n", args...)
	os.Exit(1)
}
