# Index SQLite Migration

## Goal

Migrate from msgpack (`index.pack` / `suggestions.pack`) to SQLite (`index.db` / `pool.db`). The indexer produces `index.db`, uploads as release asset. The client downloads and merges into local `pool.db` via `ATTACH DATABASE`.

## Files

```
Indexer output (release asset):
  index.db              — profiles table only

Client local:
  ~/.dating/pool.db     — profiles + scores + seen (replaces suggestions.pack)
  ~/.dating/conversations.db  — messages, peer_keys (unchanged)
```

## Schema

### index.db (produced by indexer, uploaded as release asset)

```sql
CREATE TABLE profiles (
    match_hash   TEXT PRIMARY KEY,
    filters      TEXT,              -- JSON: encoded filter values
    vector       BLOB,              -- float32 array (similarity vector)
    display_name TEXT,
    about        TEXT,
    bio          TEXT,
    updated_at   INTEGER
);
```

### pool.db (client local, replaces suggestions.pack)

```sql
-- Synced from index.db
CREATE TABLE profiles (
    match_hash   TEXT PRIMARY KEY,
    filters      TEXT,
    vector       BLOB,
    display_name TEXT,
    about        TEXT,
    bio          TEXT,
    updated_at   INTEGER
);

-- Computed locally from user's match preferences
CREATE TABLE scores (
    match_hash   TEXT PRIMARY KEY,
    total_score  REAL,
    field_scores TEXT,             -- JSON: {"age": 0.8, "interests": 0.6}
    passed       INTEGER,         -- 1 if all min thresholds met
    ranked_at    INTEGER
);

-- Discovery state
CREATE TABLE seen (
    match_hash TEXT PRIMARY KEY,
    seen_at    INTEGER,
    action     TEXT               -- skip, like
);
```

## Sync (ATTACH DATABASE)

Client downloads `index.db` from release, merges into `pool.db`:

```sql
ATTACH DATABASE 'index.db' AS remote;

-- Upsert new/updated profiles
INSERT OR REPLACE INTO profiles (match_hash, filters, vector, display_name, about, bio, updated_at)
SELECT match_hash, filters, vector, display_name, about, bio, updated_at FROM remote.profiles;

-- Remove profiles deleted from index
DELETE FROM profiles WHERE match_hash NOT IN (SELECT match_hash FROM remote.profiles);

-- Invalidate scores for changed profiles
DELETE FROM scores WHERE match_hash NOT IN (SELECT match_hash FROM profiles);

DETACH DATABASE remote;
```

## Indexer Changes

`action-tool index` currently:
1. Reads pool.json + .bin files
2. Decrypts profiles, extracts filters + vectors
3. Writes `index.pack` (msgpack `IndexRecord` array)

New:
1. Same decrypt + extract
2. Opens `index.db` (SQLite)
3. Inserts each profile into `profiles` table
4. If `--upload`: uploads `index.db` as release asset

## Client Changes

### PoolDB (shared package, used by indexer + client)

```go
// internal/pooldb/pooldb.go

type PoolDB struct {
    db *sql.DB
}

func OpenPoolDB(path string) (*PoolDB, error)
func (p *PoolDB) Close() error
func (p *PoolDB) SyncFromIndex(indexDBPath string) (int, error)  // ATTACH + merge
func (p *PoolDB) ListProfiles() ([]Profile, error)
func (p *PoolDB) SaveScore(matchHash string, score float64, fieldScores map[string]float64, passed bool) error
func (p *PoolDB) MarkSeen(matchHash, action string) error
func (p *PoolDB) GetSeen() (map[string]bool, error)
func (p *PoolDB) GetProfile(matchHash string) (*Profile, error)

type Profile struct {
    MatchHash   string
    Filters     FilterValues   // parsed from JSON
    Vector      []float32      // decoded from BLOB
    DisplayName string
    About       string
    Bio         string
}
```

### Ranking

`RankSuggestions` currently takes `[]Record` from the pack. Now takes profiles from `PoolDB`:

```go
profiles, _ := poolDB.ListProfiles()
seen, _ := poolDB.GetSeen()
ranked := RankSuggestions(schema, me, profiles, seen, limit)
```

Same ranking logic, different data source.

### Sync Flow

```
Current:
  git clone repo → read index.pack from working tree → load into Pack → save suggestions.pack

New:
  DownloadReleaseAsset("index-latest", "index.db", tempPath)
  poolDB.SyncFromIndex(tempPath)  // ATTACH + merge
```

## What Gets Deleted

- `internal/cli/suggestions/db.go` — Pack struct, SyncFromIndexPack, SyncFromRecDir, Load, Save
- `suggestions.pack` files
- `index.pack` references in client code
- msgpack dependency for index (keep for regcrypt payload if still used)

## What Stays

- `internal/cli/suggestions/rank.go` — ranking logic (adapted to new Profile type)
- `internal/cli/suggestions/rank_test.go` — tests adapted
- `internal/github/vecfile.go` — IndexRecord (used by indexer to build profiles)
- `internal/github/vector.go` — encoding/filtering logic

## Files Changed

| File | Change |
|------|--------|
| Create: `internal/pooldb/pooldb.go` | PoolDB (SQLite) |
| Create: `internal/pooldb/pooldb_test.go` | Tests |
| Modify: `cmd/action-tool/index.go` | Write index.db instead of index.pack |
| Modify: `internal/cli/suggestions/rank.go` | Accept Profile from PoolDB |
| Modify: `internal/cli/sync.go` | Use PoolDB.SyncFromIndex |
| Modify: `internal/cli/tui/screens/discover.go` | Use PoolDB |
| Delete: `internal/cli/suggestions/db.go` | Old Pack (msgpack) |
| Modify: `templates/actions/pool-indexer.yml` | Output index.db |
