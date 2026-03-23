package main

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/pooldb"
)

func cmdIndex() {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	_ = fs.String("schema", "pool.yaml", "path to pool.yaml (reserved for future matching engine)")
	weightsStr := fs.String("weights", "", "JSON weights (or INDEXER_WEIGHTS env var)")
	operatorKeyHex := fs.String("operator-key", "", "operator ed25519 private key (hex)")
	binFile := fs.String("bin-file", "", "single .bin file to index")
	matchHash := fs.String("match-hash", "", "match_hash for output filename")
	outputDir := fs.String("output-dir", "index", "directory for .rec files (single-user mode)")
	output := fs.String("output", "", "output file path for index.db")
	upload := fs.Bool("upload", false, "upload index.db as a GitHub release asset")
	usersDir := fs.String("users-dir", "users", "path to users/ directory")
	salt := fs.String("salt", "", "pool salt (or POOL_SALT env var)")
	poolURL := fs.String("pool-url", "", "pool URL (or REPO env var)")
	fs.Parse(os.Args[2:])

	// Resolve weights (reserved for future matching engine)
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
	_ = weightMap // reserved for future matching engine

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

	if *binFile != "" {
		// Single-user mode
		if *matchHash == "" {
			log.Fatal("single-user mode requires --match-hash")
		}
		os.MkdirAll(*outputDir, 0755)
		indexOne(ed25519.PrivateKey(operatorKey), *binFile, *matchHash, *outputDir)
	} else {
		// Rebuild mode (default)
		poolSalt := *salt
		if poolSalt == "" {
			poolSalt = os.Getenv("POOL_SALT")
		}
		pURL := *poolURL
		if pURL == "" {
			pURL = os.Getenv("REPO")
		}

		outPath := *output
		if outPath == "" {
			outPath = "index.db"
		}
		rebuildAll(ed25519.PrivateKey(operatorKey), poolSalt, *usersDir, outPath)

		if *upload {
			repo := os.Getenv("REPO")
			if repo == "" {
				writeError("REPO env var required for --upload")
			}
			ghCli, err := gh.NewCLI(repo)
			if err != nil {
				writeError("github CLI: " + err.Error())
			}
			if err := ghCli.UploadReleaseAsset(context.Background(), "index-latest", "index.db", outPath); err != nil {
				writeError("uploading index: " + err.Error())
			}
			log.Printf("uploaded %s as release asset", outPath)
		}
	}
}

func indexOne(operatorKey ed25519.PrivateKey, binPath, mHash, outDir string) {
	rec, err := processbin(operatorKey, binPath)
	if err != nil {
		log.Fatalf("processing %s: %v", binPath, err)
	}

	outPath := filepath.Join(outDir, mHash+".rec")
	if err := gh.WriteRecFile(outPath, *rec); err != nil {
		log.Fatalf("writing %s: %v", outPath, err)
	}
	fmt.Printf("indexed %s → %s\n", binPath, outPath)
}

// rebuildAll processes all .bin files into a single index.db (SQLite).
func rebuildAll(operatorKey ed25519.PrivateKey, poolSalt, usersDir, outPath string) {
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		log.Fatalf("reading users dir: %v", err)
	}

	// Remove existing index.db to start fresh
	os.Remove(outPath)

	indexDB, err := pooldb.Open(outPath)
	if err != nil {
		writeError("opening index db: " + err.Error())
	}
	defer indexDB.Close()

	indexDB.ClearProfiles()

	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		binPath := filepath.Join(usersDir, e.Name())
		binHash := strings.TrimSuffix(e.Name(), ".bin")

		rec, err := processbin(operatorKey, binPath)
		if err != nil {
			log.Printf("skipping %s: %v", binPath, err)
			continue
		}

		// Compute match_hash from bin_hash
		matchHash := sha256Short(poolSalt + ":" + binHash)

		filtersJSON, _ := json.Marshal(rec.Filters)
		vectorBytes := encodeFloat32s(rec.Vector)

		if err := indexDB.InsertProfile(pooldb.Profile{
			MatchHash:   matchHash,
			Filters:     string(filtersJSON),
			Vector:      vectorBytes,
			DisplayName: rec.DisplayName,
			About:       rec.About,
			Bio:         rec.Bio,
			UpdatedAt:   time.Now(),
		}); err != nil {
			log.Printf("skipping %s: %v", binPath, err)
			continue
		}
		count++
	}

	fmt.Printf("rebuilt %d records → %s\n", count, outPath)
}

// encodeFloat32s encodes a slice of float32 values to little-endian bytes.
func encodeFloat32s(fs []float32) []byte {
	buf := make([]byte, len(fs)*4)
	for i, f := range fs {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func processbin(operatorKey ed25519.PrivateKey, binPath string) (*gh.IndexRecord, error) {
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

	// Store basic profile data. Vector encoding and matching
	// will be added when the matching engine is designed.
	return &gh.IndexRecord{
		Filters:     gh.FilterValues{Fields: make(map[string]int)},
		Vector:      nil,
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
