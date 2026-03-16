# Relay Protocol Specification

## Overview

The relay is a WebSocket server that authenticates users, routes chat messages between matched users, and queues undelivered messages. It is defined as a protocol — any language can implement it.

Wire format: **MessagePack** over WebSocket (binary frames).

---

## 1. Connection

Client connects via WebSocket:
```
wss://relay.example.com/ws
```

All frames are MessagePack-encoded. Each frame has a `type` field.

---

## 2. Authentication

### Step 1: Auth Request

Client sends:
```
{
  type: "auth",
  user_id: "vutran1710",      // native ID (GitHub login)
  provider: "github",
  pool_url: "vutran1710/dating-test-pool"
}
```

### Step 2: Challenge

Relay looks up user in its index (synced from pool repo). If found, sends:
```
{
  type: "challenge",
  nonce: "a1b2c3d4..."       // random 32-byte hex string
}
```

If user not found:
```
{
  type: "error",
  code: "user_not_found",
  message: "User not registered in this pool"
}
```

### Step 3: Challenge Response

Client signs the nonce with their ed25519 private key and sends:
```
{
  type: "auth_response",
  signature: "hex-encoded ed25519 signature of nonce bytes"
}
```

### Step 4: Token

Relay verifies signature against the user's pubkey (from the `.bin` file in the repo). If valid:
```
{
  type: "authenticated",
  token: "short-lived-jwt-or-opaque-token",
  expires_at: 1710720000,     // unix timestamp
  hash_id: "fef9b374b0d6f4ad" // user's hash in this pool
}
```

If invalid:
```
{
  type: "error",
  code: "auth_failed",
  message: "Signature verification failed"
}
```

### Token Refresh

Client must refresh before `expires_at`. Send on the same WebSocket:
```
{
  type: "refresh",
  token: "current-token"
}
```

Relay responds with new token:
```
{
  type: "authenticated",
  token: "new-token",
  expires_at: 1710723600,
  hash_id: "fef9b374b0d6f4ad"
}
```

### Token Lifetime

- Default: 15 minutes
- Refresh window: last 2 minutes before expiry
- Max session: 24 hours (must re-authenticate)

---

## 3. Identity Query

Client can query their hash_id for any pool they're registered in:
```
{
  type: "identity",
  pool_url: "vutran1710/dating-test-pool"
}
```

Response:
```
{
  type: "identity_response",
  pool_url: "vutran1710/dating-test-pool",
  hash_id: "fef9b374b0d6f4ad"
}
```

Requires valid token.

---

## 4. Chat Messages

### Send Message

```
{
  type: "msg",
  token: "current-token",
  source_hash: "fef9b374b0d6f4ad",
  target_hash: "8d419fa9098bdec3",
  pool_url: "vutran1710/dating-test-pool",
  body: "hello!",
  ts: 1710720000
}
```

### Receive Message

```
{
  type: "msg",
  source_hash: "8d419fa9098bdec3",
  target_hash: "fef9b374b0d6f4ad",
  pool_url: "vutran1710/dating-test-pool",
  body: "hello!",
  ts: 1710720000,
  msg_id: "uuid-v4"
}
```

### Message Delivery

- **Online**: routed immediately via WebSocket
- **Offline**: queued in durable storage
- **Reconnect**: queued messages delivered on next authentication
- **Ack**: client sends ack after receiving

```
{
  type: "ack",
  msg_id: "uuid-v4"
}
```

### Delivery Rules

- Relay checks that `source_hash` and `target_hash` are a valid match
- Relay checks token belongs to `source_hash`
- Unmatched users cannot message each other

---

## 5. Match Registration

### Via Webhook

When a match PR is merged on the pool repo, a GitHub webhook notifies the relay:

```
POST /webhook/match
Content-Type: application/msgpack

{
  pool_url: "vutran1710/dating-test-pool",
  hash_id_1: "fef9b374b0d6f4ad",
  hash_id_2: "8d419fa9098bdec3"
}
```

Relay stores the match and enables routing between the two users.

### Via Sync

Alternatively, the relay can discover matches by syncing the pool repo's `matches/` directory during its periodic sync.

---

## 6. Repo Sync

The relay periodically syncs each pool repo to maintain its user index.

### Sync Interval

- Default: every 2 minutes
- Smart sync: compare HEAD commit before pulling
- Fallback: compare user count in `users/` directory

### Sync Process

1. `git ls-remote` → check if HEAD changed
2. If changed: `git pull`
3. Read `users/*.bin` files
4. For each `.bin`: extract pubkey (first 32 bytes)
5. Compute hash_id: `SHA256(pool_salt:pool_url:github:user_id)[:16]`
6. Note: relay needs `POOL_SALT` and the operator private key to decrypt identity
7. Upsert to index: `{ pool_url, pubkey, user_id, provider, hash_id }`

### Discovering User Identity from .bin

The `.bin` file contains `[32B pubkey][encrypted profile]`. The relay:
1. Extracts pubkey from first 32 bytes (no decryption needed)
2. Decrypts profile with operator private key to get `display_name`, etc.
3. But for auth, only the pubkey is needed — no decryption required

---

## 7. Error Frames

All errors follow the same format:
```
{
  type: "error",
  code: "error_code",
  message: "Human-readable description"
}
```

### Error Codes

| Code | Description |
|------|-------------|
| `user_not_found` | User not in pool index |
| `auth_failed` | Signature verification failed |
| `token_expired` | Token has expired |
| `token_invalid` | Token is malformed or revoked |
| `not_matched` | Users are not matched, cannot message |
| `rate_limited` | Too many requests |
| `pool_not_found` | Pool URL not recognized |
| `internal_error` | Server-side error |

---

## 8. Data Model

### User Index

```
users {
  pool_url:  string    -- which pool
  pubkey:    bytes[32] -- ed25519 public key
  user_id:   string    -- native ID (e.g. GitHub login)
  provider:  string    -- "github"
  hash_id:   string    -- SHA256(salt:pool:provider:user_id)[:16]
}
```

### Matches

```
matches {
  pool_url:   string
  hash_id_1:  string
  hash_id_2:  string
  created_at: timestamp
}
```

### Message Queue

```
messages {
  msg_id:      uuid
  pool_url:    string
  source_hash: string
  target_hash: string
  body:        bytes    -- MessagePack-encoded payload
  ts:          timestamp
  delivered:   bool
  acked:       bool
}
```

### Active Connections (in-memory)

```
routes {
  hash_id → WebSocket connection
}
```

---

## 9. Frame Types Summary

| Type | Direction | Auth Required | Description |
|------|-----------|---------------|-------------|
| `auth` | client→relay | no | Start authentication |
| `challenge` | relay→client | no | Nonce challenge |
| `auth_response` | client→relay | no | Signed nonce |
| `authenticated` | relay→client | no | Token + hash_id |
| `refresh` | client→relay | yes | Refresh token |
| `identity` | client→relay | yes | Query hash_id for a pool |
| `identity_response` | relay→client | yes | Hash_id result |
| `msg` | bidirectional | yes | Chat message |
| `ack` | client→relay | yes | Message delivery acknowledgment |
| `error` | relay→client | no | Error response |

---

## 10. Implementation Notes

### Go Native

- WebSocket: `gorilla/websocket`
- MessagePack: `vmihailenco/msgpack`
- Storage: SQLite (user index, matches, message queue)
- In-memory: connection routing map

### Cloudflare Workers

- WebSocket: Durable Objects (per-pool actor)
- MessagePack: `@msgpack/msgpack` (JS)
- Storage: KV (user index), Durable Object storage (matches, messages)
- Routing: Durable Object state

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPERATOR_PRIVATE_KEY` | Yes | Hex-encoded ed25519 private key |
| `POOL_SALT` | Yes | Secret salt for hash_id computation |
| `POOL_REPOS` | Yes | Comma-separated list of pool repo URLs to sync |
| `SYNC_INTERVAL` | No | Repo sync interval (default: 2m) |
| `TOKEN_TTL` | No | Token lifetime in seconds (default: 900) |
| `PORT` | No | Server port (default: 8081) |
