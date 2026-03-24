# Security Design

Security rationale for the openpool platform. Covers trust model, cryptographic primitives, threat model, and known limitations.

---

## 1. Overview

The platform stores user data in public GitHub repositories (pool repos). All sensitive data is encrypted before it reaches GitHub. The relay server routes ciphertext only. GitHub is used purely for storage and notification delivery.

### What the system protects

- **Identity unlinkability**: discovery (bin_hash), matching (match_hash), and registration (id_hash) are three unlinkable namespaces
- **Interest privacy**: interest issue titles use ephemeral hashes that rotate every N days — real match_hash is never publicly visible
- **Profile confidentiality**: profiles encrypted to operator's pubkey; only operator and profile owner can read them
- **Discovery rate-limiting**: chain encryption prevents mass profile scraping
- **Chat confidentiality**: E2E encrypted (NaCl secretbox, ECDH-derived key); relay routes ciphertext only
- **Comment integrity**: operator-signed comments; clients reject unsigned or forged comments

### Trust model

| Entity | Trusted for | Not trusted for |
|--------|-------------|-----------------|
| GitHub | Hosting, Action execution, raw content | Access control (repos are public), comment authenticity |
| Operator | Signing comments, holding salt, running relay | Reading chat (relay is zero-knowledge) |
| Relay | Routing ciphertext by match_hash | Decryption, key storage |
| Client | Local key material, decryption | Nothing server-side |

---

## 2. Hash Chain

```
real_id    = "github:<username>"
             │
             ▼  sha256(pool_url + ":" + provider + ":" + user_id)
id_hash    = <64 hex chars>          ← registration identity
             │
             ▼  sha256(salt + ":" + id_hash)[:16]
bin_hash   = <16 hex chars>          ← public discovery key, .bin filename
             │
             ▼  sha256(salt + ":" + bin_hash)[:16]
match_hash = <16 hex chars>          ← semi-private: relay auth, chat routing
```

Each step is one-way with pool salt. Salt lives in exactly two places:
1. Pool repo GitHub Actions secrets (`POOL_SALT`)
2. Relay server environment variable (`POOL_SALT`)

### Ephemeral interest hashes

Interest issue titles use ephemeral hashes instead of raw match_hash:

```
window = unix_timestamp / interest_expiry_seconds
ephemeral_hash = sha256(match_hash + ":" + window)[:16]
```

- Rotates every `interest_expiry` (pool-configurable)
- Cannot be reversed to match_hash
- Stale interests auto-closed by squash cron

### Public visibility

| Hash | Public location | Visibility |
|------|----------------|------------|
| `bin_hash` | `.bin` filename in pool repo | Public (discovery) |
| `match_hash` | Nowhere on GitHub | Semi-private (known to user, relay, Action only) |
| `ephemeral_hash` | Interest issue titles | Public but temporary (rotates) |
| `id_hash` | Internal only | Never public |

### Timing correlation mitigation

History squashing (hourly cron) collapses all commits into one — no timeline to correlate registrations with index updates.

---

## 3. Chain Encryption (Discovery)

Profiles in `index.db` are chain-encrypted using AES-256-GCM:

- **Key**: `sha256("{hint}:{profile_constant}:{nonce}")` with randomized component ordering
- **Three tiers**: without hint = infeasible (10^8 × 10^4 × N × 6), cold = seconds, warm = instant
- **Sequential access**: each entry reveals the hint for the next — can't skip ahead
- **Multiple permutations**: different orderings per bucket, explorer jumps between them

This prevents:
- Mass scraping of profiles
- Parallel brute-force (sequential by design)
- Profile enumeration without doing computational work

---

## 4. Relay Authentication (TOTP)

Stateless, no tokens, no sessions:

```
Client → ws://relay/ws?id=<id_hash>&match=<match_hash>&sig=<totp_signature>

Relay:
  1. Derive bin_hash from id_hash using salt
  2. Derive match_hash from bin_hash using salt
  3. Assert derived match_hash == claimed match_hash
  4. Fetch pubkey from GitHub raw content (bin_hash → .bin file)
  5. Verify: ed25519.Verify(pubkey, sha256(time_window + relay_host), sig)
  6. Accept ±1 window (15 min tolerance)
```

Channel-bound to relay host — prevents replay across relays.

---

## 5. E2E Encrypted Chat

Key derivation from existing ed25519 keys:

```
1. ed25519 private → curve25519 scalar
2. peer ed25519 pub → curve25519 point
3. shared_secret = X25519(myScalar, theirPoint)
4. conversation_key = HKDF-SHA256(shared_secret, info="openpool-e2e:<hashA>:<hashB>:<poolURL>")
```

Encryption: NaCl secretbox (XSalsa20-Poly1305). Wire format: `version(1B) || nonce(24B) || ciphertext`.

Relay sees only: `[target_match_hash (8B)][ciphertext]`.

---

## 6. Match Notification as Key Exchange

Mutual match → Action encrypts notification to each user containing peer's **pubkey**:

- Match notification IS the key exchange
- Each user receives peer's ed25519 pubkey encrypted to their own pubkey
- Client derives ECDH shared secret immediately
- Relay never stores or serves pubkeys

Match files (`matches/{pair_hash}.json`) are empty — existence is the signal, commit time is the timestamp.

---

## 7. Operator-Signed Comments

All Action-posted comments use: `base64(ciphertext).hex(ed25519_signature)`

- Signature covers raw ciphertext bytes
- CLI verifies signature BEFORE decryption
- Reverse iteration (newest first) finds real comment in O(1)
- Action closes issue first, then posts signed comment

---

## 8. Payload Size Limits

| Channel | Limit | Rationale |
|---------|-------|-----------|
| Chat plaintext | 4 KB | ~500 words |
| Relay binary frame | 8 KB | ciphertext + overhead |
| Issue body | 64 KB | Profile blob + metadata |
| `.bin` file | 256 KB | Encrypted profile |

Enforced at every layer (client, relay, Action).

---

## 9. Threat Model

### Protected

| Threat | Protection |
|--------|-----------|
| Linking bin_hash ↔ match_hash | Hash chain (requires salt) + history squashing |
| Tracking who liked whom | Ephemeral interest hashes (rotate every N days) |
| Mass profile scraping | Chain encryption (rate-limited discovery) |
| Fake registration/match comments | Operator signature required |
| Chat eavesdropping | E2E encryption (relay is zero-knowledge) |
| Relay impersonation | TLS + channel binding |
| Stale interest tracking | Auto-cleanup by squash cron |

### Not protected

| Issue | Notes |
|-------|-------|
| GitHub metadata is public | File counts, timestamps visible. Mitigated by squashing. |
| Relay sees traffic patterns | Who connects when, but not content. |
| No forward secrecy | Static ECDH keys. Key compromise exposes past chats. |
| Stolen private key = full compromise | Same model as SSH/PGP. Recovery: re-register. |

---

## 10. Key Files

| File | Purpose |
|------|---------|
| `internal/crypto/` | All crypto primitives (TOTP, ECDH, NaCl, ephemeral hashes) |
| `internal/chainenc/` | Chain encryption (AES-256-GCM, key derivation, solving) |
| `internal/relay/server.go` | TOTP auth, hash chain validation |
| `internal/github/comments.go` | Operator signature verification |
| `cmd/action-tool/` | register, match, unmatch, squash, index |
| `templates/actions/` | Action templates (5 workflows) |
