# Interest, Matching & Chat

**Status**: Spec

---

## Summary

Users express interest via GitHub PRs, a GitHub Action detects mutual interest and creates matches, and matched users chat via a Cloudflare Workers relay with E2E encryption.

Six phases, each independently testable:

1. **Like** — create interest PR
2. **Inbox** — view incoming interests
3. **Mutual Match Action** — detect reciprocal, commit match, deliver bin_hash
4. **Match Result** — poll PR for match notification
5. **Cloudflare Relay** — WebSocket service on Workers + Durable Objects
6. **Chat** — connect to relay, E2E encrypted messaging

---

## Phase 1: Like (Interest PR)

### CLI

```
dating like <match_hash> [--greeting "message"] [--pool <name>]
```

Creates a PR in the pool repo:
```
Title:  <target_match_hash>
Label:  interest
Branch: like/<sha256(my_match_hash + target_match_hash)[:12]>
Body:   <base64 encrypted {author_bin_hash, author_match_hash, greeting}>
```

The body is encrypted with `crypto.Encrypt(operatorPub, payload)` — ephemeral NaCl box. Only the operator/Action can decrypt.

### TUI

From the Discover screen, pressing `[l]` on a suggestion:
1. Optional greeting prompt (text input, can be empty)
2. Creates the interest PR
3. Toast: "Interest sent!"

### Privacy

- PR author (GitHub username) is visible
- Target match_hash is the title — but match_hashes are not publicly linkable to identities
- Body is encrypted — greeting + author's bin_hash hidden

### What Gets Created

```
PR #42
  title: "f19d63743e063cfc"
  label: interest
  branch: like/a3b4c5d6e7f8
  body: "SGVsbG8gd29ybGQ..." (base64 encrypted blob)
```

---

## Phase 2: Inbox

### CLI

```
dating inbox [--pool <name>]
```

Searches open PRs with `label:interest title:<my_match_hash>`.
Shows count: "You have 3 new interests."

No identities revealed — just a count. User doesn't know who likes them until they like back (mutual match).

### TUI

Inbox screen shows:
- Count of open interest PRs targeting the user
- No details — just the number
- Hint: "Browse Discover to find your match"

---

## Phase 3: Mutual Match Action

### GitHub Action

Trigger: `pull_request: [opened]` with label `interest`.

Flow:
1. Decrypt current PR body → get `{author_bin_hash, author_match_hash, greeting}`
2. Search open PRs: `label:interest title:<author_match_hash>` — is anyone targeting the current author?
3. **Not found** → leave PR open. No match yet.
4. **Found** → mutual interest:

### On Mutual Match

Given:
- PR A: Alice targeting Bob (body has `{alice_bin_hash, alice_match_hash, alice_greeting}`)
- PR B: Bob targeting Alice (body has `{bob_bin_hash, bob_match_hash, bob_greeting}`)

Action:
1. Read Alice's pubkey from `users/{alice_bin_hash}.bin` (first 32 bytes)
2. Read Bob's pubkey from `users/{bob_bin_hash}.bin` (first 32 bytes)
3. Commit `matches/{hash(sort(alice_match_hash, bob_match_hash))}.json`
   - Content: `{"match_hash_1": "...", "match_hash_2": "...", "created_at": timestamp}`
4. Post encrypted comment on PR A (Alice's):
   - `crypto.Encrypt(alice_pub, {matched_bin_hash: bob_bin_hash, greeting: bob_greeting})`
5. Post encrypted comment on PR B (Bob's):
   - `crypto.Encrypt(bob_pub, {matched_bin_hash: alice_bin_hash, greeting: alice_greeting})`
6. Close both PRs
7. Notify relay: `POST /match` (Phase 5)

### Action Template

```
templates/actions/pool-interest.yml
```

Trigger: PR opened with label `interest`.
Downloads `regcrypt` binary (for the `Encrypt` function — or a new `matchcrypt` tool).

### Match File

```
matches/{hash(sort(match_hash_1, match_hash_2))}.json
```

Public existence confirms a match happened. Content is minimal — just the two match_hashes + timestamp.

---

## Phase 4: Match Result

### CLI

After creating a like PR, the CLI can poll for match:

```
dating matches [--pool <name>]
```

Lists the user's closed interest PRs that have encrypted comments from the Action.
For each: decrypt → extract `{matched_bin_hash, greeting}` → display.

### TUI

- Background poll: check user's interest PRs for match comments
- On match found: toast notification "You matched!"
- Matches screen shows list of matches with greeting + bin_hash
- Hint: "dating chat <bin_hash>" to start chatting

### Polling

Same pattern as registration: poll PR comments for `github-actions[bot]` comment, decrypt with user's privkey.

---

## Phase 5: Cloudflare Relay

### Architecture

```
Cloudflare Worker (edge)
  ↓ handles /ws?bin=<bin_hash>&sig=<sig>
  ↓ TOTP verify
  ↓ routes to Durable Object

Durable Object (per-user session)
  ↓ holds WebSocket connection
  ↓ receives messages
  ↓ routes to target user's DO

KV Store
  ↓ bin_hash → {pubkey, match_hash} (user index)
  ↓ match pairs (for authorization)
```

### Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/ws` | GET (upgrade) | WebSocket connect with TOTP auth |
| `/health` | GET | Health check |
| `/match` | POST | Action notifies relay of new match |
| `/sync` | POST | Sync user index from pool repo |

### WebSocket Upgrade (`/ws`)

Query params: `bin=<bin_hash>&sig=<hex_totp_signature>`

Worker:
1. Look up `bin_hash` in KV → get `pubkey` + `match_hash`
2. `TOTPVerify(bin_hash, match_hash, sig, pubkey)` — same algorithm as Go implementation
3. If valid → get/create Durable Object for this `bin_hash`
4. Forward WebSocket to the DO

### Durable Object (UserSession)

Each connected user gets a DO instance keyed by `bin_hash`.

State:
- Active WebSocket connection
- List of matched bin_hashes (from KV)

Handles:
- Incoming messages: verify sender matches session, check match, route to target DO
- Key requests: return target's pubkey from KV
- Disconnection: cleanup

### Message Protocol

Same MessagePack protocol as the Go relay:
- `msg` — chat message (source_hash, target_hash, body, encrypted flag)
- `ack` — message acknowledgment
- `key_request` / `key_response` — E2E key exchange
- `error` — error responses

### Match Notification (`POST /match`)

Called by the GitHub Action on mutual match:
```json
POST /match
Authorization: HMAC-SHA256(pool_salt, body)
{
  "bin_hash_1": "...",
  "bin_hash_2": "..."
}
```

Worker:
1. Verify HMAC
2. Store match pair in KV
3. If either user is connected, notify their DO

### User Index Sync (`POST /sync`)

Called periodically (or by Action) to sync user data:
```json
POST /sync
Authorization: HMAC-SHA256(pool_salt, body)
{
  "bin_hash": "...",
  "pubkey": "...",
  "match_hash": "..."
}
```

### Technology

- **Cloudflare Workers** — HTTP handling, TOTP verification
- **Durable Objects** — WebSocket session management, per-user state
- **KV** — user index, match pairs
- **wrangler** — deployment CLI

### Project Structure

```
relay/
  src/
    index.ts        — Worker entry (routes, TOTP verify, upgrade)
    session.ts      — Durable Object (WebSocket handler)
    protocol.ts     — MessagePack encode/decode
    totp.ts         — TOTP sign/verify (ed25519 + SHA-256)
    kv.ts           — KV operations (user lookup, match check)
  wrangler.toml     — Cloudflare config
  package.json
```

### Environment Variables (Worker Secrets)

| Secret | Purpose |
|--------|---------|
| `POOL_SALT` | TOTP verification + HMAC auth |
| `POOL_URL` | Pool identifier |

---

## Phase 6: Chat

### CLI

```
dating chat <bin_hash> [--pool <name>]
```

1. Compute TOTP signature
2. Connect to Cloudflare relay: `wss://relay.dating.dev/ws?bin=<bin_hash>&sig=<sig>`
3. Key exchange via `key_request` / `key_response`
4. Derive conversation key (ECDH + HKDF)
5. Send/receive encrypted messages
6. Display in terminal

### TUI

Chat screen (already exists, needs wiring to real relay):
- Connect on enter
- Show messages with timestamps
- Text input at bottom
- E2E encryption transparent to UI

### E2E Encryption

Same as current Go implementation:
1. Request peer's ed25519 pubkey via `key_request`
2. ECDH shared secret (ed25519 → curve25519)
3. HKDF derive conversation key
4. NaCl secretbox seal/open per message

---

## File Summary

| File | Phase | Purpose |
|------|-------|---------|
| `internal/cli/like.go` | 1 | Like command (refactored) |
| `internal/cli/tui/screens/discover.go` | 1 | Like button in TUI |
| `internal/cli/inbox.go` | 2 | Inbox command (refactored) |
| `internal/cli/tui/screens/inbox.go` | 2 | Inbox screen (updated) |
| `templates/actions/pool-interest.yml` | 3 | Mutual match Action |
| `cmd/matchcrypt/main.go` | 3 | Encrypt match notifications for Action |
| `internal/cli/matches.go` | 4 | Matches command (polls for results) |
| `internal/cli/tui/screens/matches.go` | 4 | Matches screen (updated) |
| `relay/` | 5 | Cloudflare Workers relay project |
| `internal/cli/chat.go` | 6 | Chat command (connects to CF relay) |
| `internal/cli/tui/screens/chat.go` | 6 | Chat screen (connects to CF relay) |

---

## Implementation Order

Phase 1 → 2 → 3 → 4 can proceed sequentially.
Phase 5 can be developed in parallel with 1-4.
Phase 6 requires Phase 4 + 5 complete.
