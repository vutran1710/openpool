# Index SQLite Migration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace msgpack index.pack/suggestions.pack with SQLite index.db/pool.db. Indexer writes SQLite, client syncs via ATTACH DATABASE.

**Architecture:** New `PoolDB` replaces `Pack`. Indexer opens SQLite, inserts profiles. Client downloads index.db, merges into pool.db. Ranking reads from PoolDB instead of Pack.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), existing ranking/vector code

---

### Task 1: Create PoolDB

**Files:**
- Create: `internal/pooldb.go`
- Create: `internal/pooldb_test.go`

- [ ] **Step 1: Write tests**

```go
func TestPoolDB_SyncFromIndex(t *testing.T)
func TestPoolDB_ListProfiles(t *testing.T)
func TestPoolDB_MarkSeen(t *testing.T)
func TestPoolDB_GetSeen(t *testing.T)
func TestPoolDB_SyncRemovesDeleted(t *testing.T)
func TestPoolDB_SyncInvalidatesScores(t *testing.T)
```

Test `SyncFromIndex` by creating a temp `index.db`, populating it, then syncing into `pool.db`.

- [ ] **Step 2: Implement PoolDB**

```go
type PoolDB struct { db *sql.DB }

func OpenPoolDB(path string) (*PoolDB, error)
// Creates tables if not exist: profiles, scores, seen

func (p *PoolDB) SyncFromIndex(indexDBPath string) (int, error)
// ATTACH DATABASE indexDBPath AS remote
// INSERT OR REPLACE into profiles from remote.profiles
// DELETE profiles not in remote
// DELETE invalidated scores
// DETACH

func (p *PoolDB) ListProfiles() ([]Profile, error)
func (p *PoolDB) GetProfile(matchHash string) (*Profile, error)
func (p *PoolDB) SaveScore(matchHash string, totalScore float64, fieldScores string, passed bool) error
func (p *PoolDB) MarkSeen(matchHash, action string) error
func (p *PoolDB) GetSeen() (map[string]bool, error)
func (p *PoolDB) Close() error
```

Profile type:
```go
type Profile struct {
    MatchHash   string
    Filters     string     // raw JSON (caller parses)
    Vector      []byte     // raw bytes (caller decodes float32s)
    DisplayName string
    About       string
    Bio         string
}
```

Keep `Filters` and `Vector` as raw bytes/strings — the caller (ranking) handles decoding. Single responsibility.

- [ ] **Step 3: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v ./internal/cli/suggestions/ -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/pooldb.go internal/pooldb_test.go
git commit -m "feat: PoolDB — SQLite replacement for suggestions.pack"
```

---

### Task 2: Update indexer to write index.db

**Files:**
- Modify: `cmd/action-tool/index.go`

- [ ] **Step 1: Read current index.go**

Understand how it currently writes index.pack (msgpack). The output is controlled by `--output` flag.

- [ ] **Step 2: Change output to SQLite**

Instead of msgpack encoding to a file, open SQLite and insert:

```go
// Open output as SQLite
db, _ := sql.Open("sqlite", outputPath)
db.Exec(`CREATE TABLE IF NOT EXISTS profiles (
    match_hash TEXT PRIMARY KEY,
    filters TEXT, vector BLOB,
    display_name TEXT, about TEXT, bio TEXT,
    updated_at INTEGER
)`)

// For each profile:
db.Exec("INSERT OR REPLACE INTO profiles VALUES (?, ?, ?, ?, ?, ?, ?)",
    record.MatchHash, filtersJSON, vectorBytes,
    record.DisplayName, record.About, record.Bio,
    time.Now().Unix())

db.Close()
```

Change default output from `index.pack` to `index.db`.

Add import: `"database/sql"`, `_ "modernc.org/sqlite"`

- [ ] **Step 3: Update --upload to upload index.db**

The `--upload` flag already works — just change the default output filename.

- [ ] **Step 4: Verify build**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 5: Commit**

```bash
git add cmd/action-tool/index.go
git commit -m "feat: indexer writes index.db (SQLite) instead of index.pack"
```

---

### Task 3: Update ranking to use PoolDB

**Files:**
- Modify: `internal/cli/suggestions/rank.go`

- [ ] **Step 1: Read current rank.go**

Understand `RankSuggestions` signature — it takes `schema, me Record, records []Record, seen map[string]bool, limit int`.

- [ ] **Step 2: Adapt to PoolDB Profile type**

The `Record` type from the old Pack and the new `Profile` type from PoolDB need to be compatible. Either:
- Change `RankSuggestions` to accept `[]Profile`
- Or convert `Profile` → `Record` at the call site

Simplest: keep `RankSuggestions` working with a common type. The `Profile` from PoolDB has raw `Filters` (JSON string) and `Vector` (bytes). The ranking needs parsed `FilterValues` and `[]float32`. Add conversion helpers:

```go
func ProfileToRecord(p Profile) (Record, error) {
    var filters gh.FilterValues
    json.Unmarshal([]byte(p.Filters), &filters)
    vector := decodeFloat32s(p.Vector)
    return Record{
        MatchHash: p.MatchHash,
        Filters: filters,
        Vector: vector,
        DisplayName: p.DisplayName,
        About: p.About,
        Bio: p.Bio,
    }, nil
}
```

- [ ] **Step 3: Update tests**

Run: `DATING_HOME=$(mktemp -d) go test -v ./internal/cli/suggestions/ -count=1`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/suggestions/rank.go
git commit -m "refactor: ranking works with PoolDB profiles"
```

---

### Task 4: Update client sync + discover

**Files:**
- Modify: `internal/cli/sync.go`
- Modify: `internal/cli/tui/screens/discover.go`

- [ ] **Step 1: Update sync.go**

Replace:
```go
// Old: load index.pack → Pack.SyncFromIndexPack → save suggestions.pack
pack, _ := suggestions.Load(packPath)
pack.SyncFromIndexPack(indexPackPath)
pack.Save(packPath)
```

With:
```go
// New: download index.db → PoolDB.SyncFromIndex
poolDBPath := filepath.Join(config.Dir(), "pool.db")
poolDB, _ := suggestions.OpenPoolDB(poolDBPath)
defer poolDB.Close()

indexPath := filepath.Join(config.Dir(), "pools", poolName, "index.db")
github.DownloadReleaseAsset(pool.Repo, "index-latest", "index.db", indexPath)
poolDB.SyncFromIndex(indexPath)
```

- [ ] **Step 2: Update discover.go**

Same pattern — replace Pack usage with PoolDB. The discover screen loads profiles from PoolDB, ranks them, marks seen.

Key changes:
- `LoadDiscoverCmd` → open PoolDB, list profiles, rank
- `backgroundSync` → download index.db, sync, re-rank
- `DiscoverScreen` stores PoolDB reference or ranked results

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/sync.go internal/cli/tui/screens/discover.go
git commit -m "refactor: sync + discover use PoolDB instead of Pack"
```

---

### Task 5: Delete old Pack code

**Files:**
- Delete: `internal/cli/suggestions/db.go` (old Pack)

- [ ] **Step 1: Check for remaining references**

```bash
grep -rn "suggestions.Load\|suggestions.Pack\|SyncFromIndexPack\|SyncFromRecDir\|suggestions\.pack" --include='*.go'
```

Fix any remaining references.

- [ ] **Step 2: Delete db.go**

```bash
rm internal/cli/suggestions/db.go
```

- [ ] **Step 3: Verify build + tests**

Run: `go build ./... && DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 4: Clean up go.mod**

Run `go mod tidy` — msgpack dependency may no longer be needed (check if regcrypt payload still uses it).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "delete: remove old Pack (msgpack), go mod tidy"
```

---

### Task 6: Update Action template + publish

- [ ] **Step 1: Update pool-indexer.yml output**

Change `--output index.pack` to `--output index.db`:

```yaml
run: |
  ./action-tool index \
    --pool-json pool.json \
    --operator-key "$OPERATOR_PRIVATE_KEY" \
    --users-dir users/ \
    --output index.db \
    --upload
```

- [ ] **Step 2: Commit**

```bash
git add templates/actions/pool-indexer.yml
git commit -m "refactor: indexer outputs index.db"
```

---

### Task 7: Full test suite + E2E

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 2: Build + publish action-tool**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/action-tool-linux-amd64 ./cmd/action-tool/
gh release upload action-tool-v1.0.0 /tmp/action-tool-linux-amd64 --clobber --repo vutran1710/regcrypt
```

- [ ] **Step 3: Deploy + trigger indexer**

Deploy updated Action to test pool, trigger manually, verify `index.db` appears as release asset.

- [ ] **Step 4: Verify client sync**

Run `dating pool sync` or trigger discover — should download `index.db` and populate `pool.db`.

- [ ] **Step 5: Commit**

```bash
git commit -m "test: E2E verified SQLite index migration"
```
