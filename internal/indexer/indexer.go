// Package indexer2 builds chain-encrypted index.db from .bin files.
// Composes chainenc + bucket packages. Does NOT replace the old indexer.
package indexer

import (
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"github.com/vutran1710/dating-dev/internal/bucket"
	"github.com/vutran1710/dating-dev/internal/chainenc"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

// Config holds all parameters for building an index.
type Config struct {
	UsersDir     string
	OutputPath   string
	OperatorKey  ed25519.PrivateKey
	Partitions   []bucket.PartitionConfig
	FieldRanges  map[string][2]int // for range partitions: field → [min, max]
	Permutations int
	NonceSpace   int
	Salt         string
}

// IndexStats holds summary stats for an index.db.
type IndexStats struct {
	Buckets      int
	Entries      int
	Permutations int
}

// Build reads .bin files, buckets profiles, chain-encrypts, and writes index.db.
func Build(cfg Config) error {
	profiles, err := readProfiles(cfg.UsersDir, cfg.OperatorKey, cfg.Salt)
	if err != nil {
		return fmt.Errorf("reading profiles: %w", err)
	}

	if len(profiles) == 0 {
		// Create empty index.db with schema
		return createEmptyDB(cfg.OutputPath)
	}

	// Assign profiles to buckets
	bucketProfiles := make([]bucket.Profile, len(profiles))
	for i, p := range profiles {
		bucketProfiles[i] = bucket.Profile{
			Tag:        p.Tag,
			Attributes: p.Attributes,
		}
	}
	buckets := bucket.Assign(bucketProfiles, cfg.Partitions, cfg.FieldRanges)

	// Build profile data lookup
	profileDataMap := make(map[string][]byte)
	for _, p := range profiles {
		profileDataMap[p.Tag] = p.Data
	}

	// Write to SQLite
	os.Remove(cfg.OutputPath)
	db, err := sql.Open("sqlite", cfg.OutputPath)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, b := range buckets {
		partJSON, _ := json.Marshal(b.PartitionValues)
		_, err := db.Exec("INSERT INTO buckets (bucket_id, partition_values, profile_count) VALUES (?, ?, ?)",
			b.ID, string(partJSON), len(b.Tags))
		if err != nil {
			return fmt.Errorf("inserting bucket: %w", err)
		}

		for perm := 0; perm < cfg.Permutations; perm++ {
			tags := make([]string, len(b.Tags))
			copy(tags, b.Tags)
			rng.Shuffle(len(tags), func(i, j int) { tags[i], tags[j] = tags[j], tags[i] })

			entries := make([]chainenc.ProfileEntry, 0, len(tags))
			for _, tag := range tags {
				data, ok := profileDataMap[tag]
				if !ok {
					continue
				}
				entries = append(entries, chainenc.ProfileEntry{Tag: tag, Data: data})
			}

			if len(entries) == 0 {
				continue
			}

			seed := []byte(fmt.Sprintf("%s:perm%d", b.ID, perm))
			chain, err := chainenc.BuildChain(entries, seed, cfg.NonceSpace)
			if err != nil {
				return fmt.Errorf("building chain for %s perm %d: %w", b.ID, perm, err)
			}

			_, err = db.Exec("INSERT INTO chains (bucket_id, permutation, seed, nonce_space) VALUES (?, ?, ?, ?)",
				b.ID, perm, chain.Seed, cfg.NonceSpace)
			if err != nil {
				return fmt.Errorf("inserting chain: %w", err)
			}

			for pos, entry := range chain.Entries {
				_, err = db.Exec(
					"INSERT INTO entries (bucket_id, permutation, position, tag, ciphertext, gcm_nonce) VALUES (?, ?, ?, ?, ?, ?)",
					b.ID, perm, pos, entry.Tag, entry.Ciphertext, entry.Nonce)
				if err != nil {
					return fmt.Errorf("inserting entry: %w", err)
				}
			}
		}
	}

	return nil
}

// Stats returns summary stats for an index.db.
func Stats(dbPath string) (IndexStats, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return IndexStats{}, err
	}
	defer db.Close()

	var stats IndexStats
	db.QueryRow("SELECT COUNT(*) FROM buckets").Scan(&stats.Buckets)
	db.QueryRow("SELECT COUNT(*) FROM entries").Scan(&stats.Entries)
	db.QueryRow("SELECT COUNT(DISTINCT permutation) FROM chains").Scan(&stats.Permutations)
	return stats, nil
}

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE buckets (
			bucket_id TEXT PRIMARY KEY,
			partition_values TEXT,
			profile_count INTEGER
		);
		CREATE TABLE chains (
			bucket_id TEXT,
			permutation INTEGER,
			seed BLOB,
			nonce_space INTEGER,
			UNIQUE(bucket_id, permutation)
		);
		CREATE TABLE entries (
			bucket_id TEXT,
			permutation INTEGER,
			position INTEGER,
			tag TEXT,
			ciphertext BLOB,
			gcm_nonce BLOB,
			UNIQUE(bucket_id, permutation, position)
		);
	`)
	return err
}

func createEmptyDB(path string) error {
	os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	return createSchema(db)
}

type indexProfile struct {
	Tag        string
	Attributes map[string]any
	Data       []byte
}

func readProfiles(usersDir string, opKey ed25519.PrivateKey, salt string) ([]indexProfile, error) {
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return nil, err
	}

	var profiles []indexProfile
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		binHash := strings.TrimSuffix(e.Name(), ".bin")
		binPath := filepath.Join(usersDir, e.Name())

		binData, err := os.ReadFile(binPath)
		if err != nil {
			continue
		}

		plaintext, err := crypto.UnpackUserBin(opKey, binData)
		if err != nil {
			continue
		}

		var attrs map[string]any
		if err := json.Unmarshal(plaintext, &attrs); err != nil {
			continue
		}

		matchHash := sha256Short(salt + ":" + binHash)
		profiles = append(profiles, indexProfile{
			Tag:        matchHash,
			Attributes: attrs,
			Data:       plaintext,
		})
	}

	return profiles, nil
}

func sha256Short(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])[:16]
}
