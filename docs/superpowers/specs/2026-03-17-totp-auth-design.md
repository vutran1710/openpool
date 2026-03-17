# TOTP-Style Relay Auth + Issue-Based Registration

**Status**: Spec
**Supersedes**: `2026-03-17-relay-jwt-auth-design.md`, `2026-03-17-per-pool-relay-refactor-design.md` (auth sections)

---

## Summary

Replace the relay's challenge/response WebSocket auth and JWT token system with a stateless TOTP-style scheme. The relay verifies a deterministic signature on every WebSocket connect — no login endpoint, no token issuance, no refresh loop.

Registration delivers `bin_hash` + `match_hash` to the CLI via an encrypted issue comment from the GitHub Actions registration workflow. The blob is encrypted using NaCl box (operator's private key + user's public key), so only the user's private key can decrypt it. No external dependencies (no tunnel, no cloudflared).

---

## Naming Convention

| Name | Derivation | Purpose |
|------|-----------|---------|
| `id_hash` | `sha256(pool_url:provider:user_id)` — full 64 hex chars | Public identity, used in registration |
| `bin_hash` | `sha256(salt:id_hash)[:16]` — 16 hex chars | Relay routing key, WebSocket connect identity |
| `match_hash` | `sha256(salt:bin_hash)[:16]` — 16 hex chars | Match verification + TOTP shared secret |

---

## 1. Registration Flow (One-Time)

### GitHub Actions Side

1. Registration Action triggers on join issue
2. Action computes:
   - `id_hash = sha256(pool_url:provider:user_id)`
   - `bin_hash = sha256(salt:id_hash)[:16]`
   - `match_hash = sha256(salt:bin_hash)[:16]`
3. Action encrypts `{bin_hash, match_hash}` as msgpack, sealed with NaCl box:
   - Sender: operator's ed25519 private key (from GitHub secrets, converted to curve25519)
   - Recipient: user's ed25519 public key (from issue body, converted to curve25519)
4. Action posts encrypted blob as base64 issue comment
5. Action commits registration to pool repo (pubkey + bin_hash in user index)
6. Action closes issue as completed

If any step fails → Action closes issue as `not planned`.

### CLI Side (Gatekeeper)

After submitting join issue:

1. CLI polls issue comments every 5s (max 5 minutes)
2. For each comment by `github-actions[bot]`:
   - Base64-decode the comment body
   - Attempt NaCl box open with `user_priv_key` + `operator_pub_key`
   - `operator_pub_key` is already in pool config (`operator_public_key` field)
3. On successful decryption:
   - Msgpack-decode → extract `bin_hash` + `match_hash`
   - Persist to local config (`~/.dating/setting.toml` under pool entry)
   - Done ✓
4. On timeout → "Registration timed out. Check issue status on GitHub."

### Crypto: NaCl Box (Diffie-Hellman)

```
Encrypt (Action):  nacl_box_seal(plaintext, operator_priv → curve25519, user_pub → curve25519)
Decrypt (CLI):     nacl_box_open(ciphertext, user_priv → curve25519, operator_pub → curve25519)

Both sides compute the same shared secret:
  ECDH(operator_priv, user_pub) == ECDH(user_priv, operator_pub)
```

An attacker who sees the issue comment has:
- The ciphertext (public)
- The operator's public key (public)
- The user's public key (public)

But cannot decrypt without the user's **private** key. The project already uses this construction in `internal/crypto/encrypt.go`.

### Failure Semantics

| Scenario | Result | User Action |
|----------|--------|-------------|
| Action succeeds, CLI polls + decrypts | Registered + hashes delivered | None |
| Action fails at any step | Issue closed `not planned` | Retry join |
| CLI polls but issue closed `not planned` | CLI sees closed status, aborts | Retry join |
| CLI timeout (Action slow/stuck) | CLI aborts with message | Check issue, retry |
| CLI decrypts but can't persist | CLI retries persist, then aborts | Fix disk, retry join |

### No External Dependencies

- No tunnel (cloudflared, ngrok, etc.)
- No port binding
- No callback server
- Works behind any firewall/NAT
- Only needs GitHub API access (which the CLI already has)

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

## 3. What Gets Removed from Relay

### Deleted Endpoints
- `POST /login`
- `POST /login/refresh`
- `GET /me`

### Deleted Server Components
- `jwt.go` / `token.go` — JWT/token signing, verification, storage
- `TokenStore` — in-memory token management
- `JWTClaims` / `SignJWT` / `VerifyJWT` / `DeriveJWTSecret`
- `operatorKey`, `jwtSecret`, `tokenTTL` fields on `Server`

### Deleted Protocol Frames
- `AuthRequest` / `AuthResponse` / `Challenge` / `Authenticated` / `RefreshRequest`
- `TypeAuth` / `TypeChallenge` / `TypeAuthResponse` / `TypeAuthenticated` / `TypeRefresh`

### Deleted Client Components
- `authenticate()` method (challenge/response handshake)
- `refreshLoop()` — no tokens to refresh
- `Login()` / REST login flow
- `token` field on Client

### Simplified ServerConfig
```go
type ServerConfig struct {
    PoolURL string
    Salt    string
}
```

### Simplified Server
```go
type Server struct {
    hub      *Hub
    store    *Store
    poolURL  string
    salt     string
    upgrader websocket.Upgrader
}
```

---

## 4. What Remains on Relay

| Component | Purpose |
|-----------|---------|
| `Store` | `bin_hash → {pubkey, match_hash, user_id, provider}` + match pairs |
| `Hub` | Active WebSocket connections keyed by `bin_hash` |
| `Session` | Message routing, key exchange, ack handling |
| `HandleWS` | TOTP verify → upgrade → session |
| `HandleHealth` | Health check endpoint |
| Key exchange | `KeyRequest` / `KeyResponse` for E2E encryption |

---

## 5. Store Changes

`UserEntry` gains `MatchHash`:

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

---

## 6. Client Config Changes

`Config` for relay client:

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

No `UserID` / `Provider` needed — relay identifies by `bin_hash`.

`setting.toml` per-pool config gains:

```toml
[pools.example]
bin_hash = "abc123..."
match_hash = "def456..."
```

---

## 7. File Change Summary

| File | Changes |
|------|---------|
| `internal/relay/server.go` | Remove all auth endpoints. Add TOTP verify in `HandleWS`. Remove `operatorKey`, `jwtSecret`, `tokenTTL`. Simplify `ServerConfig`. |
| `internal/relay/jwt.go` or `token.go` | **Delete entirely** |
| `internal/relay/session.go` | Remove auth handshake from `run()`. Session starts post-auth (TOTP already verified at upgrade). Remove `handleAuth`, `handleRefresh`. |
| `internal/relay/store.go` | Add `MatchHash` to `UserEntry`. Rekey to `bin_hash`. |
| `internal/protocol/types.go` | Remove `AuthRequest`, `AuthResponse`, `Challenge`, `Authenticated`, `RefreshRequest` and their type constants. |
| `internal/cli/relay/client.go` | Remove `authenticate()`, `refreshLoop()`, `Login()`. Add TOTP computation in `Connect()`. Config gains `BinHash`, `MatchHash`. Remove `token` field. |
| `internal/cli/config/config.go` | Add `BinHash`, `MatchHash` to `PoolConfig`. |
| `internal/crypto/hash.go` | Rename `PublicID` → `IDHash`. |
| `cmd/relay/main.go` | Remove operator key env vars. Keep `POOL_URL`, `POOL_SALT`. |
| `internal/relay/relay_e2e_test.go` | Rewrite: no auth handshake, connect with `?bin=&sig=`. Add `MatchHash` to test `UserEntry`. |
| `internal/cli/relay/client_test.go` | Update for new `Config` shape. Remove token/auth tests. |
| **New**: `internal/cli/gatekeeper/` | Poll issue comments, decrypt NaCl box blob, persist hashes to config. |

---

## 8. Gatekeeper Package

`internal/cli/gatekeeper/` — handles registration credential delivery.

### Responsibilities

1. Poll GitHub issue comments for encrypted blob
2. Decrypt with NaCl box (user_priv + operator_pub)
3. Decode msgpack → `{bin_hash, match_hash}`
4. Persist to pool config in `setting.toml`

### Interface

```go
// Result holds the hashes delivered during registration.
type Result struct {
    BinHash   string
    MatchHash string
}

// Await polls the join issue for the encrypted hash delivery.
// Returns when hashes are received and persisted, or on timeout.
func Await(ctx context.Context, cfg AwaitConfig) (*Result, error)

type AwaitConfig struct {
    GitHubClient  *github.Client
    IssueNumber   int
    PoolRepo      string          // "owner/repo"
    UserPrivKey   ed25519.PrivateKey
    OperatorPub   ed25519.PublicKey
    PollInterval  time.Duration   // default 5s
    Timeout       time.Duration   // default 5min
}
```

---

## 9. Crypto Functions

### TOTP Signature (Client)

```go
func TOTPSign(binHash, matchHash string, priv ed25519.PrivateKey) string {
    tw := time.Now().Unix() / 300
    msg := sha256.Sum256([]byte(binHash + matchHash + strconv.FormatInt(tw, 10)))
    sig := ed25519.Sign(priv, msg[:])
    return hex.EncodeToString(sig)
}
```

### TOTP Verify (Relay)

```go
func TOTPVerify(binHash, matchHash, sigHex string, pub ed25519.PublicKey) bool {
    sigBytes, err := hex.DecodeString(sigHex)
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return false
    }
    now := time.Now().Unix() / 300
    for _, tw := range []int64{now, now - 1, now + 1} {
        msg := sha256.Sum256([]byte(binHash + matchHash + strconv.FormatInt(tw, 10)))
        if ed25519.Verify(pub, msg[:], sigBytes) {
            return true
        }
    }
    return false
}
```
