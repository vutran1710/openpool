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
| **match_hash** | 16 hex | Action + Relay | `SHA256(salt:bin_hash)[:16]`. Public handle for interest/chat. Unlinkable to bin_hash without salt. |

## Properties

- **Deterministic**: same input always produces same hash (no randomness)
- **One-way per layer**: knowing match-hash → can't derive bin-hash → can't derive PublicID
- **Salt is secret**: only Action and relay have `POOL_SALT`
- **Client can't self-compute bin-hash or match-hash** — must ask relay

## Who Knows What

| Entity | id_hash | bin_hash | match_hash |
|--------|---------|----------|------------|
| Client (self) | Yes (computes) | Yes (from registration) | Yes (from registration) |
| Client (others) | No | Yes (pool repo .bin filenames) | Yes (interest issue titles) |
| Relay | Yes (from connect params) | Yes (computes from salt) | Yes (computes from salt) |
| Action | Yes (from issue body) | Yes (computes from salt) | Yes (computes from salt) |
| GitHub (public) | No | Yes (.bin filenames visible) | Yes (interest issue titles) |

## Security

- **bin_hash is public** — visible in `.bin` filenames during discovery
- **match_hash is public** — appears as interest issue titles
- **Unlinkable without salt** — knowing bin_hash tells you nothing about match_hash (and vice versa)
- **Salt location** — only in pool repo GitHub Actions secrets + relay env var
- **id_hash → bin_hash → match_hash**: each layer is one-way without the salt
