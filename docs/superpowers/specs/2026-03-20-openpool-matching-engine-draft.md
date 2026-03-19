# OpenPool Generic Matching Engine — DRAFT

> **Status**: Draft — pool schema shape needs rethinking. Scoring engine, preferences, storage, and TUI rendering are approved.

## Context

Renaming dating-dev → openpool. The matching/filtering/ranking engine needs to be generic enough for dating, job boards, networking, or any profile-match-chat domain. The pool schema defines the vocabulary, the engine is reusable.

---

## What's Decided

### 1. Everything is a Score

Every field produces a score [0, 1]. "Filters" are just fields with `min: 1.0` (must score perfectly). "Preferences" are fields with `min: 0.0` and a weight. One unified model.

```
field scores 1.0 → perfect match
field scores 0.7 → partial match
field scores 0.0 → no match

min: 1.0 → hard filter (reject if < 1.0)
min: 0.5 → soft filter (reject if < 0.5)
min: 0.0 → pure scoring (never reject, just rank lower)
```

### 2. Five Scoring Primitives

| Primitive | Input | Output |
|-----------|-------|--------|
| `exact` | enum, multi | 1.0 if match/overlap, 0.0 if not |
| `complementary` | enum + target field | 1.0 if my value in their target set |
| `proximity` | range | linear decay: 1.0 at diff=0, 0.0 at diff=tolerance |
| `similarity` | multi (vector) | cosine similarity [0, 1] |
| `overlap` | multi | 1.0 if shared >= N, scales down below N |

Score function is inferred from matcher syntax in the user's preference or pool's matching rules.

### 3. Three Matching Modes

Determined by what the pool defines:

- **Pool-controlled** (`matching.mode: pool`) — operator defines all matching rules. Users just fill in attributes.
- **User-controlled** (`matching.mode: user` or no `matching` block) — users define their own match preferences in their profile.
- **Hybrid** (`matching.mode: hybrid`) — pool defines defaults, users can adjust unlocked rules. `locked: true` on a rule prevents user override.

### 4. User Match Preferences

Users write match rules against pool attributes:

```yaml
# User's match preferences
match:
  gender: male                        # exact value
  gender: [male, non-binary]          # any of these
  age: { within: 5 }                  # ±5 from my value
  age: { min: 25, max: 35 }           # absolute range
  interests: { overlap: 2 }           # at least 2 shared
  interests: { includes: hiking }     # must include specific value
  looking_for: dating                 # exact match

# Optional weights
match:
  gender: { value: male, weight: 10 }
  age: { within: 5, weight: 3 }
  interests: { overlap: 2, weight: 1 }
```

No weight = default 1.0. Final score = weighted average of all field scores.

### 5. Three Ranking Strategies

Pool-configurable in `pool.yaml`:

```yaml
ranking:
  strategy: tiered_shuffle       # dating — fairness
  tiers: [0.05, 0.20, 0.50]

ranking:
  strategy: top_score            # job board — best first

ranking:
  strategy: weighted_random      # networking — biased random
  bias: 0.7
```

### 6. Seen Decay & Cooldown

Seen entries have timestamps. After cooldown, profiles re-enter the candidate pool:

```yaml
# pool.yaml
ranking:
  seen_cooldown: 7d    # profiles reappear after 7 days
```

Default = permanent (never reappear). Seen map changes from `map[string]bool` to `map[string]time.Time`.

Profile update → re-registration → new `.bin` → indexer rebuilds. Seen list untouched.
Preference update → re-score only. Seen list untouched (cooldown still applies).

### 7. Dual Validation

- **Client-side**: Validates profile against schema before submitting (instant UX feedback)
- **Action-side**: Validates during registration (trust boundary, rejects malformed profiles)

### 8. SQLite Storage

#### Indexer produces `index.db`:

```sql
CREATE TABLE profiles (
    match_hash TEXT PRIMARY KEY,
    attributes TEXT NOT NULL,      -- JSON: all attribute values
    vector     BLOB,               -- pre-computed similarity vector
    updated_at INTEGER
);
```

#### Client maintains local `suggestions.db`:

```sql
CREATE TABLE profiles (
    match_hash TEXT PRIMARY KEY,
    attributes TEXT NOT NULL,
    vector     BLOB,
    updated_at INTEGER
);

CREATE TABLE scores (
    match_hash TEXT PRIMARY KEY,
    total_score REAL,
    field_scores TEXT,             -- JSON: {"age": 0.8, "interests": 0.6}
    passed INTEGER,               -- 1 if all min thresholds met
    ranked_at INTEGER
);

CREATE TABLE seen (
    match_hash TEXT PRIMARY KEY,
    seen_at INTEGER,
    action TEXT                    -- skip, like, pass
);
```

#### Sync via ATTACH DATABASE:

```sql
ATTACH DATABASE 'index.db' AS remote;

-- Upsert new/updated profiles
INSERT OR REPLACE INTO profiles (match_hash, attributes, vector, updated_at)
SELECT match_hash, attributes, vector, updated_at FROM remote.profiles;

-- Remove deleted profiles
DELETE FROM profiles WHERE match_hash NOT IN (SELECT match_hash FROM remote.profiles);

-- Invalidate scores for changed profiles
DELETE FROM scores WHERE match_hash NOT IN (SELECT match_hash FROM profiles)
    OR match_hash IN (
        SELECT r.match_hash FROM remote.profiles r
        JOIN profiles p ON r.match_hash = p.match_hash
        WHERE r.updated_at > p.updated_at
    );

DETACH DATABASE remote;
```

### 9. Adaptive TUI Rendering

TUI generates forms dynamically from pool schema:

| Attribute type | TUI component |
|---------------|---------------|
| `enum` (single) | Radio select |
| `enum` + target | Radio + multi-select for target |
| `multi` | Checkbox group |
| `range` | Number input with min/max |
| `text` | Text input |

Match preference editors:

| Matcher | TUI component |
|---------|---------------|
| exact value | Same as attribute input |
| `within: N` | Slider or number input |
| `min/max` | Two number inputs |
| `overlap: N` | Number input |
| weight | Small number next to each rule |

In `pool` mode → no preference section rendered.
In `hybrid` mode → locked fields shown as dimmed/read-only.

---

## What Needs Rethinking

### Pool Schema Shape

The current draft uses `attributes` as a root key with typed fields:

```yaml
name: <3 dating
description: terminal-native dating

ranking:
  strategy: tiered_shuffle
  tiers: [0.05, 0.20, 0.50]
  seen_cooldown: 7d

attributes:
  display_name:
    type: text
  gender:
    type: enum
    values: [male, female, non-binary]
  age:
    type: range
    min: 18
    max: 100
  interests:
    type: enum
    values: [hiking, coding, music, travel, food]

# Optional — if present, pool controls matching
matching:
  mode: pool
  rules:
    gender: { complementary: gender_target }
    age: { within: 5, weight: 2 }
    interests: { overlap: 1 }
```

**Open questions:**
- Overall shape doesn't feel friendly/intuitive enough
- How should roles (employer/candidate) fit in?
- Should `matching` be part of the schema or separate?
- Is the attribute type system (`enum`, `range`, `text`, `multi`) the right abstraction?

This is the main blocker before the spec can be finalized.

---

## Migration from Current System

| Current | OpenPool |
|---------|----------|
| `pool.json` | `pool.yaml` |
| `index.pack` (msgpack) | `index.db` (SQLite) |
| `suggestions.pack` | `suggestions.db` |
| 4 hardcoded match modes | 5 composable scoring primitives |
| Operator-only weights | User-configurable preferences (or pool-controlled) |
| `FilterValues` + vector | Unified scoring [0, 1] per field |
| `map[string]bool` seen | `map[string]time.Time` with cooldown |
