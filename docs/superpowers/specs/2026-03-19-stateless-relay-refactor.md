# Stateless Relay Refactor

## Goal

Eliminate the relay's database (SQLite) and stateful endpoints by deriving all identity from a salt, fetching pubkeys and match authorization from GitHub raw content, replacing the MessagePack protocol with minimal binary frames, and removing the key exchange protocol entirely.

## Motivation

The current relay stores user entries (bin_hash → pubkey, match_hash) and match pairs in SQLite. This is unnecessary — the relay already has the salt and can derive all hashes on the fly. Pubkeys live in `.bin` files on GitHub. Match authorization lives in `matches/{pair_hash}.json` on GitHub. The key exchange protocol (`key_request`/`key_response`) can be eliminated by delivering peer pubkeys in match notifications.

## Security Model

### Hash Chain

```
real_id    = github:username                        → private, never leaves client
id_hash    = sha256(pool_url:provider:user_id)     → 64 hex, one-way from real_id
bin_hash   = sha256(salt:id_hash)[:16]             → 16 hex, one-way from id_hash
match_hash = sha256(salt:bin_hash)[:16]             → 16 hex, one-way from bin_hash
```

Each layer is unlinkable without the salt. The salt only exists in:
1. Pool repo's GitHub Actions secrets
2. Relay server's environment variable

### Connect Authentication

```
Client sends:
  GET /ws?id=<id_hash>&match=<match_hash>&sig=<hex_sig>

  sig = ed25519.Sign(priv, sha256(time_window))
  time_window = strconv.FormatInt(unix_now / 300, 10)  (5-minute buckets)

Relay validates:
  1. bin_hash = sha256(salt:id_hash)[:16]
  2. expected_match = sha256(salt:bin_hash)[:16]
  3. Assert expected_match == match_hash  → rejects if chain doesn't match
  4. pubkey = fetch users/{bin_hash}.bin[:32] from GitHub (cached, no TTL)
  5. ed25519.Verify(pubkey, sha256(time_window), sig)  ±1 window for clock drift
  6. Valid → upgrade WebSocket, session keyed by match_hash
```

The signature only signs the time window — identity binding comes from the chain validation (step 3) and pubkey verification (step 5). Cross-pool replay is not a concern because each pool has a different salt, so the chain check fails for a different pool's relay.

## Wire Format

### Chat Messages (Binary Frames)

```
Send:    [ target_match_hash (8 bytes) ][ ciphertext (variable) ]
Receive: [ sender_match_hash (8 bytes) ][ ciphertext (variable) ]

target_match_hash = hex-decoded match_hash (16 hex chars = 8 bytes raw)
ciphertext = raw bytes: version(1) + nonce(24) + secretbox sealed data
```

### Control Frames (Text Frames)

Relay sends plain UTF-8 text frames for errors and status notifications:

| Text Frame | Meaning |
|------------|---------|
| `not matched` | Pair not found in match files — message rejected |
| `match check failed` | Transient GitHub error — retry later |
| `queue full` | Target's offline queue is at capacity — message rejected |
| `queued` | Target is offline, message queued for delivery |
| `unknown user` | Auth: bin_hash has no .bin file on GitHub |

Client distinguishes: binary frame = chat message, text frame = control/status.

### What's Eliminated

- Entire `internal/protocol/` package (MessagePack types + codec)
- Frame types: `Message`, `Ack`, `KeyRequest`, `KeyResponse`, `Error`
- Fields: `source_hash`, `msg_id`, `ts`, `encrypted`, `type`

## Message Routing

```
Relay receives binary frame from sender (session.matchHash = sender's match_hash):
  1. Extract target_match_hash from first 8 bytes (hex-encode → 16 hex chars)
  2. pair_hash = sha256(min(a,b) + ":" + max(a,b))[:12]  where a,b = sender/target match_hashes
  3. Check match: GET raw.githubusercontent.com/{pool}/main/matches/{pair_hash}.json
     - Cached in memory with 5-minute TTL
     - HTTP 200 → cached as positive (matched)
     - HTTP 404 → cached as negative (not matched), send text frame "not matched"
     - HTTP 5xx / network error → NOT cached, send text frame "match check failed" (transient)
  4. Lookup target session in hub (keyed by match_hash)
     - Online → forward as [sender_match_hash (8 bytes)][ciphertext]
     - Offline → append to in-memory queue, send text frame "queued" to sender
```

## Hub & Session

### Hub (Re-keyed by match_hash)

```go
type Hub struct {
    mu       sync.RWMutex
    sessions map[string]*Session    // match_hash → session
    queue    map[string][][]byte    // match_hash → pending binary messages
}
```

### Offline Queue

- In-memory only (lost on relay restart — acceptable)
- Capped at 20 messages per user; when full, newest message is rejected with text frame "queue full"
- On connect: flush queued messages before entering read loop

### Session Replacement

When a user reconnects (same match_hash already in hub):
- Old WebSocket connection is closed (`conn.Close()`)
- Old session's read loop goroutine terminates on the close error
- New session replaces old in hub map

### Session

```go
type Session struct {
    conn      *websocket.Conn
    server    *Server
    writeMu   sync.Mutex
    matchHash string    // session identity
}
```

## Server

### Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /ws` | upgrade | Stateless TOTP auth + WebSocket |
| `GET /health` | GET | `{status, online, queued}` |

### Removed Endpoints

| Endpoint | Reason |
|----------|--------|
| `POST /sync` | No user index — pubkeys fetched from GitHub |
| `POST /match` | Match files on GitHub |

### Server Struct

```go
type Server struct {
    hub      *Hub
    poolURL  string
    salt     string
    pubkeys  sync.Map              // bin_hash → []byte (ed25519 pubkey, no TTL)
    matches  sync.Map              // pair_hash → matchCacheEntry{exists bool, fetchedAt time.Time}
    upgrader websocket.Upgrader
}
```

### Caching

| Cache | Key | TTL | Source |
|-------|-----|-----|--------|
| Pubkeys | bin_hash | None (immutable) | `raw.githubusercontent.com/{pool}/main/users/{bin_hash}.bin[:32]` |
| Matches | pair_hash | 5 minutes | `raw.githubusercontent.com/{pool}/main/matches/{pair_hash}.json` |

### Environment Variables

| Var | Required | Default | Purpose |
|-----|----------|---------|---------|
| `POOL_URL` | Yes | — | Repository path (e.g., `owner/pool-name`) |
| `POOL_SALT` | Yes | — | Secret salt for hash derivation |
| `PORT` | No | `8081` | Listen port |

Removed: `DB_PATH` (no SQLite).

### Error Handling (GitHub Unavailability)

| Scenario | Behavior |
|----------|----------|
| GitHub down during auth (pubkey fetch) | If pubkey already cached → auth succeeds. If not cached → connection rejected with 503 |
| GitHub down during match check | If match already cached → use cache. If not cached → text frame "match check failed" (transient, NOT cached as negative) |
| GitHub rate limited (429) | Same as network error — transient, not cached |
| `.bin` file not found (404) | Connection rejected with 401 "unknown user" |

Pubkeys are immutable (cached forever), so GitHub outages only affect first-time connections. Match cache has 5-min TTL, so existing conversations survive short outages.

## Client Changes

### Config

```go
type Config struct {
    RelayURL  string
    PoolURL   string
    IDHash    string              // NEW: for connect URL
    MatchHash string              // for TOTP signing + peer identification
    Pub       ed25519.PublicKey
    Priv      ed25519.PrivateKey
}
```

Removed: `BinHash` (client no longer needs it for relay communication).

### Connect

```go
sig := ed25519.Sign(priv, sha256(timeWindow))
GET /ws?id=<id_hash>&match=<match_hash>&sig=<hex_sig>
```

### Send

```go
frame := append(hexDecode(targetMatchHash), SealRaw(key, []byte(plaintext))...)
conn.WriteMessage(BinaryMessage, frame)
```

Note: `SealRaw` is a new function returning raw `[]byte` (version + nonce + secretbox sealed data) instead of base64. The existing `SealMessage` (base64) is kept for other uses but not used in the wire protocol.

### Receive

```go
// Text frame → error message
// Binary frame → first 8 bytes = sender match_hash, rest = ciphertext
senderMatch := hexEncode(data[:8])
plaintext := OpenMessage(key, data[8:])
```

### Key Derivation

No key exchange protocol. Peer's pubkey comes from the match notification (decrypted at match discovery time). Client derives conversation key on first message:

```
shared = ECDH(myPriv, peerPub)
key = HKDF(shared, info="dating-e2e:" + min(a,b) + ":" + max(a,b) + ":" + poolURL)
    where a,b = myMatchHash, peerMatchHash
```

### Chat Command

`dating chat <match_hash>` (was `dating chat <bin_hash>`).

## matchcrypt Changes

### encrypt (updated payload)

```
Current:  {matched_bin_hash, greeting}
New:      {matched_match_hash, greeting, pubkey}
```

- `matched_bin_hash` removed (client doesn't need bin_hash for chat)
- `matched_match_hash` added (client identifies peer by match_hash)
- `pubkey` added (hex-encoded ed25519 — this IS the key exchange)

### encrypt CLI flags

```
matchcrypt encrypt \
  --user-pubkey <recipient_pub> \
  --match-hash <peer_match_hash> \
  --peer-pubkey <peer_pub> \
  --greeting "<greeting>"
```

### decrypt (no change to command)

Decrypts interest PR body → `{author_bin_hash, author_match_hash, greeting}`. This is the interest PR body (submitted by the liker), not the match notification. The `decrypt` command is used by the Action to identify the PR author, not by the client.

Note: The match notification (posted by the Action as a comment) has a different payload: `{matched_match_hash, greeting, pubkey}`. The client decrypts this directly using `crypto.Decrypt`, not via `matchcrypt decrypt`.

## Matches Screen Changes

```go
// Current
type MatchItem struct {
    BinHash  string
    Greeting string
}

// New
type MatchItem struct {
    MatchHash string
    Greeting  string              // optional — may be empty
    PubKey    ed25519.PublicKey
}
```

`decryptMatchNotification` extracts `{matched_match_hash, greeting, pubkey}` from the encrypted comment body.

**Greeting is optional.** TUI and CLI must handle empty greetings gracefully:
- Matches list: show `MatchHash[:16]` only (no quote) when greeting is empty
- Chat command: no greeting display if empty

## Files Deleted

| File | Reason |
|------|--------|
| `internal/relay/store.go` | No database |
| `internal/protocol/types.go` | No MessagePack frame types |
| `internal/protocol/codec.go` | No MessagePack encoding |

## Files Modified

| File | Changes |
|------|---------|
| `internal/relay/server.go` | Stateless auth, remove sync/match endpoints, add caches |
| `internal/relay/session.go` | Binary frame routing, text frame errors, no protocol import |
| `internal/relay/hub.go` | Re-key by match_hash, add offline queue |
| `internal/cli/relay/client.go` | id_hash connect, binary frames, remove key exchange |
| `internal/cli/chat.go` | `<match_hash>` target, no bin_hash |
| `internal/cli/tui/screens/matches.go` | MatchItem.MatchHash + PubKey |
| `internal/cli/tui/screens/chat.go` | Wire to match_hash, pass pubkey |
| `cmd/matchcrypt/main.go` | Updated encrypt payload + flags |
| `cmd/relay/main.go` | Remove DB_PATH, simplify |
| `templates/actions/pool-interest.yml` | Updated matchcrypt encrypt invocation |
| `internal/relay/relay_e2e_test.go` | Full rewrite |

## Testing

### Unit/Integration Tests (relay package)

Setup: in-process relay server with injectable pubkey + match caches (pre-populated, no GitHub fetches).

| Category | Tests |
|----------|-------|
| Auth | Valid chain + sig → upgrade; wrong sig → 401; mismatched chain → 401; missing params → 401; expired sig → 401 |
| Routing | Both online → message routed; not matched → error; sender match_hash prepended |
| Queue | Target offline → queued; target connects → flushed; cap enforced (21st rejected with "queue full") |
| Session | Reconnect replaces old session; multiple concurrent users |
| Cache | Pubkey fetched once and cached; match check cached with 5min TTL; 404 cached as negative; 5xx/network errors NOT cached |
| Health | `/health` returns `{status, online, queued}` |

### E2E Tests (full stack, real GitHub test pool)

#### CLI E2E

1. Start relay server pointed at real test pool repo
2. Two separate `DATING_HOME` dirs (tmpdir1, tmpdir2)
3. Two managed accounts registered in test pool (via `pool join`)
4. Wait for registration Actions to complete
5. Both express interest → wait for mutual match Action
6. Each receives match notification with peer's pubkey
7. User A: `dating chat <B_match_hash>` → connects to relay
8. User B: `dating chat <A_match_hash>` → connects to relay
9. A sends encrypted message → B receives + decrypts → verify plaintext
10. B sends encrypted message → A receives + decrypts → verify plaintext
11. Disconnect B → A sends → reconnect B → B receives queued message
12. Both disconnect cleanly

#### TUI E2E

1. Two `dating tui` instances with separate `DATING_HOME` dirs
2. Navigate to Matches screen → verify peer appears with greeting
3. Select match → enter Chat screen
4. Send message from A's TUI → verify B's TUI receives + displays
5. Send message from B's TUI → verify A's TUI receives + displays
6. Close B's TUI → send from A → reopen B → queued message appears
