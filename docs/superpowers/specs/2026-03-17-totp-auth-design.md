# TOTP-Style Relay Auth + Cloudflare Tunnel Registration

**Status**: Spec
**Supersedes**: `2026-03-17-relay-jwt-auth-design.md`, `2026-03-17-per-pool-relay-refactor-design.md` (auth sections)

---

## Summary

Replace the relay's challenge/response WebSocket auth and JWT token system with a stateless TOTP-style scheme. The relay verifies a deterministic signature on every WebSocket connect — no login endpoint, no token issuance, no refresh loop.

Registration delivers `bin_hash` + `match_hash` to the CLI via a Cloudflare Tunnel callback from the GitHub Actions registration workflow. The hashes are encrypted to the user's public key, so only the CLI can decrypt them.

---

## Naming Convention

| Name | Derivation | Purpose |
|------|-----------|---------|
| `id_hash` | `sha256(pool_url:provider:user_id)` — full 64 hex chars | Public identity, used in registration |
| `bin_hash` | `sha256(salt:id_hash)[:16]` — 16 hex chars | Relay routing key, WebSocket connect identity |
| `match_hash` | `sha256(salt:bin_hash)[:16]` — 16 hex chars | Match verification + TOTP shared secret |

---

## 1. Registration Flow (One-Time)

### CLI Side

#### Step 1: Start Callback Server + Tunnel

On CLI startup (before any pool join logic):

1. Bind a local HTTP server on a random port (loop-try `30000–40000` until one binds)
2. Start `cloudflared tunnel --url http://localhost:PORT` — parse public URL from stderr
3. **Self-verify** before proceeding:
   - HTTP GET the tunnel URL `/health` through the public internet
   - Expect `200 OK` with `{"status":"ready"}`
   - Retry up to 5 times with 1s backoff (tunnel takes ~2s to establish)
   - If verify fails after retries → abort with error, do not submit join issue
4. Only after verification passes: the callback server is confirmed reachable

This ensures the Action will always have a live endpoint to deliver to. The join issue is never created unless the tunnel is proven healthy.

#### Step 2: Join Flow

5. CLI submits pool join issue (includes tunnel URL + user's ed25519 public key)
6. CLI waits for POST callback on `/callback` (max 5 minutes timeout)
7. On receiving the encrypted blob:
   - Decrypt with private key
   - Extract `bin_hash` + `match_hash`
   - **Persist to local config** (`~/.dating/setting.toml`)
   - Only after successful persist: reply `200 OK` to the Action
8. Tunnel + callback server shut down

#### Callback Server Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/health` | GET | Self-verify: returns `{"status":"ready"}` |
| `/callback` | POST | Receives encrypted `{bin_hash, match_hash}` from Action |

### GitHub Actions Side

1. Registration Action triggers on join issue
2. Action computes:
   - `id_hash = sha256(pool_url:provider:user_id)`
   - `bin_hash = sha256(salt:id_hash)[:16]`
   - `match_hash = sha256(salt:bin_hash)[:16]`
3. Action encrypts `{bin_hash, match_hash}` with user's ed25519 pubkey (NaCl box, ed25519→curve25519)
4. Action POSTs encrypted blob to CLI's tunnel URL `/callback`
5. If CLI replies `200 OK`:
   - Action commits registration to pool repo (pubkey + bin_hash in user index)
   - Action closes issue as completed
6. If POST fails (timeout, non-200, tunnel down):
   - Action closes issue as `not planned`
   - No registration committed — clean failure, user retries

### Failure Semantics

| Scenario | Result | User Action |
|----------|--------|-------------|
| Tunnel up, CLI persists, Action commits | Registered + hashes delivered | None |
| Tunnel down before Action runs | Issue closed `not planned` | Retry join |
| CLI crashes before persist | No 200 → issue closed `not planned` | Retry join |
| CLI persists, 200 sent, Action fails to commit | CLI has hashes, not registered | Retry join — Action overwrites with same values |
| Network blip on POST | Issue closed `not planned` | Retry join |

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
| **New**: `internal/cli/tunnel/tunnel.go` | Cloudflare tunnel management: start `cloudflared`, capture URL, run callback server, shutdown. |

---

## 8. Registration Callback Protocol

### CLI Callback Server

```
POST /callback
Content-Type: application/octet-stream
Body: NaCl box encrypted {bin_hash, match_hash} as msgpack
```

CLI decrypts with its private key, persists to config, replies `200 OK`.

Any other status → Action treats as failure.

### Tunnel Startup + Self-Verify

```go
// 1. Bind random port
listener, port := tryBindRandom(30000, 40000)

// 2. Start callback server (goroutine)
srv := &http.Server{Handler: callbackHandler(privKey, persistFn)}
go srv.Serve(listener)

// 3. Start cloudflared
cmd := exec.Command("cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
tunnelURL := parseTunnelURL(cmd.Stderr) // blocks until URL printed

// 4. Self-verify: hit tunnel URL through public internet
verified := false
for i := 0; i < 5; i++ {
    resp, err := http.Get(tunnelURL + "/health")
    if err == nil && resp.StatusCode == 200 {
        verified = true
        break
    }
    time.Sleep(time.Duration(i+1) * time.Second)
}
if !verified {
    // Abort — do not submit join issue
    return fmt.Errorf("tunnel not reachable after retries")
}

// 5. Safe to submit join issue with tunnelURL
```

### Timeout

CLI waits max 5 minutes for the `/callback` POST. If no callback arrives, CLI exits with: "Waiting for registration... if this takes too long, check the issue status on GitHub."

### Dependency: cloudflared

CLI checks for `cloudflared` in PATH before starting. If not found:
```
Error: cloudflared not found. Install it:
  brew install cloudflare/cloudflare/cloudflared
  # or: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/
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
