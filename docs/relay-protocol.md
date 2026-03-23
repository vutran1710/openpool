# Relay Protocol

## Overview

The relay is a stateless WebSocket server that authenticates users via TOTP, routes E2E encrypted chat messages between matched users, and queues messages for offline peers. No database — fully in-memory.

## Connection

```
ws(s)://relay.example.com/ws?id=<id_hash>&match=<match_hash>&sig=<totp_sig>
```

Authentication happens at connection time via query parameters. No login endpoint, no tokens, no session state.

## Authentication (TOTP)

The relay verifies three things at connect:

1. **Hash chain**: `bin_hash = sha256(salt:id_hash)[:16]`, `match_hash = sha256(salt:bin_hash)[:16]` — must match the submitted `match_hash`
2. **Pubkey**: fetched from `users/<bin_hash>.bin` via raw.githubusercontent.com (cached 5 min)
3. **TOTP signature**: `ed25519.Verify(pubkey, sha256(time_window + relay_host), sig)` — ±1 window (5 min) for clock drift

Channel-bound to the relay host to prevent replay across relays.

## Wire Format

### Binary frames (chat messages)

```
Client → Relay:  [target_match_hash 8B][ciphertext]
Relay → Client:  [sender_match_hash 8B][ciphertext]
```

- Target is the first 8 bytes of the recipient's match_hash (hex-encoded)
- Relay prepends the sender's match_hash when forwarding
- Ciphertext is NaCl secretbox (ECDH-derived key)
- Max frame size: 8 KB

### Text frames (control messages)

Relay → Client only:

| Message | Meaning |
|---------|---------|
| `queued` | Message queued for offline recipient |
| `not matched` | No match file found for this pair |
| `match check failed` | Error checking match authorization |
| `queue full` | Offline queue at capacity (20 messages) |
| `frame too large` | Binary frame exceeds 8 KB |
| `unknown user` | Target match_hash not recognized |

## Match Authorization

Before routing a message, the relay checks for a match file:

```
matches/<pair_hash>.json
```

Where `pair_hash = sha256(min(a,b) + ":" + max(a,b))[:12]` using both users' match_hashes.

Fetched from raw.githubusercontent.com. Cached for 5 minutes (positive + 404 negative). 5xx/network errors NOT cached.

## Offline Queue

- Up to 20 messages queued per user
- Flushed on reconnect
- No persistence (lost on relay restart)

## Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /ws` | TOTP auth + WebSocket upgrade |
| `GET /health` | `{"status":"ok","online":N,"queued":N}` |

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `POOL_URL` | Yes | Pool repo (`owner/repo`) |
| `POOL_SALT` | Yes | Secret salt for hash computation |
| `PORT` | No | Server port (default: `8081`) |

## Deployment

Single binary: `go run ./cmd/relay/`

Deployed on Railway (~$0.40/month). One relay instance per pool.
