// indexer processes encrypted .bin profiles into a single index.pack for discovery.
// Used by GitHub Actions (cron) to rebuild the pool's discovery index.
//
// Rebuild mode (cron):
//
//	indexer --pool-json pool.json --rebuild --users-dir users/ --output index.pack --operator-key hex
//
// Single-user mode (legacy):
//
//	indexer --pool-json pool.json --bin-file users/abc.bin --match-hash abc123 --operator-key hex --output-dir index/
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func main() {
	poolJSON := flag.String("pool-json", "pool.json", "path to pool.json with schema")
	weightsStr := flag.String("weights", "", "JSON weights (or INDEXER_WEIGHTS env var)")
	operatorKeyHex := flag.String("operator-key", "", "operator ed25519 private key (hex)")
	binFile := flag.String("bin-file", "", "single .bin file to index")
	matchHash := flag.String("match-hash", "", "match_hash for output filename")
	outputDir := flag.String("output-dir", "index", "directory for .rec files (single-user mode)")
	output := flag.String("output", "", "output file path for index.pack (rebuild mode)")
	rebuild := flag.Bool("rebuild", false, "rebuild entire index from all .bin files")
	usersDir := flag.String("users-dir", "users", "path to users/ directory")
	salt := flag.String("salt", "", "pool salt (or POOL_SALT env var, needed for hash computation in rebuild)")
	poolURL := flag.String("pool-url", "", "pool URL (or from pool.json, needed for hash computation in rebuild)")
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

	if *rebuild {
		// Resolve salt + poolURL for hash computation
		poolSalt := *salt
		if poolSalt == "" {
			poolSalt = os.Getenv("POOL_SALT")
		}
		pURL := *poolURL
		if pURL == "" {
			pURL = manifest.Name
		}

		outPath := *output
		if outPath == "" {
			outPath = "index.pack"
		}
		rebuildAll(manifest.Schema, weightMap, ed25519.PrivateKey(operatorKey), poolSalt, pURL, *usersDir, outPath)
	} else {
		if *binFile == "" || *matchHash == "" {
			log.Fatal("single-user mode requires --bin-file and --match-hash")
		}
		os.MkdirAll(*outputDir, 0755)
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

// rebuildAll processes all .bin files into a single index.pack (msgpack).
func rebuildAll(schema *gh.PoolSchema, weights map[string]float64, operatorKey ed25519.PrivateKey, poolSalt, poolURL, usersDir, outPath string) {
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		log.Fatalf("reading users dir: %v", err)
	}

	var records []gh.NamedRecord
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

		// Compute match_hash from bin_hash
		matchHash := sha256Short(poolSalt + ":" + binHash)

		records = append(records, gh.NamedRecord{
			MatchHash: matchHash,
			Record:    *rec,
		})
	}

	// Write index.pack
	data, err := msgpack.Marshal(records)
	if err != nil {
		log.Fatalf("marshaling index: %v", err)
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		log.Fatalf("writing %s: %v", outPath, err)
	}
	fmt.Printf("rebuilt %d records → %s (%d bytes)\n", len(records), outPath, len(data))
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

func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}
