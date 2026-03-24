package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	gh "github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/bucket"
	"github.com/vutran1710/openpool/internal/indexer"
	"github.com/vutran1710/openpool/internal/schema"
)

func cmdIndex() {
	fs := flag.NewFlagSet("index2", flag.ExitOnError)
	schemaPath := fs.String("schema", "pool.yaml", "path to pool.yaml")
	operatorKeyHex := fs.String("operator-key", "", "operator ed25519 private key (hex)")
	output := fs.String("output", "index.db", "output file path for index.db")
	upload := fs.Bool("upload", false, "upload index.db as a GitHub release asset")
	usersDir := fs.String("users-dir", "users", "path to users/ directory")
	salt := fs.String("salt", "", "pool salt (or POOL_SALT env var)")
	fs.Parse(os.Args[2:])

	// Resolve env vars
	if *operatorKeyHex == "" {
		*operatorKeyHex = os.Getenv("OPERATOR_PRIVATE_KEY")
	}
	if *operatorKeyHex == "" {
		log.Fatal("operator key required (--operator-key or OPERATOR_PRIVATE_KEY env)")
	}

	poolSalt := *salt
	if poolSalt == "" {
		poolSalt = os.Getenv("POOL_SALT")
	}
	if poolSalt == "" {
		log.Fatal("pool salt required (--salt or POOL_SALT env)")
	}

	opKey, err := hex.DecodeString(*operatorKeyHex)
	if err != nil || len(opKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key: must be 128 hex chars (64 bytes)")
	}

	// Load schema for partition config
	s, err := schema.Load(*schemaPath)
	if err != nil {
		log.Fatalf("loading schema: %v", err)
	}

	// Convert schema partitions to bucket partitions
	partitions, fieldRanges := schemaToPartitions(s)

	// Determine permutations and nonce_space from schema
	permutations := 5 // default
	nonceSpace := 20  // default
	if s.Indexing != nil {
		if s.Indexing.Permutations > 0 {
			permutations = s.Indexing.Permutations
		}
		if s.Indexing.Difficulty > 0 {
			nonceSpace = s.Indexing.Difficulty
		}
	}

	fmt.Printf("Building chain-encrypted index...\n")
	fmt.Printf("  schema:       %s\n", *schemaPath)
	fmt.Printf("  users:        %s\n", *usersDir)
	fmt.Printf("  partitions:   %d\n", len(partitions))
	fmt.Printf("  permutations: %d\n", permutations)
	fmt.Printf("  difficulty:   %d (nonce_space)\n", nonceSpace)

	err = indexer.Build(indexer.Config{
		UsersDir:     *usersDir,
		OutputPath:   *output,
		OperatorKey:  ed25519.PrivateKey(opKey),
		Partitions:   partitions,
		FieldRanges:  fieldRanges,
		Permutations: permutations,
		NonceSpace:   nonceSpace,
		Salt:         poolSalt,
	})
	if err != nil {
		log.Fatalf("building index: %v", err)
	}

	stats, _ := indexer.Stats(*output)
	fmt.Printf("  buckets:      %d\n", stats.Buckets)
	fmt.Printf("  entries:      %d\n", stats.Entries)
	fmt.Printf("  output:       %s\n", *output)

	if *upload {
		repo := os.Getenv("REPO")
		if repo == "" {
			writeError("REPO env var required for --upload")
		}
		ghCli, err := gh.NewCLI(repo)
		if err != nil {
			writeError("github CLI: " + err.Error())
		}
		if err := ghCli.UploadReleaseAsset(context.Background(), "index-latest", "index.db", *output); err != nil {
			writeError("uploading index: " + err.Error())
		}
		fmt.Printf("  uploaded as release asset\n")
	}
}

// schemaToPartitions converts schema indexing config to bucket partition config.
func schemaToPartitions(s *schema.PoolSchema) ([]bucket.PartitionConfig, map[string][2]int) {
	fieldRanges := make(map[string][2]int)

	if s.Indexing == nil || len(s.Indexing.Partitions) == 0 {
		// Default: partition by role if roles exist
		roles, _ := s.ParsedRoles()
		if len(roles) > 0 {
			return []bucket.PartitionConfig{{Field: "role"}}, fieldRanges
		}
		return nil, fieldRanges
	}

	partitions := make([]bucket.PartitionConfig, len(s.Indexing.Partitions))
	for i, p := range s.Indexing.Partitions {
		partitions[i] = bucket.PartitionConfig{
			Field:   p.Field,
			Step:    p.Step,
			Overlap: p.Overlap,
		}

		// Extract field ranges for range partitions
		if p.Step > 0 {
			if attr, ok := s.Profile[p.Field]; ok {
				min, max := 0, 100
				if attr.Min != nil {
					min = *attr.Min
				}
				if attr.Max != nil {
					max = *attr.Max
				}
				fieldRanges[p.Field] = [2]int{min, max}
			}
		}
	}

	return partitions, fieldRanges
}
