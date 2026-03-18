# Discovery Feature Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement client-side discovery — indexer produces per-user `.vec` files in pool repo, CLI syncs them into local SQLite, computes cosine similarity, shows tier-shuffled suggestions.

**Architecture:** Indexer (`cmd/indexer/`) runs in GitHub Action after registration. Writes `index/{match_hash}.vec`. CLI syncs pool repo, ingests `.vec` files into `~/.dating/pools/{name}/suggestions.db` (SQLite), queries for suggestions. All commands non-interactive.

**Tech Stack:** Go 1.25, SQLite (modernc.org/sqlite), msgpack, cosine similarity, ed25519/NaCl

**Spec:** `docs/superpowers/specs/2026-03-18-indexer-pool-schema-design.md`

---

### Task 1: Pool Schema Types

**Files:**
- Modify: `internal/github/types.go`

- [ ] **Step 1: Write failing test for schema parsing**

```go
// internal/github/types_test.go
func TestPoolManifest_SchemaFields(t *testing.T) {
    manifest := PoolManifest{
        Schema: &PoolSchema{
            Version: 1,
            Fields: []SchemaField{
                {Name: "gender", Type: "enum", Values: []string{"male", "female"}},
                {Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
                {Name: "interests", Type: "multi", Values: []string{"hiking", "coding"}},
            },
        },
    }
    if manifest.Schema.Dimensions() != 5 { // 2 + 1 + 2
        t.Errorf("dimensions = %d, want 5", manifest.Schema.Dimensions())
    }
}
```

- [ ] **Step 2: Add schema types to types.go**

```go
type PoolSchema struct {
    Version int           `json:"version"`
    Fields  []SchemaField `json:"fields"`
}

type SchemaField struct {
    Name   string   `json:"name"`
    Type   string   `json:"type"`             // "enum", "multi", "range"
    Values []string `json:"values,omitempty"`  // for enum/multi
    Min    *int     `json:"min,omitempty"`     // for range
    Max    *int     `json:"max,omitempty"`     // for range
}

// Dimensions returns the total vector dimensions for this field.
func (f SchemaField) Dimensions() int {
    switch f.Type {
    case "enum", "multi":
        return len(f.Values)
    case "range":
        return 1
    }
    return 0
}

// Dimensions returns the total vector dimensions across all fields.
func (s *PoolSchema) Dimensions() int {
    d := 0
    for _, f := range s.Fields {
        d += f.Dimensions()
    }
    return d
}
```

Add `Schema` field to `PoolManifest`:

```go
type PoolManifest struct {
    // ... existing fields ...
    Schema *PoolSchema `json:"schema,omitempty"`
}
```

- [ ] **Step 3: Run tests**

Run: `make test`

- [ ] **Step 4: Commit**

```bash
git add internal/github/types.go internal/github/types_test.go
git commit -m "feat: add pool schema types (enum, multi, range fields)"
```

---

### Task 2: Vector Encoding

**Files:**
- Create: `internal/github/vector.go`
- Create: `internal/github/vector_test.go`

- [ ] **Step 1: Write failing tests for vector encoding**

```go
func TestEncodeVector_Enum(t *testing.T) {
    field := SchemaField{Name: "gender", Type: "enum", Values: []string{"male", "female", "nb"}}
    vec := EncodeFieldValue(field, "female")
    // one-hot: [0, 1, 0]
    want := []float32{0, 1, 0}
    assertVec(t, vec, want)
}

func TestEncodeVector_Multi(t *testing.T) {
    field := SchemaField{Name: "interests", Type: "multi", Values: []string{"hiking", "coding", "music"}}
    vec := EncodeFieldValue(field, []string{"hiking", "music"})
    want := []float32{1, 0, 1}
    assertVec(t, vec, want)
}

func TestEncodeVector_Range(t *testing.T) {
    field := SchemaField{Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)}
    vec := EncodeFieldValue(field, 49)
    want := []float32{0.5} // (49-18)/(80-18) = 0.5
    assertVec(t, vec, want)
}

func TestEncodeProfile_FullSchema(t *testing.T) {
    schema := &PoolSchema{
        Version: 1,
        Fields: []SchemaField{
            {Name: "gender", Type: "enum", Values: []string{"male", "female"}},
            {Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
            {Name: "interests", Type: "multi", Values: []string{"a", "b", "c"}},
        },
    }
    profile := map[string]any{
        "gender": "female",
        "age": float64(49),
        "interests": []any{"a", "c"},
    }
    vec := EncodeProfile(schema, profile)
    // gender: [0,1] + age: [0.5] + interests: [1,0,1] = 6 dims
    if len(vec) != 6 {
        t.Errorf("len = %d, want 6", len(vec))
    }
}

func TestApplyWeights(t *testing.T) {
    schema := &PoolSchema{Fields: []SchemaField{
        {Name: "gender", Type: "enum", Values: []string{"m", "f"}},
        {Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
    }}
    vec := []float32{0, 1, 0.5}
    weights := map[string]float64{"gender": 10.0, "age": 2.0}
    result := ApplyWeights(schema, vec, weights)
    // gender dims * 10: [0, 10], age dim * 2: [1.0]
    want := []float32{0, 10, 1.0}
    assertVec(t, result, want)
}

func TestEncodeProfile_InvalidEnum(t *testing.T) {
    schema := &PoolSchema{Fields: []SchemaField{
        {Name: "gender", Type: "enum", Values: []string{"male", "female"}},
    }}
    profile := map[string]any{"gender": "invalid"}
    vec := EncodeProfile(schema, profile)
    // invalid value → zero vector for that field
    want := []float32{0, 0}
    assertVec(t, vec, want)
}

func TestEncodeProfile_MissingField(t *testing.T) {
    schema := &PoolSchema{Fields: []SchemaField{
        {Name: "gender", Type: "enum", Values: []string{"male", "female"}},
    }}
    profile := map[string]any{}
    vec := EncodeProfile(schema, profile)
    want := []float32{0, 0}
    assertVec(t, vec, want)
}
```

- [ ] **Step 2: Implement vector encoding**

```go
// internal/github/vector.go

// EncodeFieldValue encodes a single profile field value into vector dimensions.
func EncodeFieldValue(field SchemaField, value any) []float32

// EncodeProfile encodes an entire profile into a vector per the schema.
func EncodeProfile(schema *PoolSchema, profile map[string]any) []float32

// ApplyWeights multiplies each field's vector segment by its weight.
func ApplyWeights(schema *PoolSchema, vec []float32, weights map[string]float64) []float32
```

- [ ] **Step 3: Run tests**

Run: `make test`

- [ ] **Step 4: Commit**

```bash
git add internal/github/vector.go internal/github/vector_test.go
git commit -m "feat: vector encoding for pool schema fields + weight application"
```

---

### Task 3: Profile Validation Against Schema

**Files:**
- Create: `internal/github/validate.go`
- Create: `internal/github/validate_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestValidateProfile_Valid(t *testing.T) {
    schema := &PoolSchema{Fields: []SchemaField{
        {Name: "gender", Type: "enum", Values: []string{"male", "female"}},
        {Name: "age", Type: "range", Min: intPtr(18), Max: intPtr(80)},
        {Name: "interests", Type: "multi", Values: []string{"hiking", "coding"}},
    }}
    profile := map[string]any{"gender": "male", "age": float64(25), "interests": []any{"hiking"}}
    err := ValidateProfile(schema, profile)
    if err != nil {
        t.Errorf("expected valid, got: %v", err)
    }
}

func TestValidateProfile_InvalidEnum(t *testing.T) // "invalid" not in values
func TestValidateProfile_OutOfRange(t *testing.T)   // age: 5 (below min)
func TestValidateProfile_InvalidMulti(t *testing.T) // interest not in values
func TestValidateProfile_MissingField(t *testing.T) // required field missing
```

- [ ] **Step 2: Implement**

```go
// ValidateProfile checks a profile against the pool schema.
func ValidateProfile(schema *PoolSchema, profile map[string]any) error
```

- [ ] **Step 3: Run tests + commit**

---

### Task 4: Update `profile create` — Schema-Driven Scaffolding

**Files:**
- Modify: `internal/cli/profile.go`
- Modify: `internal/github/pool.go` (add GetManifest for local clone)

- [ ] **Step 1: Update `profile create` to fetch pool.json and scaffold from schema**

When a pool is in the config with a repo, `profile create` should:
1. Clone/sync the pool repo
2. Read `pool.json` → parse schema
3. Scaffold profile.json with schema field names + empty values
4. Write `profile.schema.json` alongside for editor reference

- [ ] **Step 2: Add `--pool-repo` flag for when pool isn't in config yet**

```
dating profile create my-pool --pool-repo owner/my-pool
```

- [ ] **Step 3: Update `pool join` to validate profile against schema before submitting**

- [ ] **Step 4: Write tests**

Test: scaffold from schema produces correct field names and types.
Test: validation rejects bad profiles.

- [ ] **Step 5: Run tests + commit**

---

### Task 5: Indexer Binary

**Files:**
- Create: `cmd/indexer/main.go`

- [ ] **Step 1: Implement indexer CLI**

```go
// cmd/indexer/main.go
// Flags:
//   --pool-json       path to pool.json
//   --weights         JSON string of weights (or INDEXER_WEIGHTS env)
//   --operator-key    hex ed25519 private key
//   --bin-file        single .bin file to index
//   --match-hash      match_hash for output filename
//   --output-dir      directory for .vec files (default: index/)
//   --rebuild         re-index all .bin files
//   --users-dir       path to users/ directory (for --rebuild)
```

Single-user mode:
1. Read pool.json schema
2. Read weights
3. Decrypt .bin file with operator key → profile JSON
4. Parse profile, encode to vector per schema
5. Apply weights
6. Write `{output-dir}/{match-hash}.vec` (raw float32 bytes)

Rebuild mode:
1. Delete all .vec files in output-dir
2. For each .bin in users-dir, process as above

- [ ] **Step 2: Verify it builds**

Run: `go build -o bin/indexer ./cmd/indexer`

- [ ] **Step 3: Smoke test**

Create a test .bin file, run indexer, verify .vec output size matches schema dimensions × 4 bytes.

- [ ] **Step 4: Commit**

```bash
git add cmd/indexer/main.go
git commit -m "feat: add indexer binary — profile to weighted vector"
```

---

### Task 6: `.vec` File I/O

**Files:**
- Create: `internal/github/vecfile.go`
- Create: `internal/github/vecfile_test.go`

- [ ] **Step 1: Write tests for .vec read/write**

```go
func TestWriteVecFile(t *testing.T) {
    dir := t.TempDir()
    vec := []float32{1.0, 2.0, 3.0}
    err := WriteVecFile(filepath.Join(dir, "abc123.vec"), vec)
    // verify file size = 12 bytes (3 × 4)
}

func TestReadVecFile(t *testing.T) {
    // write then read, verify round-trip
}

func TestReadVecDir(t *testing.T) {
    // write 3 .vec files, read dir, verify 3 records with correct match_hashes
}
```

- [ ] **Step 2: Implement**

```go
// WriteVecFile writes a vector as raw float32 bytes.
func WriteVecFile(path string, vec []float32) error

// ReadVecFile reads a vector from raw float32 bytes.
func ReadVecFile(path string) ([]float32, error)

// VecRecord is a match_hash + vector pair read from the index directory.
type VecRecord struct {
    MatchHash string
    Vector    []float32
}

// ReadVecDir reads all .vec files from a directory.
func ReadVecDir(dir string) ([]VecRecord, error)
```

- [ ] **Step 3: Run tests + commit**

---

### Task 7: Local SQLite Suggestions DB

**Files:**
- Create: `internal/cli/suggestions/db.go`
- Create: `internal/cli/suggestions/db_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestSuggestionsDB_InsertAndLoad(t *testing.T) {
    db := openTestDB(t)
    db.Upsert("hash1", []float32{1.0, 2.0, 3.0})
    db.Upsert("hash2", []float32{4.0, 5.0, 6.0})
    records := db.LoadAll()
    if len(records) != 2 { ... }
}

func TestSuggestionsDB_Upsert_ReplacesExisting(t *testing.T)
func TestSuggestionsDB_SyncFromVecDir(t *testing.T)
func TestSuggestionsDB_EmptyDB(t *testing.T)
```

- [ ] **Step 2: Implement suggestions DB**

```go
// internal/cli/suggestions/db.go
package suggestions

type DB struct {
    conn *sql.DB
}

// Open opens or creates a suggestions database.
func Open(path string) (*DB, error)

// Upsert inserts or replaces a vector record.
func (db *DB) Upsert(matchHash string, vector []float32) error

// LoadAll loads all records into memory.
func (db *DB) LoadAll() ([]Record, error)

// SyncFromDir reads .vec files from a directory and upserts new/changed records.
func (db *DB) SyncFromDir(dir string) (added int, err error)

// Close closes the database.
func (db *DB) Close() error

type Record struct {
    MatchHash string
    Vector    []float32
}
```

- [ ] **Step 3: Run tests + commit**

```bash
git add internal/cli/suggestions/
git commit -m "feat: local SQLite suggestions DB with sync from .vec files"
```

---

### Task 8: Cosine Similarity + Tier-Shuffled Ranking

**Files:**
- Create: `internal/cli/suggestions/rank.go`
- Create: `internal/cli/suggestions/rank_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCosine_Identical(t *testing.T) {
    a := []float32{1, 0, 1}
    if cosine(a, a) != 1.0 { ... }
}

func TestCosine_Orthogonal(t *testing.T) {
    a := []float32{1, 0}
    b := []float32{0, 1}
    if cosine(a, b) != 0.0 { ... }
}

func TestRankSuggestions(t *testing.T) {
    me := []float32{1, 0, 1, 0}
    records := []Record{
        {MatchHash: "close", Vector: []float32{1, 0, 1, 0}},   // identical
        {MatchHash: "far", Vector: []float32{0, 1, 0, 1}},     // opposite
        {MatchHash: "mid", Vector: []float32{1, 0, 0, 1}},     // partial
    }
    ranked := RankSuggestions(me, "my_hash", records, 10)
    if ranked[0].MatchHash != "close" { ... }
}

func TestRankSuggestions_ExcludesSelf(t *testing.T)
func TestRankSuggestions_TierShuffling(t *testing.T)
func TestRankSuggestions_Limit(t *testing.T)
```

- [ ] **Step 2: Implement**

```go
type Suggestion struct {
    MatchHash string
    Score     float64
}

// RankSuggestions computes cosine similarity, tier-shuffles, returns top N.
func RankSuggestions(myVec []float32, myHash string, records []Record, limit int) []Suggestion

func cosine(a, b []float32) float64
```

- [ ] **Step 3: Run tests + commit**

---

### Task 9: `pool sync` Command

**Files:**
- Modify: `internal/cli/pool.go`

- [ ] **Step 1: Add `pool sync` subcommand**

```
dating pool sync <pool>         Sync pool repo + update local suggestions DB
  --force                        Force full re-sync
```

Flow:
1. Find pool in config
2. `git pull` pool repo (or clone if first time)
3. Read `index/` directory from cloned repo
4. Open/create `~/.dating/pools/{name}/suggestions.db`
5. `SyncFromDir` — upsert new/changed .vec files
6. Print: "Synced N new vectors (total M)"

- [ ] **Step 2: Write test**

Test with a temp dir containing .vec files — verify sync populates DB.

- [ ] **Step 3: Run tests + commit**

---

### Task 10: `discover` Command

**Files:**
- Create: `internal/cli/discover_cmd.go`

- [ ] **Step 1: Add `discover` command**

```
dating discover <pool>          Show suggestions
  --limit <N>                    Max results (default: 20)
  --sync                         Sync before discovering
```

Flow:
1. If `--sync`, run pool sync first
2. Open `suggestions.db`
3. Load all vectors
4. Find own vector by own match_hash (from pool config)
5. `RankSuggestions` → top N
6. Print ranked list: `match_hash  score`

- [ ] **Step 2: Write test**

Test with pre-populated suggestions.db — verify output format, limit, excludes self.

- [ ] **Step 3: Run tests + commit**

---

### Task 11: Update Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add indexer target**

```makefile
build: cli relay indexer

indexer:
	go build -o bin/indexer ./cmd/indexer
```

- [ ] **Step 2: Commit**

---

### Task 12: Update Action Template

**Files:**
- Modify: `templates/actions/pool-register.yml`

- [ ] **Step 1: Add indexer step after regcrypt**

```yaml
- name: Download indexer
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/indexer-linux-amd64 -o indexer
    chmod +x indexer

- name: Index user vector
  env:
    INDEXER_WEIGHTS: ${{ secrets.INDEXER_WEIGHTS }}
    OPERATOR_PRIVATE_KEY: ${{ secrets.OPERATOR_PRIVATE_KEY }}
  run: |
    ./indexer \
      --pool-json pool.json \
      --weights "$INDEXER_WEIGHTS" \
      --operator-key "$OPERATOR_PRIVATE_KEY" \
      --bin-file "users/${BIN_HASH}.bin" \
      --match-hash "$MATCH_HASH" \
      --output-dir index/
    git add "index/${MATCH_HASH}.vec"
```

Note: `MATCH_HASH` needs to be output by regcrypt. Update regcrypt to output 3 lines: bin_hash, match_hash, encrypted_blob.

- [ ] **Step 2: Update regcrypt to output match_hash**

Modify `cmd/regcrypt/main.go` output to 3 lines:
```
Line 1: bin_hash
Line 2: match_hash
Line 3: base64 encrypted blob
```

- [ ] **Step 3: Update Action to capture match_hash**

```bash
BIN_HASH=$(echo "$OUTPUT" | sed -n '1p')
MATCH_HASH=$(echo "$OUTPUT" | sed -n '2p')
ENCRYPTED_BLOB=$(echo "$OUTPUT" | sed -n '3p')
```

- [ ] **Step 4: Commit**

---

### Task 13: Update test pool (dating-test-pool)

**Files:**
- Modify: `/Users/vutran/Works/terminal-dating/dating-test-pool/pool.json`
- Modify: `/Users/vutran/Works/terminal-dating/dating-test-pool/.github/workflows/register.yml`

- [ ] **Step 1: Add schema to pool.json**

```json
{
  "schema": {
    "version": 1,
    "fields": [
      {"name": "gender", "type": "enum", "values": ["male", "female", "non-binary"]},
      {"name": "gender_target", "type": "multi", "values": ["male", "female", "non-binary"]},
      {"name": "age", "type": "range", "min": 18, "max": 80},
      {"name": "intent", "type": "multi", "values": ["dating", "friends"]},
      {"name": "interests", "type": "multi", "values": ["hiking", "coding", "music", "travel", "food"]}
    ]
  }
}
```

- [ ] **Step 2: Add INDEXER_WEIGHTS secret to dating-test-pool**

User must manually add this secret.

- [ ] **Step 3: Update workflow to include indexer step**

- [ ] **Step 4: Commit + push dating-test-pool**

---

### Task 14: E2E Test

- [ ] **Step 1: Run full test suite**

Run: `make test`

- [ ] **Step 2: Build all binaries**

Run: `make build`

- [ ] **Step 3: Manual E2E — indexer**

```bash
# Create test .bin file with known profile
# Run indexer
# Verify .vec file size = dimensions × 4 bytes
# Verify vector values match expected encoding
```

- [ ] **Step 4: Manual E2E — full flow**

```bash
# 1. Create pool.json with schema
# 2. Create profile matching schema
# 3. Pack profile as .bin
# 4. Run indexer → produces .vec
# 5. Create suggestions.db from .vec files
# 6. Run discover → verify ranked output
```

- [ ] **Step 5: Manual E2E — real Action**

Create a registration issue in dating-test-pool with a schema-valid profile. Verify:
- Action runs regcrypt + indexer
- .vec file committed to index/
- CLI syncs and discovers

- [ ] **Step 6: Document manual test procedure**

Write `docs/testing/discovery-e2e.md` with step-by-step manual test procedure.
