# OpenPool Generic Matching Engine — DRAFT

> **Status**: Draft — iterating on pool schema shape. Scoring engine, preferences, storage, and TUI rendering are approved.

## Context

Renaming dating-dev → openpool. The matching/filtering/ranking engine needs to be generic enough for dating, job boards, networking, or any profile-match-chat domain. The pool schema defines the vocabulary, the engine is reusable.

---

## Pool Repo File Structure

Split config into separate files by concern:

```
pool-repo/
  pool.json        # metadata (machine config, rarely changes)
  schema.yaml      # attributes (data contract, human-authored)
  matcher.yaml     # matching rules (operator-controlled scoring, optional)
  users/           # .bin files (encrypted profiles)
  matches/         # match files
  index.db         # built by indexer cron
```

**Why separate files:**
- `pool.json` changes rarely (relay URL, operator key) — machine config
- `schema.yaml` changes trigger re-registration (profile structure changed)
- `matcher.yaml` changes only trigger re-indexing (scoring weights changed)
- Different change cadences → different files

### pool.json — Metadata

```json
{
  "name": "<3 dating",
  "description": "terminal-native dating for developers",
  "version": 2,
  "operator_public_key": "c251e2cf...",
  "relay_url": "wss://relay.example.com",
  "ranking": {
    "strategy": "tiered_shuffle",
    "tiers": [0.05, 0.20, 0.50],
    "seen_cooldown": "7d"
  }
}
```

### schema.yaml — Attributes

Each attribute has a type, optionality, and **visibility**:

```yaml
# schema.yaml — dating pool
attributes:
  display_name:
    type: text
    required: true
    visibility: public

  gender:
    type: enum
    values: [male, female, non-binary]
    required: true
    visibility: public

  age:
    type: range
    min: 18
    max: 100
    required: true
    visibility: public

  interests:
    type: multi
    values: [hiking, coding, music, travel, food]
    required: true
    visibility: public

  about:
    type: text
    required: false
    visibility: public

  phone:
    type: text
    required: true
    visibility: private      # only revealed after match

  social_links:
    type: text
    required: false
    visibility: private
```

**Visibility:**
- `public` — indexed, visible in discovery, used for matching
- `private` — encrypted in `.bin`, only revealed to matched users

**Optionality:**
- `required: true` — registration rejected if missing
- `required: false` — optional, can be omitted

**Four combinations:**
| | required | optional |
|---|---------|----------|
| **public** | Must provide, used in matching/discovery | Can provide, used if present |
| **private** | Must provide, only revealed after match | Can provide, only after match |

### matcher.yaml — Matching Rules (Optional)

```yaml
# matcher.yaml — pool-controlled matching
mode: pool    # pool | user | hybrid

rules:
  gender:
    score: complementary
    target: gender_target
    min: 1.0
    locked: true            # hybrid mode: users can't override

  age:
    score: proximity
    tolerance: 5
    min: 0.5
    weight: 2.0

  interests:
    score: overlap
    count: 1
    min: 0.0
    weight: 1.0
```

- No `matcher.yaml` = user-controlled mode (users define their own preferences)
- `mode: hybrid` + `locked: true` on some rules = users can adjust unlocked rules
- `mode: pool` = operator controls all matching, users just fill in attributes

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

Determined by `matcher.yaml`:

- **Pool-controlled** (`mode: pool`) — operator defines all matching rules. Users just fill in attributes.
- **User-controlled** (no `matcher.yaml`) — users define their own match preferences in their profile.
- **Hybrid** (`mode: hybrid`) — pool defines defaults, users can adjust unlocked rules. `locked: true` on a rule prevents user override.

### 4. User Match Preferences

Users write match rules against pool attributes (in user-controlled or hybrid mode):

```yaml
# User's match preferences (stored in profile)
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

Configured in `pool.json`:

```yaml
# tiered_shuffle — dating (fairness, avoid bias)
"ranking": { "strategy": "tiered_shuffle", "tiers": [0.05, 0.20, 0.50] }

# top_score — job board (best match first)
"ranking": { "strategy": "top_score" }

# weighted_random — networking (biased randomization)
"ranking": { "strategy": "weighted_random", "bias": 0.7 }
```

### 6. Seen Decay & Cooldown

Seen entries have timestamps. After cooldown, profiles re-enter the candidate pool:

```json
"ranking": { "seen_cooldown": "7d" }
```

Default = permanent (never reappear). Seen map: `map[string]time.Time`.

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
    attributes TEXT NOT NULL,      -- JSON: public attribute values only
    vector     BLOB,               -- pre-computed similarity vector
    updated_at INTEGER
);
```

Note: only `visibility: public` attributes go into `index.db`. Private attributes stay encrypted in `.bin`.

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

INSERT OR REPLACE INTO profiles (match_hash, attributes, vector, updated_at)
SELECT match_hash, attributes, vector, updated_at FROM remote.profiles;

DELETE FROM profiles WHERE match_hash NOT IN (SELECT match_hash FROM remote.profiles);

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

Match preference editors (user/hybrid mode):

| Matcher | TUI component |
|---------|---------------|
| exact value | Same as attribute input |
| `within: N` | Slider or number input |
| `min/max` | Two number inputs |
| `overlap: N` | Number input |
| weight | Small number next to each rule |

In `pool` mode → no preference section rendered.
In `hybrid` mode → locked fields shown as dimmed/read-only.

Private fields marked with a lock icon in the profile form.

### 10. Private Attribute Revelation

After a mutual match, users exchange encrypted profiles. The flow:

1. **Discovery**: user sees public attributes only (from `index.db`)
2. **Match**: mutual interest detected → Action posts encrypted notification with peer's pubkey
3. **Chat connect**: users derive ECDH shared secret
4. **Profile exchange**: users can request/send full profiles (including private fields) over the encrypted chat channel
5. **TUI**: match detail screen shows both public + private attributes

Private attributes are never in `index.db`, never on the relay, never visible to the operator (encrypted to peer's pubkey in the exchange).

---

## Examples

### Dating Pool

```
pool.json:   name="<3 dating", relay_url, ranking=tiered_shuffle
schema.yaml: gender, age, interests (public), phone (private)
matcher.yaml: mode=hybrid, gender=complementary+locked, age=proximity, interests=overlap
```

### Job Board

```
pool.json:   name="DevHire", ranking=top_score
schema.yaml: skills, experience (public), salary_expectation (private), resume_url (private)
matcher.yaml: mode=pool, skills=overlap, experience=proximity
```

### Open Networking

```
pool.json:   name="DevConnect", ranking=weighted_random
schema.yaml: interests, location, role (public), email (private)
matcher.yaml: (none — user-controlled mode)
```

---

## Migration from Current System

| Current | OpenPool |
|---------|----------|
| `pool.json` (monolithic) | `pool.json` + `schema.yaml` + `matcher.yaml` |
| `index.pack` (msgpack) | `index.db` (SQLite) |
| `suggestions.pack` | `suggestions.db` |
| 4 hardcoded match modes | 5 composable scoring primitives |
| Operator-only weights | User-configurable preferences (or pool/hybrid controlled) |
| `FilterValues` + vector | Unified scoring [0, 1] per field |
| `map[string]bool` seen | `map[string]time.Time` with cooldown |
| All attributes public | Public/private visibility per attribute |
