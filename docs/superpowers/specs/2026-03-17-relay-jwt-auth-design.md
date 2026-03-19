# Relay Server: JWT Auth + REST Endpoints + Repo Sync

## Overview

Replace WebSocket challenge-response auth with REST-based JWT login. Add repo sync to populate user index. Relay server builds independently from CLI.

## Auth Flow

```
POST /login
{
  "user_id": "vutran1710",
  "provider": "github",
  "timestamp": 1710720000,
  "signature": "hex(ed25519.Sign(priv, 'user_id:provider:timestamp'))"
}

→ 200 {
  "token": "eyJ...",
  "bin_hash": "abc123...",
  "match_hash": "def456..."
}
```

- Timestamp is Unix epoch (timezone-neutral). Server rejects if `|now - timestamp| > 30s`.
- Server looks up user's pubkey from `.bin` file in synced repo, verifies signature.
- JWT contains: `bin_hash`, `match_hash`, `user_id`, `provider`, `pool_url`, `exp` (self-contained).
- JWT signed with HMAC-SHA256 using secret derived from operator private key.
- TTL: 15 minutes.

## REST Endpoints

| Endpoint | Method | Auth | Response |
|----------|--------|------|----------|
| `/health` | GET | No | `{"status":"ok","online":N}` |
| `/login` | POST | Signature | `{"token":"...","bin_hash":"...","match_hash":"..."}` |
| `/login/refresh` | POST | JWT | `{"token":"..."}` (new JWT) |
| `/me` | GET | JWT | `{"bin_hash":"...","match_hash":"...","user_id":"..."}` |

## WebSocket

- Connect: `ws://relay/ws?token=JWT`
- Server validates JWT on upgrade. Rejects if invalid/expired.
- No auth frames on WebSocket. Only: `msg`, `ack`, `key_request`, `key_response`, `error`.
- JWT validated once at connect time. Active connections stay trusted.

## Protocol Cleanup

Remove frame types: `auth`, `challenge`, `auth_response`, `authenticated`, `refresh`.
Remove structs: `AuthRequest`, `Challenge`, `AuthResponse`, `Authenticated`, `RefreshRequest`.
Remove: `token.go` (stateless JWT replaces token store).

Keep: `msg`, `ack`, `key_request`, `key_response`, `error`, `identity`, `identity_response`.

## Env Vars

| Var | Required | Description |
|-----|----------|-------------|
| `POOL_URL` | Yes | Pool repo (e.g. `owner/pool-name`) |
| `OPERATOR_PRIVATE_KEY` | Yes | Hex ed25519 private key |
| `POOL_SALT` | Yes | Secret salt for hash computation |
| `POOL_TOKEN` | Yes | GitHub PAT for repo access |
| `PORT` | No | Server port (default: 8081) |
| `TOKEN_TTL` | No | JWT lifetime seconds (default: 900) |
| `SYNC_INTERVAL` | No | Repo sync interval (default: 2m) |

## Repo Sync

Periodic sync of pool repo:
1. `git clone` (first time) or `git pull` (subsequent)
2. Read `users/*.bin` files
3. For each `.bin`: extract pubkey (first 32 bytes)
4. Compute bin_hash: `SHA256(salt:PublicID)[:16]` where PublicID from identity proof
5. Upsert to store: `{pubkey, user_id, provider, bin_hash, match_hash}`
6. Read `matches/*.json` files → populate match store

## Client Changes

On startup with active pool:
1. Auto `POST /login` to relay
2. Store JWT in memory
3. Background refresh every 13min via `POST /login/refresh`
4. WebSocket connects with `?token=JWT`
5. Remove `authenticate()` challenge-response from client

## Tests

### Happy paths
- Login with valid signature → JWT + hashes
- Login refresh with valid JWT → new JWT
- GET /me with valid JWT → user info
- WebSocket connect with valid JWT → session
- Message routing works after JWT auth
- E2E encrypted messages work
- Repo sync populates user index

### Sad paths
- Login with wrong signature → 401
- Login with expired timestamp → 401
- Login with unknown user → 404
- Login refresh with expired JWT → 401
- GET /me without token → 401
- GET /me with tampered JWT → 401
- WebSocket connect without token → rejected
- WebSocket connect with expired JWT → rejected
- WebSocket connect with tampered JWT → rejected
