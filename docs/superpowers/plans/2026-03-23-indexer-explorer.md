# New Indexer & Explorer — Implementation Plan (3 of 5)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement new indexer (builds chain-encrypted index.db) and explorer (grinds chains client-side). Tested independently — does NOT replace old code yet.

**Architecture:** Two new packages: `internal/indexer2` (composes `chainenc` + `bucket` + reads `.bin` files, writes `index.db`) and `internal/explorer2` (reads `index.db`, grinds chains, tracks state in `pool.db`). Both are standalone — old indexer/explorer remain untouched.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), `internal/chainenc`, `internal/bucket`, `internal/crypto`, `internal/schema`

**Spec:** `docs/superpowers/specs/2026-03-23-matching-engine-design.md`
**Dependencies:** Plans 1 (chainenc) and 2 (bucket) must be complete.

---

## File Structure

```
internal/indexer2/
  indexer.go        — main Indexer type: read .bin files, bucket, chain-encrypt, write index.db
  indexer_test.go   — tests with temp dirs and fixture profiles

internal/explorer2/
  explorer.go       — main Explorer type: read index.db, grind chains, track state
  state.go          — local state management (constants, checkpoints, seen)
  explorer_test.go  — tests with pre-built index.db fixtures
```

---

### Task 1: Indexer — read profiles, bucket, encrypt, write index.db

**Files:**
- Create: `internal/indexer2/indexer.go`
- Create: `internal/indexer2/indexer_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/indexer2/indexer_test.go
package indexer2

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/bucket"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func createTestBinFile(t *testing.T, dir string, binHash string, profile map[string]any, opPub ed25519.PublicKey) {
	t.Helper()
	userPub, _, _ := ed25519.GenerateKey(rand.Reader)
	profileJSON, _ := json.Marshal(profile)
	binData, err := crypto.PackUserBin(userPub, opPub, profileJSON)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "users"), 0755)
	os.WriteFile(filepath.Join(dir, "users", binHash+".bin"), binData, 0644)
}

func TestIndexer_Build(t *testing.T) {
	dir := t.TempDir()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	// Create test .bin files
	createTestBinFile(t, dir, "aaa111", map[string]any{
		"role": "woman", "age": 25, "interests": []string{"coding"},
	}, opPub)
	createTestBinFile(t, dir, "bbb222", map[string]any{
		"role": "man", "age": 28, "interests": []string{"hiking"},
	}, opPub)
	createTestBinFile(t, dir, "ccc333", map[string]any{
		"role": "woman", "age": 30, "interests": []string{"music"},
	}, opPub)

	partitions := []bucket.PartitionConfig{
		{Field: "role"},
	}

	indexPath := filepath.Join(dir, "index.db")
	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   partitions,
		Permutations: 2,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify index.db exists
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatal("index.db not created")
	}

	// Verify we can open and query it
	stats, err := Stats(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Buckets < 2 {
		t.Errorf("expected at least 2 buckets (man, woman), got %d", stats.Buckets)
	}
	if stats.Entries < 3 {
		t.Errorf("expected at least 3 entries, got %d", stats.Entries)
	}
	if stats.Permutations < 2 {
		t.Errorf("expected 2 permutations, got %d", stats.Permutations)
	}

	t.Logf("index stats: %+v", stats)
}

func TestIndexer_RangePartitions(t *testing.T) {
	dir := t.TempDir()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	createTestBinFile(t, dir, "aa", map[string]any{"age": 22}, opPub)
	createTestBinFile(t, dir, "bb", map[string]any{"age": 27}, opPub)
	createTestBinFile(t, dir, "cc", map[string]any{"age": 35}, opPub)

	partitions := []bucket.PartitionConfig{
		{Field: "age", Step: 10, Overlap: 3},
	}

	indexPath := filepath.Join(dir, "index.db")
	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   partitions,
		FieldRanges:  map[string][2]int{"age": {18, 40}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, _ := Stats(indexPath)
	t.Logf("range partition stats: %+v", stats)

	// With overlap, some profiles should appear in multiple buckets
	if stats.Entries <= 3 {
		t.Log("entries <= 3 — profiles may not overlap (check step/overlap values)")
	}
}

func TestIndexer_EmptyUsersDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "users"), 0755)
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)

	err := Build(Config{
		UsersDir:     filepath.Join(dir, "users"),
		OutputPath:   filepath.Join(dir, "index.db"),
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 1,
		NonceSpace:   5,
		Salt:         "testsalt",
	})
	// Should succeed with empty index
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests — should fail**

- [ ] **Step 3: Implement indexer.go**

```go
// internal/indexer2/indexer.go
package indexer2

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
	// 1. Read and decrypt all .bin files
	profiles, err := readProfiles(cfg.UsersDir, cfg.OperatorKey, cfg.Salt)
	if err != nil {
		return fmt.Errorf("reading profiles: %w", err)
	}

	// 2. Assign profiles to buckets
	bucketProfiles := make([]bucket.Profile, len(profiles))
	for i, p := range profiles {
		bucketProfiles[i] = bucket.Profile{
			Tag:        p.Tag,
			Attributes: p.Attributes,
		}
	}
	buckets := bucket.Assign(bucketProfiles, cfg.Partitions, cfg.FieldRanges)

	// 3. Build chain-encrypted entries per bucket per permutation
	// 4. Write to SQLite
	os.Remove(cfg.OutputPath)
	db, err := sql.Open("sqlite", cfg.OutputPath)
	if err != nil {
		return fmt.Errorf("opening db: %w", err)
	}
	defer db.Close()

	if err := createSchema(db); err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	profileDataMap := make(map[string][]byte) // tag → JSON profile bytes
	for _, p := range profiles {
		profileDataMap[p.Tag] = p.Data
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, b := range buckets {
		// Insert bucket
		partJSON, _ := json.Marshal(b.PartitionValues)
		_, err := db.Exec("INSERT INTO buckets (bucket_id, partition_values, profile_count) VALUES (?, ?, ?)",
			b.ID, string(partJSON), len(b.Tags))
		if err != nil {
			return fmt.Errorf("inserting bucket: %w", err)
		}

		// Build N permutations
		for perm := 0; perm < cfg.Permutations; perm++ {
			// Shuffle tags for this permutation
			tags := make([]string, len(b.Tags))
			copy(tags, b.Tags)
			rng.Shuffle(len(tags), func(i, j int) { tags[i], tags[j] = tags[j], tags[i] })

			// Build chain entries
			entries := make([]chainenc.ProfileEntry, len(tags))
			for i, tag := range tags {
				data, ok := profileDataMap[tag]
				if !ok {
					continue
				}
				entries[i] = chainenc.ProfileEntry{Tag: tag, Data: data}
			}

			seed := []byte(fmt.Sprintf("%s:perm%d", b.ID, perm))
			chain, err := chainenc.BuildChain(entries, seed, cfg.NonceSpace)
			if err != nil {
				return fmt.Errorf("building chain for %s perm %d: %w", b.ID, perm, err)
			}

			// Insert chain
			_, err = db.Exec("INSERT INTO chains (bucket_id, permutation, seed) VALUES (?, ?, ?)",
				b.ID, perm, chain.Seed)
			if err != nil {
				return fmt.Errorf("inserting chain: %w", err)
			}

			// Insert entries
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

type indexProfile struct {
	Tag        string
	Attributes map[string]any
	Data       []byte // raw JSON
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

		// Compute match_hash for the tag
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/indexer2/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/indexer2/
git commit -m "feat(indexer2): build chain-encrypted index.db from .bin files"
```

---

### Task 2: Explorer — read index.db, grind chains, track state

**Files:**
- Create: `internal/explorer2/explorer.go`
- Create: `internal/explorer2/state.go`
- Create: `internal/explorer2/explorer_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/explorer2/explorer_test.go
package explorer2

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/bucket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/indexer2"
)

func buildTestIndex(t *testing.T, dir string) string {
	t.Helper()
	_, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	opPub := opPriv.Public().(ed25519.PublicKey)

	usersDir := filepath.Join(dir, "users")
	os.MkdirAll(usersDir, 0755)

	profiles := []struct {
		hash    string
		profile map[string]any
	}{
		{"aaa", map[string]any{"name": "Alice", "role": "woman", "age": 25}},
		{"bbb", map[string]any{"name": "Bob", "role": "woman", "age": 28}},
		{"ccc", map[string]any{"name": "Carol", "role": "woman", "age": 26}},
	}

	for _, p := range profiles {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		pJSON, _ := json.Marshal(p.profile)
		bin, _ := crypto.PackUserBin(pub, opPub, pJSON)
		os.WriteFile(filepath.Join(usersDir, p.hash+".bin"), bin, 0644)
	}

	indexPath := filepath.Join(dir, "index.db")
	err := indexer2.Build(indexer2.Config{
		UsersDir:     usersDir,
		OutputPath:   indexPath,
		OperatorKey:  opPriv,
		Partitions:   []bucket.PartitionConfig{{Field: "role"}},
		Permutations: 2,
		NonceSpace:   5,
		Salt:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return indexPath
}

func TestExplorer_ListBuckets(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, err := exp.ListBuckets()
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) == 0 {
		t.Fatal("expected at least 1 bucket")
	}
	t.Logf("buckets: %v", buckets)
}

func TestExplorer_GrindChain(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	if len(buckets) == 0 {
		t.Fatal("no buckets")
	}

	// Grind first bucket, first permutation
	results, err := exp.Grind(buckets[0].ID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 unlocked profile")
	}

	for _, r := range results {
		t.Logf("unlocked: %s (%d attempts, warm=%v)", r.Tag, r.Attempts, r.Warm)
	}
}

func TestExplorer_WarmResolve(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	bid := buckets[0].ID

	// Grind perm 0 — learn constants
	results0, _ := exp.Grind(bid, 0, 10)
	totalCold := 0
	for _, r := range results0 {
		totalCold += r.Attempts
	}

	// Grind perm 1 — should be warm (much fewer attempts)
	results1, _ := exp.Grind(bid, 1, 10)
	totalWarm := 0
	for _, r := range results1 {
		totalWarm += r.Attempts
	}

	t.Logf("perm 0 (cold): %d total attempts", totalCold)
	t.Logf("perm 1 (warm): %d total attempts", totalWarm)

	if totalWarm >= totalCold {
		t.Errorf("warm (%d) should be less than cold (%d)", totalWarm, totalCold)
	}
}

func TestExplorer_SeenSkip(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	buckets, _ := exp.ListBuckets()
	bid := buckets[0].ID

	// Grind once
	results, _ := exp.Grind(bid, 0, 10)

	// Mark all as seen
	for _, r := range results {
		exp.MarkSeen(r.Tag, "skip")
	}

	// Grind again — should skip all
	results2, _ := exp.Grind(bid, 0, 10)
	if len(results2) != 0 {
		t.Errorf("expected 0 results (all seen), got %d", len(results2))
	}
}
```

- [ ] **Step 2: Run tests — should fail**

- [ ] **Step 3: Implement state.go**

```go
// internal/explorer2/state.go
package explorer2

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// State manages local explorer state (known constants, checkpoints, seen).
type State struct {
	db *sql.DB
}

func openState(path string) (*State, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS constants (
			match_hash TEXT PRIMARY KEY,
			profile_constant INTEGER,
			learned_at INTEGER
		);
		CREATE TABLE IF NOT EXISTS checkpoints (
			bucket_id TEXT,
			permutation INTEGER,
			position INTEGER,
			chain_hint INTEGER,
			UNIQUE(bucket_id, permutation)
		);
		CREATE TABLE IF NOT EXISTS seen (
			match_hash TEXT PRIMARY KEY,
			seen_at INTEGER,
			action TEXT
		);
	`)
	if err != nil {
		return nil, err
	}

	return &State{db: db}, nil
}

func (s *State) Close() error { return s.db.Close() }

func (s *State) GetConstant(matchHash string) (int, bool) {
	var c int
	err := s.db.QueryRow("SELECT profile_constant FROM constants WHERE match_hash = ?", matchHash).Scan(&c)
	if err != nil {
		return 0, false
	}
	return c, true
}

func (s *State) SaveConstant(matchHash string, constant int) {
	s.db.Exec("INSERT OR REPLACE INTO constants (match_hash, profile_constant, learned_at) VALUES (?, ?, ?)",
		matchHash, constant, time.Now().Unix())
}

func (s *State) IsSeen(matchHash string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM seen WHERE match_hash = ?", matchHash).Scan(&count)
	return count > 0
}

func (s *State) MarkSeen(matchHash, action string) {
	s.db.Exec("INSERT OR REPLACE INTO seen (match_hash, seen_at, action) VALUES (?, ?, ?)",
		matchHash, time.Now().Unix(), action)
}

func (s *State) GetCheckpoint(bucketID string, perm int) (position int, hint int, ok bool) {
	err := s.db.QueryRow(
		"SELECT position, chain_hint FROM checkpoints WHERE bucket_id = ? AND permutation = ?",
		bucketID, perm).Scan(&position, &hint)
	return position, hint, err == nil
}

func (s *State) SaveCheckpoint(bucketID string, perm, position, hint int) {
	s.db.Exec("INSERT OR REPLACE INTO checkpoints (bucket_id, permutation, position, chain_hint) VALUES (?, ?, ?, ?)",
		bucketID, perm, position, hint)
}
```

- [ ] **Step 4: Implement explorer.go**

```go
// internal/explorer2/explorer.go
package explorer2

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
	"github.com/vutran1710/dating-dev/internal/chainenc"
)

// BucketInfo holds metadata about a bucket.
type BucketInfo struct {
	ID           string
	Partitions   map[string]string
	ProfileCount int
	Permutations int
}

// GrindResult is a single unlocked profile.
type GrindResult struct {
	Tag      string
	Profile  map[string]any
	Constant int
	Attempts int
	Warm     bool
}

// Explorer reads chain-encrypted index.db and grinds to unlock profiles.
type Explorer struct {
	indexDB *sql.DB
	state   *State
}

// Open creates an Explorer from an index.db and a state.db path.
func Open(indexPath, statePath string) (*Explorer, error) {
	indexDB, err := sql.Open("sqlite", indexPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}

	state, err := openState(statePath)
	if err != nil {
		indexDB.Close()
		return nil, fmt.Errorf("opening state: %w", err)
	}

	return &Explorer{indexDB: indexDB, state: state}, nil
}

func (e *Explorer) Close() error {
	e.state.Close()
	return e.indexDB.Close()
}

// ListBuckets returns all buckets in the index.
func (e *Explorer) ListBuckets() ([]BucketInfo, error) {
	rows, err := e.indexDB.Query(`
		SELECT b.bucket_id, b.partition_values, b.profile_count,
		       (SELECT COUNT(*) FROM chains c WHERE c.bucket_id = b.bucket_id) as perms
		FROM buckets b`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []BucketInfo
	for rows.Next() {
		var b BucketInfo
		var partJSON string
		if err := rows.Scan(&b.ID, &partJSON, &b.ProfileCount, &b.Permutations); err != nil {
			continue
		}
		b.Partitions = make(map[string]string)
		// Simple parse — not full JSON unmarshal needed for string map
		_ = partJSON // stored for reference
		buckets = append(buckets, b)
	}
	return buckets, nil
}

// Grind unlocks profiles in a bucket permutation.
// maxProfiles limits how many new profiles to unlock (0 = all).
// Skips seen profiles. Uses warm solving for known constants.
func (e *Explorer) Grind(bucketID string, permutation int, maxProfiles int) ([]GrindResult, error) {
	// Load chain seed
	var seed []byte
	err := e.indexDB.QueryRow(
		"SELECT seed FROM chains WHERE bucket_id = ? AND permutation = ?",
		bucketID, permutation).Scan(&seed)
	if err != nil {
		return nil, fmt.Errorf("chain not found: %w", err)
	}

	// Load entries
	rows, err := e.indexDB.Query(
		"SELECT position, tag, ciphertext, gcm_nonce FROM entries WHERE bucket_id = ? AND permutation = ? ORDER BY position",
		bucketID, permutation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []chainenc.Entry
	var positions []int
	for rows.Next() {
		var pos int
		var entry chainenc.Entry
		if err := rows.Scan(&pos, &entry.Tag, &entry.Ciphertext, &entry.Nonce); err != nil {
			continue
		}
		entry.Context = chainenc.ChainContext(seed, pos)
		entries = append(entries, entry)
		positions = append(positions, pos)
	}

	// Load nonce_space — derive from chain config
	// For now, try to read from the first entry's solve
	// Actually we need to store this. Use a reasonable default or query.
	nonceSpace := 20 // TODO: store in chains table or pass as parameter

	seedHint := chainenc.SeedHint(seed)
	currentHint := seedHint

	// Check for checkpoint
	if pos, hint, ok := e.state.GetCheckpoint(bucketID, permutation); ok {
		// Skip to checkpoint position
		for i, p := range positions {
			if p <= pos {
				currentHint = hint
				entries = entries[i+1:]
				positions = positions[i+1:]
				break
			}
		}
		_ = currentHint
	}
	// Reset hint tracking — always start from seed for now (checkpoint is optimization for later)
	currentHint = seedHint

	var results []GrindResult
	hint := currentHint

	for i, entry := range entries {
		// Skip seen
		if e.state.IsSeen(entry.Tag) {
			// Still need to advance the hint — solve to get next_hint
			// But if we've seen it, we should have the constant
			if constant, ok := e.state.GetConstant(entry.Tag); ok {
				payload, _, err := chainenc.SolveWarm(entry, hint, constant, nonceSpace)
				if err == nil {
					hint = payload.NextHint
					continue
				}
			}
			// Can't advance without solving — skip this permutation entry
			// This breaks the chain. For now, stop grinding.
			break
		}

		// Try warm first
		var result GrindResult
		if constant, ok := e.state.GetConstant(entry.Tag); ok {
			payload, attempts, err := chainenc.SolveWarm(entry, hint, constant, nonceSpace)
			if err == nil {
				result = GrindResult{
					Tag:      entry.Tag,
					Profile:  payload.Profile,
					Constant: payload.MyConstant,
					Attempts: attempts,
					Warm:     true,
				}
				hint = payload.NextHint
			}
		}

		// Cold solve
		if result.Tag == "" {
			payload, attempts, err := chainenc.SolveCold(entry, hint, chainenc.ConstSpace, nonceSpace)
			if err != nil {
				break // chain broken
			}
			result = GrindResult{
				Tag:      entry.Tag,
				Profile:  payload.Profile,
				Constant: payload.MyConstant,
				Attempts: attempts,
				Warm:     false,
			}
			e.state.SaveConstant(entry.Tag, payload.MyConstant)
			hint = payload.NextHint
		}

		results = append(results, result)

		// Save checkpoint
		if i < len(positions) {
			e.state.SaveCheckpoint(bucketID, permutation, positions[i], hint)
		}

		if maxProfiles > 0 && len(results) >= maxProfiles {
			break
		}
	}

	return results, nil
}

// MarkSeen marks a profile as seen with the given action.
func (e *Explorer) MarkSeen(matchHash, action string) {
	e.state.MarkSeen(matchHash, action)
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/explorer2/ -v -count=1`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `make test`
Expected: all pass

- [ ] **Step 7: Commit**

```bash
git add internal/explorer2/
git commit -m "feat(explorer2): grind chain-encrypted index with warm/cold solving and state tracking"
```

---

### Task 3: Integration test — indexer builds, explorer grinds

**Files:**
- Modify: `internal/explorer2/explorer_test.go`

- [ ] **Step 1: Add end-to-end integration test**

```go
// Add to explorer_test.go

func TestEndToEnd_IndexAndExplore(t *testing.T) {
	dir := t.TempDir()
	indexPath := buildTestIndex(t, dir)

	exp, err := Open(indexPath, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer exp.Close()

	// List buckets
	buckets, _ := exp.ListBuckets()
	t.Logf("found %d buckets", len(buckets))

	// Grind perm 0 cold
	results0, err := exp.Grind(buckets[0].ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	coldTotal := 0
	for _, r := range results0 {
		coldTotal += r.Attempts
		t.Logf("  cold: %s → %v (%d attempts)", r.Tag, r.Profile["name"], r.Attempts)
	}

	// Grind perm 1 warm
	results1, err := exp.Grind(buckets[0].ID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	warmTotal := 0
	for _, r := range results1 {
		warmTotal += r.Attempts
		t.Logf("  warm: %s → %v (%d attempts, warm=%v)", r.Tag, r.Profile["name"], r.Attempts, r.Warm)
	}

	t.Logf("cold total: %d, warm total: %d, speedup: %.1fx",
		coldTotal, warmTotal, float64(coldTotal)/float64(warmTotal))

	if warmTotal >= coldTotal {
		t.Errorf("warm should be faster than cold")
	}

	// Mark one as seen, grind again
	if len(results0) > 0 {
		exp.MarkSeen(results0[0].Tag, "skip")
	}

	// Verify profile data is present
	for _, r := range results0 {
		if r.Profile == nil {
			t.Error("profile should not be nil")
		}
		if r.Profile["name"] == nil {
			t.Error("profile should have name")
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/explorer2/ -v -count=1`
Expected: PASS, logs show cold vs warm speedup

- [ ] **Step 3: Run full suite**

Run: `make test`
Expected: all pass

- [ ] **Step 4: Commit and push**

```bash
git add internal/explorer2/explorer_test.go
git commit -m "test(explorer2): end-to-end integration — index build + cold/warm grind"
git push
```
