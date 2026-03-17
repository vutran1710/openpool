# Per-Pool Relay Refactor

## Problem

The relay server is per-pool (one instance = one pool), but the client currently sends `pool_url` in the `AuthRequest` frame. This is redundant — the server already knows its pool. It also leaks the pool topology to clients unnecessarily.

## Changes

### 1. Server gets `POOL_URL` env var (required)

`ServerConfig` gains `PoolURL string`. Add `poolURL string` field to `Server` struct (stored from config at construction). `cmd/relay/main.go` reads `POOL_URL` from env. Server refuses to start if empty.

### 2. Remove `PoolURL` from `AuthRequest`

Before:
```
{ type: "auth", user_id: "alice", provider: "github", pool_url: "owner/pool" }
```

After:
```
{ type: "auth", user_id: "alice", provider: "github" }
```

### 3. Add `PoolURL` to `Authenticated` response

Client needs pool_url for `DeriveConversationKey`. Server sends it in both initial auth and token refresh responses.

Before:
```
{ type: "authenticated", token: "...", expires_at: 123, hash_id: "abc" }
```

After:
```
{ type: "authenticated", token: "...", expires_at: 123, hash_id: "abc", pool_url: "owner/pool" }
```

Both `handleAuthResponse` and `handleRefresh` send `Authenticated` frames — both must include `PoolURL`. Client ignores `PoolURL` on refresh frames (already stored from initial auth).

### 4. Session gets pool_url from server config

`session.go` reads `s.server.poolURL` instead of `req.PoolURL` during auth.

### 5. Store keys simplify

Before: `poolURL:userID` composite key.
After: `userID:provider` composite key. Pool is implicit (single pool per server).

This also fixes a latent bug: the old key `poolURL:userID` didn't include provider, so the same `userID` with different providers (e.g. GitHub "alice" vs Google "alice") would collide. The new `userID:provider` key prevents this.

Remove `poolURL` parameter from `LookupUser`, `LookupHashForPool`, `UpsertUser`. Remove `PoolURL` field from `UserEntry`.

### 6. Remove `IdentityRequest`/`IdentityResponse`

Single pool = the user's hash_id is already in the `Authenticated` response. No need for a separate identity query. Remove:
- `TypeIdentity`, `TypeIdentityResponse` constants
- `IdentityRequest`, `IdentityResponse` structs
- `DecodeFrame` cases for them
- `handleIdentity` in session.go
- `QueryIdentity` in client.go

### 7. Client learns pool_url from auth response

Remove `PoolURL` from client `Config`. Client stores `poolURL` from the `Authenticated` frame received during `authenticate()`. This is then used for `DeriveConversationKey`.

### 8. Message frame: client stops setting PoolURL, server populates it on forward

Client omits `PoolURL` when sending messages (removes `PoolURL: c.poolURL` from `SendMessage`/`SendMessagePlain`). Server fills it in from its config when forwarding to the target. Keeps field in the wire format for logging/tracking on the receiving end.

### 9. Clean up data model structs

Remove `PoolURL` from these data model structs (pool is implicit per-server):
- `UserIndex` — pool is the server itself
- `Match` — matches are within this server's pool
- `QueuedMessage` — queued within this server's pool
- `MatchWebhook` — keep `PoolURL` here for webhook origin verification (external callers need to specify which pool)

### 10. Remove dead error code

Remove `ErrPoolNotFound = "pool_not_found"` from protocol constants. With per-pool relay, there's no concept of "pool not found."

## Not affected

- `internal/cli/tui/screens/join.go`, `pools.go`, `internal/cli/registry.go` — these use local `poolURL` variables for git clone URLs, unrelated to the relay protocol.

## File Changes

| File | Changes |
|------|---------|
| `internal/protocol/types.go` | Remove `PoolURL` from `AuthRequest`, `UserIndex`, `Match`, `QueuedMessage`. Add `PoolURL` to `Authenticated`. Remove `IdentityRequest`/`IdentityResponse` structs + constants. Remove `ErrPoolNotFound`. |
| `internal/protocol/codec.go` | Remove `TypeIdentity`/`TypeIdentityResponse` from `DecodeFrame` switch. |
| `internal/protocol/codec_test.go` | Update tests: remove identity frame tests, update auth/authenticated tests. |
| `internal/relay/server.go` | Add `PoolURL` to `ServerConfig`. Add `poolURL string` field to `Server` struct. |
| `internal/relay/session.go` | Use `s.server.poolURL` in auth + refresh + message forwarding. Remove `handleIdentity`. Both auth and refresh `Authenticated` frames include `PoolURL`. |
| `internal/relay/store.go` | Rekey to `userID:provider`. Remove `PoolURL` from `UserEntry` and method signatures. |
| `internal/relay/token.go` | Remove `poolURL` from `tokenEntry`, `Issue`, `Validate`. `Validate` returns `(hashID string, ok bool)`. |
| `internal/cli/relay/client.go` | Remove `PoolURL` from `Config`. Store `poolURL` from `Authenticated` response. Remove `QueryIdentity`. Stop setting `PoolURL` on outbound messages. |
| `internal/cli/relay/client_test.go` | Update tests for no-PoolURL config. |
| `internal/cli/relay/client_integration_test.go` | Update mock relay `doAuth` + all `Authenticated` frames to include `PoolURL`. |
| `internal/relay/relay_e2e_test.go` | Update test helpers for new signatures. Remove `TestE2E_MultiPool_Isolation` (meaningless for single-pool). |
| `cmd/relay/main.go` | Read `POOL_URL` env var, pass to `ServerConfig`. |
| `docs/relay-protocol.md` | Update auth flow, remove identity query section, update env vars. |
| `README.md` | Replace `POOL_REPOS` with `POOL_URL` in relay env vars. |

## Testing

- All existing E2E tests updated for new signatures
- `TestE2E_MultiPool_Isolation` removed (replaced by startup validation test)
- Auth flow works without pool_url in request
- Client receives pool_url in Authenticated response
- E2E encrypted messages still work (pool_url from auth response used in key derivation)
- Token refresh includes PoolURL in response
- Server refuses to start without POOL_URL
