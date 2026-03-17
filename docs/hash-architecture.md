# Hash Architecture

Multi-layer pseudonym system ensuring unlinkability across contexts.

## Hash Types

```
PublicID → (salt) → bin-hash → (salt) → match-hash
```

| Hash | Computed by | Known to | Purpose |
|------|------------|----------|---------|
| **PublicID** | Client | Client, Action, Relay | Self-identity. `SHA256(pool_repo:provider:user_id)`. Used for auth proofs to Action. |
| **bin-hash** | Action/Relay | Action, Relay, Client (via relay API) | Pool repo filename (`users/{bin-hash}.bin`). `SHA256(salt:PublicID)[:16]`. Client sees others' bin-hashes from synced pool repo. |
| **match-hash** | Matching engine | Relay, matched users | Used in discovery results, like PRs, and labels. `SHA256(salt:bin-hash)` or similar. Unlinkable to bin-hash without salt. |

## Properties

- **Deterministic**: same input always produces same hash (no randomness)
- **One-way per layer**: knowing match-hash → can't derive bin-hash → can't derive PublicID
- **Salt is secret**: only Action and relay have `POOL_SALT`
- **Client can't self-compute bin-hash or match-hash** — must ask relay

## Who Knows What

| Entity | PublicID | bin-hash | match-hash |
|--------|----------|----------|------------|
| Client (self) | Yes (computes) | Yes (asks relay) | Yes (from discovery) |
| Client (others) | No | Yes (pool repo files) | Yes (from discovery) |
| Relay | Yes | Yes | Yes |
| Action | Yes | Yes | Yes |
| GitHub (public) | No | Yes (filenames visible) | Yes (in PR labels) |

## Current State

- `PublicID`: implemented (`crypto.UserHash()`)
- `bin-hash`: computed by Action, client doesn't query relay for it yet
- `match-hash`: not implemented (needs matching engine)
- Labels use full hash for now, will use match-hash when matching engine exists
