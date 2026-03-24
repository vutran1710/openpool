# Chain Encryption

Rate-limited profile discovery using sequential AES-256-GCM encryption.

## Problem

Profiles are encrypted in pool repos. Without rate-limiting, anyone who downloads `index.db` could brute-force all profiles instantly. We need a mechanism that forces sequential work — you can't skip ahead or parallelize.

## Model

Profiles are organized into **buckets** (by role, age range, skills, etc.) and encrypted into **chains**. Each chain entry reveals the hint needed to solve the next — enforcing sequential access.

### Three Components per Entry

| Component | Range | Source |
|-----------|-------|--------|
| **hint** | 10^8 (8 digits) | Revealed by solving the previous entry |
| **profile_constant** | 10^4 (4 digits) | `sha256(profile_content) % 10000` — fixed per profile |
| **nonce** | `nonce_space` (configurable) | Random per entry per permutation |

### Key Derivation

```
ordering = pick_ordering(chain_context)   # one of 6 (3!) orderings
key = sha256("{a}:{b}:{c}")               # a,b,c = hint, constant, nonce in random order
ciphertext = AES-256-GCM(key, profile_plaintext)
```

The ordering is deterministic per chain position but unknown to the explorer — adds 6x multiplier to search.

### Payload

Each decrypted entry contains:

```json
{
  "profile": { "name": "Alice", "age": 27, "interests": ["coding"] },
  "my_constant": 4821,
  "next_hint": 73482159
}
```

- `profile` — the user's public attributes
- `my_constant` — saved by explorer for faster re-encounters
- `next_hint` — enables solving the next entry in the chain

## Security Tiers

| Scenario | Search Space | Time (~10M attempts/s) |
|----------|-------------|------------------------|
| **Without hint** | 10^8 × 10^4 × N × 6 | Days to years (infeasible) |
| **Cold** (have hint) | 10^4 × N × 6 | Seconds |
| **Warm** (know constant) | N × 6 | Instant |

- **Without hint**: seed provides the first hint; without it, search is infeasible
- **Cold**: first time seeing a profile — brute-force constant × nonce × ordering
- **Warm**: already solved this profile in another permutation — know the constant, only brute-force nonce × ordering

## Buckets

Profiles are partitioned into buckets based on pool.yaml config:

```yaml
indexing:
  partitions:
    - field: role          # discrete: one bucket per value
    - field: age           # range: step=5, overlap=2
      step: 5
      overlap: 2
  permutations: 5          # shuffle orderings per bucket
  difficulty: 20           # nonce_space — single tuning knob
```

Each bucket has N permutations — same profiles in different order. The explorer picks a permutation randomly and grinds through it.

## Difficulty Tuning

The `difficulty` field (nonce_space) controls cold unlock time:

| difficulty | Cold time (~10M/s) | Profiles/hour |
|------------|-------------------|---------------|
| 5 | ~0.03s | ~120,000 |
| 20 | ~0.12s | ~30,000 |
| 200 | ~1.2s | ~3,000 |
| 1000 | ~6s | ~600 |

## Data Flow

```
Indexer (Action cron):
  decrypt .bin files → bucket by partitions → chain-encrypt → write index.db → upload release asset

Explorer (client):
  download index.db → select bucket (from preferences) → grind chain → unlock one profile at a time
```

## Permutation Jumping

The explorer can jump between permutations:

- Known constants from any permutation apply everywhere (warm solve)
- Checkpoints saved per permutation (resume where left off)
- On new index rebuild, checkpoints are cleared (permutations reshuffled) but constants persist

## Local State

```
~/.openpool/pools/<pool>/
  index.db        — downloaded chain-encrypted index (read-only)
  explorer.db     — local state:
    constants     — known profile_constants (carry across rebuilds)
    checkpoints   — resume position per bucket/permutation
    seen          — skip/like/pass history
```

## Implementation

| Package | Purpose |
|---------|---------|
| `internal/chainenc/` | Key derivation, AES-256-GCM encrypt/decrypt, chain building/solving |
| `internal/bucket/` | Partition profiles into buckets from pool.yaml config |
| `internal/indexer/` | Build chain-encrypted index.db from .bin files |
| `internal/explorer/` | Grind chains, track state (constants, checkpoints, seen) |

## Reference

Python demo: `docs/concepts/chain-encryption-demo.py`
