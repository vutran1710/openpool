# VDF-Gated Discovery

## Status: Brainstorming

## Core Idea

Instead of a public index file that anyone can read in full, discovery is gated by a Verifiable Delay Function (VDF). Users must solve sequential computational puzzles to reveal profiles one at a time. No server needed — fully client-side.

## Design

### Tree-Based Bucketing

Pool schema defines a profile tree. Attributes split profiles into buckets:

```
[gender_target] → [age_range] → [city_cluster] → Leaf bucket
```

- Split order follows schema field priority (operator-defined)
- Leaves must have minimum N profiles (privacy threshold, e.g., 10)
- If a split would create too-small buckets, stop splitting
- Tree structure is public metadata (client navigates it)
- Leaf contents (profiles) are encrypted

### Permutation Chains

Each leaf bucket contains M pre-computed permutations of the same N profiles. Each permutation is a VDF chain:

```
Indexer builds:
  Permutation 1: [A3, A7, A1, A5, ...] → VDF chain
  Permutation 2: [A7, A1, A5, A3, ...] → VDF chain
  ...
  Permutation M: [A5, A3, A7, A1, ...] → VDF chain

Client:
  Pick permutation = hash(user_key) mod M
  Peel: solve VDF step → reveal profile → solve next → reveal next
```

### VDF Construction (Iterated Hashing)

```
seed = initial value for the chain
proof_0 = sha256^K(seed)           → decrypts profile at position 0
proof_1 = sha256^K(proof_0)        → decrypts profile at position 1
...

K = iteration count tuned to target delay (~1-5 seconds per profile)
```

Properties:
- **Sequential** — can't compute step N without step N-1
- **Non-parallelizable** — each step depends on the previous
- **Deterministic** — same seed = same chain (verifiable)

### Per-User Variation

Different users pick different permutations:
```
my_permutation = hash(user_match_hash) mod M
```

M=3-5 is sufficient. Buckets are sparse (10-50 profiles per leaf due to min_bucket_size). 5 permutations of 30 profiles = negligible overhead. Some users share a permutation — acceptable since VDF is the real gating mechanism, not the permutation.

### Compression

Profiles repeat across M permutations. LZ4 compression on the full bucket data:

```
Per bucket:  M × N × profile_size (e.g., 5 × 30 × 500B = 75KB)
Full index:  buckets × per_bucket (e.g., 1000 × 75KB = 75MB raw)
Compressed:  ~15MB (LZ4, profiles repeat across permutations)
```

### Index as Release Asset

Index file uploaded as GitHub release asset (not committed to repo):
- Avoids repo bloat
- One release tag (`index-latest`), overwrite on each rebuild
- Client downloads via `gh release download` or direct URL

## Flow

```
Indexer (Action cron):
  1. Read pool schema → determine tree structure
  2. Decrypt all .bin profiles
  3. Build tree → group into leaf buckets
  4. For each bucket: create M permutations, build VDF chains
  5. LZ4 compress
  6. Upload as release asset

Client (local):
  1. Download index from release
  2. Decompress
  3. Navigate tree based on own profile → find target bucket
  4. Pick permutation = hash(my_key) mod M
  5. Start VDF chain → reveal one profile per solve
  6. Display revealed profile → user decides (like/skip)
  7. Solve next → repeat
```

## Open Questions

- How to handle ranking within permutations? Should the indexer sort each permutation by match score before encrypting?
- What's the right K (VDF difficulty)? Needs to be tuned per hardware. Too fast = no protection, too slow = bad UX.
- How does the client know which bucket to enter without revealing their profile to the index? (Tree navigation is based on their own attributes — is the tree structure leaking info?)
- Should bucket membership be encrypted too? (Double-layer: encrypted bucket membership + encrypted profiles within)
- M permutations × N profiles storage — what's the practical size limit?
- How does the matching engine redesign (schema, scoring) integrate with bucket construction?
