# Pool Schema + Indexer + Client-Side Discovery

**Status**: Spec

---

## Summary

Each pool defines a schema in `pool.json` that describes profile fields (interests, gender, age, etc.). The indexer — a Go binary run inside the registration GitHub Action — reads each profile, applies operator-defined weights, and produces a fixed-dimension vector stored as one `.vec` file per user in the pool repo. No shared index file — race-free.

The client syncs the pool repo, builds a local SQLite DB from `.vec` files, computes cosine similarity, and presents tier-shuffled suggestions. No relay needed for discovery.

All CLI commands are non-interactive with flag overrides for full scriptability and testability.

---

## 1. Pool Schema (pool.json)

The schema defines what profile fields exist and how they encode into vectors. Lives in `pool.json` alongside existing pool metadata.

```json
{
  "name": "berlin-singles",
  "operator_public_key": "abc...",
  "relay_url": "wss://relay.berlin-singles.dev",
  "schema": {
    "version": 1,
    "fields": [
      {"name": "gender", "type": "enum", "values": ["male", "female", "non-binary"]},
      {"name": "gender_target", "type": "multi", "values": ["male", "female", "non-binary"]},
      {"name": "age", "type": "range", "min": 18, "max": 80},
      {"name": "intent", "type": "multi", "values": ["dating", "friends", "networking"]},
      {"name": "interests", "type": "multi", "values": ["hiking", "cooking", "gaming", "reading", "music", "travel", "fitness", "art", "tech", "photography"]}
    ]
  }
}
```

### Field Types

| Type | Profile Value | Vector Encoding | Dimensions |
|------|--------------|-----------------|------------|
| `enum` | `"male"` | one-hot: `[0, 1, 0]` | len(values) |
| `multi` | `["hiking", "music"]` | multi-hot: `[1, 0, 0, 0, 1, ...]` | len(values) |
| `range` | `28` | normalized: `(val - min) / (max - min)` | 1 |

Three types. No special cases. Every field is a vector component.

### Vector Dimensions

Total dimensions = sum of dimensions across all fields.

For the example above:
- gender: 3
- gender_target: 3
- age: 1
- intent: 3
- interests: 10
- **Total: 20 dimensions**

### Schema Version

When the operator changes the schema (adds an interest, removes a field), they bump `schema.version`. The indexer detects a version mismatch and triggers a full re-index of all users.

---

## 2. Operator Weights (Private)

The operator defines weights per field. NOT in `pool.json` — stored as a GitHub Actions secret (`INDEXER_WEIGHTS`).

```json
{
  "gender": 10.0,
  "gender_target": 10.0,
  "age": 5.0,
  "intent": 3.0,
  "interests": 1.0
}
```

The indexer multiplies each field's vector segment by its weight. Gender gets 10x amplification — cosine similarity naturally punishes mismatches without filtering logic.

- **High weight** → strong match signal, mismatches tank similarity
- **Low weight** → nice-to-have, doesn't dominate
- **Zero weight** → field exists in schema (for display) but doesn't affect matching

The client never sees the weights. It just compares pre-weighted vectors.

---

## 3. Indexer Binary

`cmd/indexer/` — standalone Go binary, run by the registration GitHub Action. Same distribution model as `regcrypt` (pre-built, published to public release repo).

### What It Does

For a single user (on registration):

1. Read `pool.json` schema
2. Read operator weights (from `INDEXER_WEIGHTS` env var)
3. Decrypt the user's `.bin` file (operator private key)
4. Extract field values per schema
5. Encode to vector (one-hot, multi-hot, normalized)
6. Multiply by weights
7. Write `index/{match_hash}.vec` — single binary file

### When It Runs

- **On registration**: indexer processes the new `.bin` file, writes one `.vec` file
- **On schema change**: full re-index — delete all `.vec` files, reprocess all `.bin` files

### CLI

```
Usage: indexer \
  --pool-json <path>           pool.json with schema
  --weights <json_string>      operator weights (or INDEXER_WEIGHTS env var)
  --operator-key <hex>         operator private key for .bin decryption
  --bin-file <path>            single .bin file to index (registration mode)
  --match-hash <hash>          match_hash for the user (output filename)
  --output-dir <path>          directory for .vec files (default: index/)
  --rebuild                    re-index all .bin files (schema change mode)
  --users-dir <path>           path to users/ directory (for --rebuild)
```

### Output: `.vec` File Format

Fixed-size binary file. Just the raw vector — no header, no framing.

```
[D × float32]
```

- D = total dimensions from schema
- 20 dimensions = 80 bytes per file
- Filename is the match_hash: `index/{match_hash}.vec`

---

## 4. Per-User `.vec` Files (Race-Free)

Each user gets their own `.vec` file in the pool repo:

```
index/
  a1b2c3d4e5f67890.vec    ← 80 bytes (20 dims × 4 bytes)
  f0e1d2c3b4a59876.vec
  ...
```

### Why Per-File

Two registrations happening simultaneously write different files — **no race condition**. No shared `index.db` to conflict on.

### Scale

- 10K users × 80 bytes = 800 KB total (plus git overhead for 10K tree entries)
- `git clone --depth 1` handles this fine
- `git pull` only fetches new `.vec` files (delta)

### Schema Version Change

On schema version bump, the Action deletes all `.vec` files and rebuilds from all `.bin` files. This is rare and uses a GitHub Actions concurrency group to prevent races:

```yaml
concurrency:
  group: reindex
  cancel-in-progress: false
```

---

## 5. Local SQLite DB (Client-Side)

The client maintains a local SQLite database per pool for fast discovery queries.

### Location

```
~/.dating/pools/{pool-name}/suggestions.db
```

### Schema

```sql
CREATE TABLE vectors (
  match_hash TEXT PRIMARY KEY,
  vector     BLOB NOT NULL
);
```

### Sync Flow

On `dating pool sync` or `dating discover`:

1. `git pull` the pool repo
2. Detect new/changed `.vec` files in `index/` (compare against DB)
3. `INSERT OR REPLACE` into `suggestions.db`
4. Ready for queries

### Why SQLite

- Incremental updates: `INSERT` new records only (~1.8ms for 50 records)
- Full load into memory: ~5.6ms for 10K records
- Pure Go: `modernc.org/sqlite` (no CGo, no system dependency)
- Persistent: no rebuild on every query
- Only used by CLI, not relay

---

## 6. Profile Creation (Schema-Driven)

`dating profile create <pool>` reads `pool.json` to scaffold the profile with correct fields.

### Flow

1. Fetch/read `pool.json` from pool repo
2. Parse `schema.fields`
3. Scaffold `profile.json` with correct field names, types, and valid values:

```json
{
  "gender": "",
  "gender_target": [],
  "age": 0,
  "intent": [],
  "interests": []
}
```

4. Write a `profile.schema.json` alongside for editor reference:

```json
{
  "gender": {"type": "enum", "values": ["male", "female", "non-binary"]},
  "interests": {"type": "multi", "values": ["hiking", "cooking", ...]}
}
```

### Validation

`pool join` validates the profile against the schema before submitting:
- `enum` fields: value must be in `values` list
- `multi` fields: all values must be in `values` list
- `range` fields: value must be within `[min, max]`
- All schema fields must be present

---

## 7. Client-Side Discovery

### CLI Commands

All non-interactive, fully scriptable:

```
dating discover <pool>                Show suggestions for a pool
  --limit <N>                          Max suggestions to show (default: 20)
  --sync                               Force sync before discovering

dating pool sync <pool>               Sync pool repo + update local SQLite
```

### Discovery Flow

1. Open `suggestions.db`
2. Load all vectors into memory (~5ms for 10K)
3. Find own vector by own match_hash
4. Compute cosine similarity against all other vectors
5. Group into tiers (top 10%, next 10%, ...)
6. Shuffle within each tier
7. Present top N results

### Tier-Based Shuffled Ranking

```
all_users = load all from suggestions.db
my_vector = lookup my match_hash
scores = [(user, cosine(my_vector, user.vector)) for user in all_users if user != me]
sort by score descending
tier_size = len(scores) / 10
for each tier (0..9):
    tier_users = scores[tier * tier_size : (tier+1) * tier_size]
    shuffle(tier_users)
    append to result
show result[:limit]
```

No threshold. No fixed N. Just a ranked, shuffled list.

### Output Format

```
dating discover berlin-singles

  Suggestions for berlin-singles (10050 users)

  1. a1b2c3d4e5f67890  score: 0.94
  2. f0e1d2c3b4a59876  score: 0.91
  3. ...

  Show full profile: dating profile view <match_hash> --pool berlin-singles
```

---

## 8. Action Pipeline (Updated)

```
Issue created (registration)
    ↓
Action: regcrypt
    → compute id_hash → bin_hash → match_hash
    → encrypt {bin_hash, match_hash} to user's pubkey
    → post encrypted comment
    ↓
Action: commit users/{bin_hash}.bin
    ↓
Action: indexer --bin-file users/{bin_hash}.bin --match-hash {match_hash}
    → decrypt .bin → extract fields → encode → apply weights
    → write index/{match_hash}.vec
    → commit
    ↓
Issue closed
```

---

## 9. File Summary

| File | Location | Purpose |
|------|----------|---------|
| `pool.json` | Pool repo root | Schema + pool metadata (public) |
| `users/{bin_hash}.bin` | Pool repo | Encrypted profiles |
| `index/{match_hash}.vec` | Pool repo | Per-user weighted vector (public) |
| `suggestions.db` | Local `~/.dating/pools/{name}/` | Client-side SQLite for discovery queries |
| Operator weights | Actions secret (`INDEXER_WEIGHTS`) | Weight config (private) |
| `cmd/indexer/` | dating-dev repo | Indexer binary source |
| `templates/actions/pool-register.yml` | dating-dev repo | Action template (includes indexer step) |

---

## 10. Implementation Order

1. **Pool schema parsing** — read `pool.json` schema, validate fields
2. **Vector encoding** — enum/multi/range → vector with weights
3. **Indexer binary** (`cmd/indexer/`) — decrypt .bin, encode, write .vec
4. **Profile create** — schema-driven scaffolding + validation
5. **Local SQLite** — sync .vec files into suggestions.db
6. **Discover command** — cosine similarity, tier shuffle, display
7. **Updated Action template** — add indexer step after regcrypt
8. **E2E test** — register user → indexer → sync → discover
