# TOTP-Style Relay Auth + Issue-Based Registration

**Status**: Implemented (merged PR #15)
**Supersedes**: `2026-03-17-relay-jwt-auth-design.md`, `2026-03-17-per-pool-relay-refactor-design.md` (auth sections)

---

## Summary

Replace the relay's challenge/response WebSocket auth and JWT token system with a stateless TOTP-style scheme. The relay verifies a deterministic signature on every WebSocket connect — no login endpoint, no token issuance, no refresh loop.

Registration delivers `bin_hash` + `match_hash` to the CLI via an encrypted issue comment from the GitHub Actions registration workflow. The blob is encrypted using ephemeral NaCl box (`crypto.Encrypt` — no operator private key needed, GitHub Actions context proves origin). No external dependencies.

---

## Naming Convention

| Name | Derivation | Purpose |
|------|-----------|---------|
| `id_hash` | `sha256(pool_url:provider:user_id)` — full 64 hex chars | Public identity, used in registration |
| `bin_hash` | `sha256(salt:id_hash)[:16]` — 16 hex chars | Relay routing key, WebSocket connect identity |
| `match_hash` | `sha256(salt:bin_hash)[:16]` — 16 hex chars | Match verification + TOTP shared secret |

Go types: `crypto.IDHash` (was `PublicID`).

---

## 1. Registration Flow (One-Time)

### GitHub Actions Side

1. Registration Action triggers on join issue
2. Action downloads `regcrypt` binary from `vutran1710/regcrypt` releases (public repo)
3. Action runs `regcrypt` which computes:
   - `id_hash = sha256(pool_url:provider:user_id)`
   - `bin_hash = sha256(salt:id_hash)[:16]`
   - `match_hash = sha256(salt:bin_hash)[:16]`
4. `regcrypt` encrypts `{bin_hash, match_hash}` as msgpack, sealed with ephemeral NaCl box:
   - Uses `crypto.Encrypt(userPub, payload)` — ephemeral keypair, no operator key needed
   - Recipient: user's ed25519 public key (extracted from issue body)
5. Action posts encrypted blob as base64 issue comment
6. Action commits `.bin` file to pool repo (keyed by `bin_hash`)
7. Action closes issue as completed

If any step fails → Action closes issue as `not planned`.

### CLI Side

After submitting join issue (`pool join`):

1. CLI polls issue comments every 5s (max 5 minutes)
2. For each comment by `github-actions[bot]`:
   - Base64-decode the comment body
   - Attempt `crypto.Decrypt(userPriv, blob)` — ephemeral scheme, no operator key needed
3. On successful decryption:
   - Msgpack-decode → extract `bin_hash` + `match_hash`
   - Persist to local config (`~/.dating/setting.toml` under pool entry)
   - Set pool status to `active`
   - Done ✓
4. On timeout → saves pool as `pending` with `PendingIssue` number for retry

### Crypto: Ephemeral NaCl Box

```
Encrypt (regcrypt):  crypto.Encrypt(userPub, payload)
                     → generates ephemeral curve25519 keypair
                     → NaCl box.Seal with ephemeral_priv + user_pub
                     → output: [ephemeral_pub_32][nonce_24][ciphertext]

Decrypt (CLI):       crypto.Decrypt(userPriv, blob)
                     → extracts ephemeral_pub from blob
                     → NaCl box.Open with user_priv + ephemeral_pub
```

No operator key involved. GitHub Actions authentication proves origin (only the Action can post comments). The ephemeral pubkey is embedded in the ciphertext.

### Failure Semantics

**Action responsibility ends** after posting the encrypted comment + closing the issue. The hashes are permanently in the issue comments — the Action never needs to run again for this user.

**CLI is solely responsible** for retrieving and persisting the hashes. It can retry any number of times — just re-read the same closed issue's comments.

| Scenario | Result | User Action |
|----------|--------|-------------|
| Action succeeds, CLI polls + decrypts + persists | Registered + hashes delivered | None |
| Action fails (missing salt, crypto error, commit fail) | Issue closed `not planned` | Retry join (new issue) |
| CLI polls, sees issue closed `not planned` | CLI aborts immediately with error | Retry join (new issue) |
| CLI polls, no reply within 5 min | CLI saves as pending, aborts | Re-run CLI — it re-polls the same issue |
| CLI decrypts but persist fails (disk error) | CLI shows error | Re-run CLI — issue still has the comment, retries persist |
| CLI crashes mid-poll | No state lost | Re-run CLI — resumes polling same issue |
| User already registered | Action still computes hashes, posts encrypted comment, updates `.bin` | CLI still polls + decrypts — same flow every time |

**Key invariant**: once the issue has the encrypted comment, the CLI can always recover by re-polling. No re-registration needed.

---

## 2. TOTP-Style WebSocket Auth

### Connect

```
time_window = floor(unix_timestamp / 300)   // 5-minute buckets
message     = sha256(bin_hash + match_hash + time_window_string)
sig         = ed25519.Sign(priv_key, message)

ws://relay/ws?bin=<bin_hash>&sig=<hex(sig)>
```

No REST call. No JWT. No token refresh. Client recomputes on every connect.

### Relay Verification (Inline at WebSocket Upgrade)

1. Parse `bin` and `sig` from query params
2. Look up `UserEntry` by `bin_hash` → get `pubkey` + `match_hash`
3. Compute current `time_window = floor(now / 300)`
4. Try `time_window` ∈ `{current, current-1, current+1}` (clock drift tolerance):
   - `message = sha256(bin_hash + match_hash + tw_string)`
   - If `ed25519.Verify(pubkey, message, sig)` → accept
5. If no window matches → reject with 401, do not upgrade

### Security Properties

- **Two-factor proof**: attacker needs both `match_hash` (shared secret) AND `priv_key` (signing key) — compromise of either alone is insufficient
- **Replay window**: max 10 minutes (current ± 1 window × 5 min)
- **Stateless**: relay stores no tokens, no sessions (just the user index)
- **No token refresh**: TOTP recomputes automatically every 5 minutes
- **Salt-protected**: `match_hash` can't be derived without the server salt

---

## 3. What Was Removed from Relay

### Deleted Files
- `internal/relay/token.go` — token store with TTL
- `internal/relay/verify.go` — ed25519 verify wrapper
- `internal/relay/jwt.go` — JWT signing/verification (existed on some branches)

### Deleted Protocol Frames
- `AuthRequest` / `AuthResponse` / `Challenge` / `Authenticated` / `RefreshRequest`
- `TypeAuth` / `TypeChallenge` / `TypeAuthResponse` / `TypeAuthenticated` / `TypeRefresh`
- `Token` field from `Message` struct
- `ErrTokenExpired` / `ErrTokenInvalid` error codes

### Deleted Client Components
- `authenticate()` method (challenge/response handshake)
- `refreshLoop()` — no tokens to refresh
- `readFrame()` — only used by authenticate
- `token`, `hashID`, `userID`, `provider` fields on Client

---

## 4. What Remains on Relay

| Component | Purpose |
|-----------|---------|
| `Store` | `bin_hash → {pubkey, match_hash, user_id, provider}` + match pairs |
| `Hub` | Active WebSocket connections keyed by `bin_hash` |
| `Session` | Message routing, key exchange, ack handling |
| `HandleWS` | TOTP verify → upgrade → session |
| `HandleHealth` | Health check endpoint |
| `BinHash()` / `MatchHash()` | Hash derivation (for user index sync) |
| Key exchange | `KeyRequest` / `KeyResponse` for E2E encryption |

---

## 5. Current Structs

### Relay Server
```go
type ServerConfig struct {
    PoolURL string
    Salt    string
}

type Server struct {
    hub      *Hub
    store    *Store
    poolURL  string
    salt     string
    upgrader websocket.Upgrader
}
```

### Relay Store
```go
type UserEntry struct {
    PubKey    ed25519.PublicKey
    UserID    string
    Provider  string
    BinHash   string  // primary key
    MatchHash string  // TOTP shared secret
}
```

Store keyed by `bin_hash` (O(1) lookup on connect).

### CLI Relay Client
```go
type Config struct {
    RelayURL  string
    PoolURL   string              // for conversation key derivation
    BinHash   string              // from registration
    MatchHash string              // from registration (TOTP secret)
    Pub       ed25519.PublicKey
    Priv      ed25519.PrivateKey
}
```

### Pool Config (setting.toml)
```go
type PoolConfig struct {
    // ... existing fields ...
    BinHash   string `toml:"bin_hash,omitempty"`
    MatchHash string `toml:"match_hash,omitempty"`
}
```

---

## 6. regcrypt CLI Tool

Standalone Go binary at `cmd/regcrypt/`. Pre-built for `linux/amd64` and published to `vutran1710/regcrypt` (public repo, manual upload).

```
Usage: regcrypt \
  --pool-url <owner/repo> \
  --provider <github> \
  --user-id <issue_author_id> \
  --salt <pool_salt> \
  --user-pubkey <hex_ed25519_pubkey>

Output (stdout):
  Line 1: bin_hash (16 hex chars)
  Line 2: base64 ephemeral NaCl box encrypted {bin_hash, match_hash}
```

No `--operator-privkey` flag — uses ephemeral encryption.

### Distribution

```
https://github.com/vutran1710/regcrypt/releases/latest/download/regcrypt-linux-amd64
```

Binary uploaded manually. No cross-repo release workflow (will automate when dating-dev goes public).

### Action Secrets Required

| Secret | Purpose | Notes |
|--------|---------|-------|
| `POOL_SALT` | Derives bin_hash and match_hash | Already exists |

`OPERATOR_PRIVATE_KEY` is **not** needed by regcrypt (ephemeral encryption).

---

## 7. Crypto Functions

### TOTP Signature (Client) — `internal/crypto/totp.go`

```go
func TOTPSign(binHash, matchHash string, priv ed25519.PrivateKey) string
func TOTPSignAt(binHash, matchHash string, priv ed25519.PrivateKey, timeWindow int64) string
func TOTPVerify(binHash, matchHash, sigHex string, pub ed25519.PublicKey) bool
```

### IDHash — `internal/crypto/hash.go`

```go
type IDHash string  // was PublicID
func UserHash(poolRepo, provider, providerUserID string) IDHash
```
