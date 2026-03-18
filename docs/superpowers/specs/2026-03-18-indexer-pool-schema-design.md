# Pool Schema + Indexer + Client-Side Suggestions

**Status**: Spec

---

## Summary

Each pool defines a schema in `pool.json` that describes the profile fields (interests, gender, age, etc.). The indexer — a Go binary run inside the registration GitHub Action — reads each profile, applies operator-defined weights, and produces a fixed-dimension vector. These vectors are stored in a flat binary index file committed to the pool repo.

The client reads the index, computes cosine similarity against all users, groups results into tiers, shuffles within tiers, and presents suggestions. No relay needed for discovery — just the git repo.

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

Three types. No special cases, no filter flags. Every field is a vector component.

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

The operator defines weights per field. This is NOT in `pool.json` — it's a private config stored as a GitHub Actions secret or a private file.

```json
{
  "weights": {
    "gender": 10.0,
    "gender_target": 10.0,
    "age": 5.0,
    "intent": 3.0,
    "interests": 1.0
  }
}
```

The indexer multiplies each field's vector segment by its weight before writing to the index. Gender dimensions get 10x amplification — cosine similarity naturally punishes mismatches without any filtering logic.

### What Weights Control

- **High weight** (gender: 10.0) → strong match signal, mismatches tank similarity
- **Low weight** (interests: 1.0) → nice-to-have, doesn't dominate
- **Zero weight** (0.0) → field exists in schema (for profile display) but doesn't affect matching

The client never sees the weights. It just compares pre-weighted vectors.

### Weight Distribution

The weight applies uniformly to all dimensions of a field. For a `multi` field with 10 values and weight 2.0, all 10 dimensions are multiplied by 2.0.

---

## 3. Indexer Binary

`cmd/indexer/` — standalone Go binary, run by the registration GitHub Action. Same distribution model as `regcrypt` (pre-built, published to a public release repo).

### What It Does

1. Reads `pool.json` schema
2. Reads operator weights (from env var or file)
3. For each `.bin` file in `users/`:
   - Decrypt profile with operator private key
   - Extract field values per schema
   - Encode to vector (one-hot, multi-hot, normalized)
   - Multiply by weights
4. Write all vectors to `index.db`

### When It Runs

- **On registration**: Action runs indexer after committing the new `.bin` file
- **On schema change**: Full re-index (version bump detected)

### CLI

```
Usage: indexer \
  --pool-json <path>           pool.json with schema
  --weights <json_string>      operator weights (or INDEXER_WEIGHTS env var)
  --operator-key <hex>         operator private key for .bin decryption
  --users-dir <path>           path to users/ directory
  --output <path>              output index.db path
```

---

## 4. Index File Format (index.db)

Flat binary file. Fixed-stride records. No headers, no framing — just concatenated records.

### Record Layout

```
[match_hash 8B][vector D×f32]
```

- `match_hash`: first 8 bytes of the 16-hex-char match_hash (compact binary form)
- `vector`: D float32 values (D = total dimensions from schema)

The index uses `match_hash`, not `bin_hash`. `bin_hash` is for relay routing. `match_hash` is the matching identity — the client compares vectors by `match_hash` and only resolves to `bin_hash` when initiating a relay connection.

### Record Size

`8 + D × 4` bytes per user.

Examples:
- 20 dimensions: `8 + 80 = 88 bytes` per user
- 10K users: 880 KB
- 50K users: 4.4 MB

### File Properties

- **Append-friendly**: new registrations append a record
- **Git-friendly**: binary, but small and grows linearly
- **No index structure**: brute-force scan is fast at this scale (sub-millisecond for 50K records)
- **Deterministic order**: sorted by match_hash for reproducible diffs

### Match Hash Lookup

To find a specific user's vector, binary search on sorted match_hash (8 bytes). Or scan — it's fast enough.

### Schema Version Check

The client computes expected record size from the schema: `8 + D × 4`. If the file's size isn't divisible by this, the schema version has changed and the index is stale — re-clone the repo.

---

## 5. Profile Creation (Schema-Driven)

`dating profile create <pool>` must read `pool.json` to scaffold the profile with the correct fields.

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

4. For `enum` and `multi` fields, include valid values as a comment or separate `profile.schema.json` for editor autocompletion

### Validation

`pool join` validates the profile against the schema before submitting:
- `enum` fields: value must be in `values` list
- `multi` fields: all values must be in `values` list
- `range` fields: value must be within `[min, max]`
- All schema fields must be present

---

## 6. Client-Side Suggestions

The client reads `index.db` and the user's own vector to produce suggestions.

### Flow

1. Read `index.db` from cloned pool repo
2. Compute user's own vector from their profile (same encoding, no weights — weights are baked into the index)
3. Actually: user's vector IS in the index (they're registered). Read it.
4. Compute cosine similarity against all other vectors
5. Group into tiers (top 10%, next 10%, ...)
6. Shuffle within each tier
7. Present in order

### Wait — User's Vector Has Weights Baked In

The user's vector in the index already has operator weights applied. Cosine similarity between two weighted vectors naturally reflects the operator's priorities. No additional weighting needed at query time.

### Tier-Based Shuffled Ranking

```
all_users = read index.db
scores = [(user, cosine(my_vector, user.vector)) for user in all_users if user != me]
sort by score descending
tier_size = len(scores) / 10
for each tier (0..9):
    tier_users = scores[tier * tier_size : (tier+1) * tier_size]
    shuffle(tier_users)
    append to result
show result
```

No threshold. No fixed N. Just a ranked, shuffled list. User scrolls until bored.

---

## 7. Action Pipeline (Updated)

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
Action: indexer
    → decrypt .bin → extract fields → encode → apply weights
    → append to index.db (or full rebuild on schema version change)
    → commit updated index.db
    ↓
Issue closed
```

---

## 8. File Summary

| File | Location | Purpose |
|------|----------|---------|
| `pool.json` | Pool repo root | Schema + pool metadata (public) |
| `users/{bin_hash}.bin` | Pool repo | Encrypted profiles |
| `index.db` | Pool repo root | Flat binary vector index (public) |
| Operator weights | Actions secret / private | Weight config (private) |
| `cmd/indexer/` | dating-dev repo | Indexer binary source |
| `templates/actions/pool-register.yml` | dating-dev repo | Action template (includes indexer step) |
