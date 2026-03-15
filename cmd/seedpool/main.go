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
	gh "github.com/vutran1710/dating-dev/internal/github"
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
	registryOut := flag.String("registry-out", "", "registry pool directory (writes pool.json + encrypted tokens.bin)")
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
	keysDir := filepath.Join(*outDir, "keys")
	os.MkdirAll(usersDir, 0755)
	os.MkdirAll(keysDir, 0700)

	type seedUser struct {
		Hash        string   `json:"hash"`
		DisplayName string   `json:"display_name"`
		City        string   `json:"city"`
		Provider    string   `json:"provider"`
		ProviderUID string   `json:"provider_user_id"`
		Interests   []string `json:"interests"`
		PubKey      string   `json:"public_key"`
		PrivKey     string   `json:"private_key"`
	}

	var seedUsers []seedUser

	for _, u := range testUsers {
		// Generate user keypair
		userPub, userPriv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fatal("generating user keys: %v", err)
		}

		pubHex := hex.EncodeToString(userPub)
		privHex := hex.EncodeToString(userPriv)

		profile := map[string]any{
			"display_name": u.DisplayName,
			"bio":          u.Bio,
			"city":         u.City,
			"interests":    u.Interests,
			"public_key":   pubHex,
			"status":       "open",
		}
		plaintext, _ := json.Marshal(profile)

		bin, err := crypto.PackUserBin(userPub, operatorPub, plaintext)
		if err != nil {
			fatal("packing profile for %s: %v", u.DisplayName, err)
		}

		hash := crypto.UserHash(*poolRepo, u.Provider, u.ProviderUserID)

		// Write .bin
		writeFile(filepath.Join(usersDir, hash+".bin"), bin, 0644)

		// Write keypair
		userKeysDir := filepath.Join(keysDir, hash)
		os.MkdirAll(userKeysDir, 0700)
		writeFile(filepath.Join(userKeysDir, "identity.pub"), []byte(pubHex), 0644)
		writeFile(filepath.Join(userKeysDir, "identity.key"), []byte(privHex), 0600)

		seedUsers = append(seedUsers, seedUser{
			Hash:        hash,
			DisplayName: u.DisplayName,
			City:        u.City,
			Provider:    u.Provider,
			ProviderUID: u.ProviderUserID,
			Interests:   u.Interests,
			PubKey:      pubHex,
			PrivKey:     privHex,
		})

		fmt.Printf("users/%s.bin  keys/%s/  %s (%s, %s)\n", hash, hash, u.DisplayName, u.City, u.Provider)
	}

	// Write seed.json manifest
	seedManifest := map[string]any{
		"pool_repo":            *poolRepo,
		"operator_public_key":  hex.EncodeToString(operatorPub),
		"operator_private_key": hex.EncodeToString(operatorPriv),
		"users":                seedUsers,
	}
	seedJSON, _ := json.MarshalIndent(seedManifest, "", "  ")
	seedJSON = append(seedJSON, '\n')
	writeFile(filepath.Join(*outDir, "seed.json"), seedJSON, 0600)

	// Optionally write registry entry
	if *registryOut != "" {
		os.MkdirAll(*registryOut, 0755)

		regEntry := gh.PoolEntry{
			Name:           "test-pool",
			Repo:           *poolRepo,
			Description:    "A test dating pool for development and testing",
			OperatorPubKey: hex.EncodeToString(operatorPub),
			RelayURL:       "ws://localhost:8081",
			CreatedAt:      "2026-03-15T00:00:00Z",
		}
		regJSON, _ := json.MarshalIndent(regEntry, "", "  ")
		regJSON = append(regJSON, '\n')
		writeFile(filepath.Join(*registryOut, "pool.json"), regJSON, 0644)

		tokens := gh.PoolTokens{GHToken: "test_token_placeholder"}
		tokensBin, err := gh.SerializeTokens(tokens, hex.EncodeToString(operatorPub))
		if err != nil {
			fatal("encrypting tokens: %v", err)
		}
		writeFile(filepath.Join(*registryOut, "tokens.bin"), tokensBin, 0644)

		fmt.Printf("registry entry written to %s\n", *registryOut)
	}

	fmt.Printf("\nDone. %d test users seeded into %s\n", len(testUsers), *outDir)
	fmt.Println("seed.json written (all keys + user mapping)")
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
