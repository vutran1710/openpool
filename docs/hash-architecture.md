# Hash Architecture

Multi-layer pseudonym system ensuring unlinkability across contexts.

## Hash Types

```
id_hash → (salt) → bin_hash → (salt) → match_hash
```

| Hash | Length | Computed by | Purpose |
|------|--------|------------|---------|
| **id_hash** | 64 hex | Client + Action | `SHA256(pool_repo:provider:user_id)`. Identity proof at registration. |
| **bin_hash** | 16 hex | Action + Relay | `SHA256(salt:id_hash)[:16]`. Filename in pool repo (`users/{bin_hash}.bin`). Public in discovery. |
| **match_hash** | 16 hex | Action + Relay | `SHA256(salt:bin_hash)[:16]`. Used for relay auth + chat routing. Semi-private (see below). |

## Ephemeral Interest Hash

Interest issue titles do NOT use raw match_hash. Instead:

```
window = unix_timestamp / interest_expiry_seconds
ephemeral_hash = SHA256(match_hash + ":" + window)[:16]
```

- Rotates every `interest_expiry` (pool-configurable, e.g. 3 days)
- Cannot be reversed to match_hash
- Prevents long-term tracking of who liked whom
- Real target_match_hash is inside the encrypted issue body (only operator can read)

## Properties

- **Deterministic**: same input always produces same hash (no randomness)
- **One-way per layer**: knowing match_hash → can't derive bin_hash → can't derive id_hash
- **Salt is secret**: only Action and relay have `POOL_SALT`
- **Client can't self-compute bin_hash or match_hash** — receives them from operator after registration

## Who Knows What

| Entity | id_hash | bin_hash | match_hash | ephemeral_hash |
|--------|---------|----------|------------|----------------|
| Client (self) | Yes (computes) | Yes (from registration) | Yes (from registration) | Yes (computes) |
| Client (others) | No | Yes (.bin filenames) | No | No |
| Relay | Yes (connect params) | Yes (from salt) | Yes (from salt) | N/A |
| Action | Yes (issue body) | Yes (from salt) | Yes (from salt) | Yes (computes) |
| GitHub (public) | No | Yes (.bin filenames) | No | Yes (interest issue titles, but rotates) |

## Security

- **bin_hash is public** — visible in `.bin` filenames during discovery
- **match_hash is semi-private** — known only to the user, relay, and Action. NOT visible on GitHub (interest titles use ephemeral hashes instead)
- **ephemeral_hash is public but temporary** — visible as interest issue titles, rotates every N days, cannot be linked to match_hash
- **Unlinkable without salt** — knowing bin_hash tells you nothing about match_hash (and vice versa)
- **Salt location** — only in pool repo GitHub Actions secrets + relay env var
- **id_hash → bin_hash → match_hash**: each layer is one-way without the salt
