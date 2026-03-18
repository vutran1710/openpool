// indexer processes encrypted .bin profiles into filter values + weighted vectors for discovery.
// Used by GitHub Actions registration workflows.
//
// Single-user mode:
//
//	indexer --pool-json pool.json --bin-file users/abc.bin --match-hash abc123 --operator-key hex
//
// Rebuild mode:
//
//	indexer --pool-json pool.json --rebuild --users-dir users/ --operator-key hex
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func main() {
	poolJSON := flag.String("pool-json", "pool.json", "path to pool.json with schema")
	weightsStr := flag.String("weights", "", "JSON weights (or INDEXER_WEIGHTS env var)")
	operatorKeyHex := flag.String("operator-key", "", "operator ed25519 private key (hex)")
	binFile := flag.String("bin-file", "", "single .bin file to index")
	matchHash := flag.String("match-hash", "", "match_hash for output filename")
	outputDir := flag.String("output-dir", "index", "directory for .rec files")
	rebuild := flag.Bool("rebuild", false, "re-index all .bin files")
	usersDir := flag.String("users-dir", "users", "path to users/ directory (for --rebuild)")
	flag.Parse()

	// Resolve weights
	weights := *weightsStr
	if weights == "" {
		weights = os.Getenv("INDEXER_WEIGHTS")
	}
	var weightMap map[string]float64
	if weights != "" {
		if err := json.Unmarshal([]byte(weights), &weightMap); err != nil {
			log.Fatalf("parsing weights: %v", err)
		}
	} else {
		weightMap = make(map[string]float64)
	}

	// Read pool.json
	poolData, err := os.ReadFile(*poolJSON)
	if err != nil {
		log.Fatalf("reading pool.json: %v", err)
	}
	var manifest gh.PoolManifest
	if err := json.Unmarshal(poolData, &manifest); err != nil {
		log.Fatalf("parsing pool.json: %v", err)
	}
	if manifest.Schema == nil {
		log.Fatal("pool.json has no schema")
	}

	// Parse operator key
	if *operatorKeyHex == "" {
		*operatorKeyHex = os.Getenv("OPERATOR_PRIVATE_KEY")
	}
	if *operatorKeyHex == "" {
		log.Fatal("operator key required (--operator-key or OPERATOR_PRIVATE_KEY env)")
	}
	operatorKey, err := hex.DecodeString(*operatorKeyHex)
	if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
		log.Fatal("invalid operator key: must be 128 hex chars (64 bytes)")
	}

	os.MkdirAll(*outputDir, 0755)

	if *rebuild {
		rebuildAll(manifest.Schema, weightMap, ed25519.PrivateKey(operatorKey), *usersDir, *outputDir)
	} else {
		if *binFile == "" || *matchHash == "" {
			log.Fatal("single-user mode requires --bin-file and --match-hash")
		}
		indexOne(manifest.Schema, weightMap, ed25519.PrivateKey(operatorKey), *binFile, *matchHash, *outputDir)
	}
}

func indexOne(schema *gh.PoolSchema, weights map[string]float64, operatorKey ed25519.PrivateKey, binPath, mHash, outDir string) {
	rec, err := processbin(schema, weights, operatorKey, binPath)
	if err != nil {
		log.Fatalf("processing %s: %v", binPath, err)
	}

	outPath := filepath.Join(outDir, mHash+".rec")
	if err := gh.WriteRecFile(outPath, *rec); err != nil {
		log.Fatalf("writing %s: %v", outPath, err)
	}
	fmt.Printf("indexed %s → %s (%d dims, %d filters)\n", binPath, outPath, len(rec.Vector), len(rec.Filters.Fields))
}

func rebuildAll(schema *gh.PoolSchema, weights map[string]float64, operatorKey ed25519.PrivateKey, usersDir, outDir string) {
	// Delete existing .rec files
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".rec") {
			os.Remove(filepath.Join(outDir, e.Name()))
		}
	}

	entries, err := os.ReadDir(usersDir)
	if err != nil {
		log.Fatalf("reading users dir: %v", err)
	}

	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		binPath := filepath.Join(usersDir, e.Name())
		binHash := strings.TrimSuffix(e.Name(), ".bin")

		rec, err := processbin(schema, weights, operatorKey, binPath)
		if err != nil {
			log.Printf("skipping %s: %v", binPath, err)
			continue
		}

		outPath := filepath.Join(outDir, binHash+".rec")
		if err := gh.WriteRecFile(outPath, *rec); err != nil {
			log.Printf("skipping %s: write error: %v", binPath, err)
			continue
		}
		count++
	}
	fmt.Printf("rebuilt %d records in %s\n", count, outDir)
}

func processbin(schema *gh.PoolSchema, weights map[string]float64, operatorKey ed25519.PrivateKey, binPath string) (*gh.IndexRecord, error) {
	binData, err := os.ReadFile(binPath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	plaintext, err := crypto.UnpackUserBin(operatorKey, binData)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	var profile map[string]any
	if err := json.Unmarshal(plaintext, &profile); err != nil {
		return nil, fmt.Errorf("parsing profile: %w", err)
	}

	filters := gh.ExtractFilters(schema, profile)
	vec := gh.EncodeProfile(schema, profile)
	vec = gh.ApplyWeights(schema, vec, weights)

	return &gh.IndexRecord{
		Filters:     filters,
		Vector:      vec,
		DisplayName: strField(profile, "display_name"),
		About:       strField(profile, "about"),
		Bio:         strField(profile, "bio"),
	}, nil
}

func strField(profile map[string]any, key string) string {
	if v, ok := profile[key].(string); ok {
		return v
	}
	return ""
}
