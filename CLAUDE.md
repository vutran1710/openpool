# CLAUDE.md

## Project Overview

Terminal-native, decentralized matching platform. GitHub repos as the database, encrypted profiles, CLI + TUI interface. Pre-launch (private repo). Planned rename to **openpool**.

## Tech Stack

- **Language**: Go 1.25
- **CLI**: Cobra (commands), Bubbletea (TUI), Lipgloss (styling)
- **Crypto**: ed25519 (signing/TOTP) + NaCl box with ed25519→curve25519 (encryption) + ECDH (chat keys) + AES-256-GCM (chain encryption for discovery)
- **Config**: TOML (`~/.dating/setting.toml`)
- **Protocol**: Binary frames over WebSocket (text frames for control)
- **Schema**: YAML (`pool.yaml`) — pool config, profile attributes, roles, indexing, interest expiry

## Architecture

```
cmd/dating/       → CLI entry point
cmd/relay/        → Per-pool WebSocket relay server
cmd/action-tool/  → Unified Action binary (register, match, squash, index, sign, decrypt, pubkey, managed-register)
internal/
  bucket/         → Profile bucketing (partition by attributes)
  chainenc/       → Chain encryption (AES-256-GCM, rate-limited discovery)
  cli/            → CLI commands + TUI app
  cli/chat/       → ChatClient + ConversationDB (SQLite message persistence)
  cli/svc/        → Service interfaces (dependency injection)
  cli/tui/        → Bubbletea screens, components, theme
  cli/config/     → Config file management
  crypto/         → Encryption, signing, TOTP, ephemeral hashes, key derivation
  explorer/       → Chain-encrypted index explorer (grind, state tracking)
  github/         → GitHubClient interface (CLIClient + HTTPClient), pool operations
  gitrepo/        → Git clone management, raw content fetcher
  indexer/        → Chain-encrypted index builder (buckets → chains → SQLite)
  limits/         → Payload size constants
  message/        → Structured message format (Format/Parse for openpool blocks)
  pooldb/         → PoolDB (SQLite for legacy pool profiles)
  relay/          → Relay server (stateless TOTP auth, hub, sessions, GitHub cache)
  schema/         → Pool schema parser (pool.yaml), validation, form generation
  debug/          → Debug logger (DEBUG=1)
templates/
  actions/        → GitHub Action templates for pool repos (thin wrappers)
```

## Key Concepts

### Hash Chain

```
real_id    = github:username                        → real identity (private, never leaves client)
id_hash    = sha256(pool_url:provider:user_id)     → 64 hex chars (registration identity)
bin_hash   = sha256(salt:id_hash)[:16]             → 16 hex chars (relay routing key, public in discovery)
match_hash = sha256(salt:bin_hash)[:16]             → 16 hex chars (TOTP secret, public handle for chat)
```

Salt is a pool secret (only in GitHub Actions secrets + relay env var).

### Security Rationale

- **bin_hash is public** — visible in `.bin` filenames
- **match_hash is public** — used in interest issue titles (ephemeral hash)
- **Unlinkable without salt** — knowing bin_hash tells you nothing about match_hash
- discovery (bin_hash), matching (match_hash), and identity (id_hash) are three unlinkable namespaces

### Pool Schema (`pool.yaml`)

Single YAML file per pool repo with metadata, profile attributes, roles, indexing config, and interest expiry:

```yaml
name: "pool name"
description: "description"
relay_url: wss://relay.example.com
operator_public_key: hex...
interest_expiry: 3d           # required — ephemeral interest hash rotation

profile:
  age: { type: range, min: 18, max: 100 }
  interests: { type: multi, values: hiking, coding, music }
  about: { type: text, required: false }
  phone: { type: text, visibility: private }

roles:
  - man
  - woman

indexing:
  partitions:
    - field: role
    - field: age
      step: 5
      overlap: 2
  permutations: 5
  difficulty: 20
```

### Chain Encryption (Discovery)

Profiles are chain-encrypted for rate-limited discovery:

- **Indexer** (Action cron): decrypts .bin files → buckets by partitions → builds VDF-like chains per bucket → produces `index.db`
- **Explorer** (client): downloads `index.db` → grinds chains to unlock profiles one at a time
- **Three security tiers**: without hint = infeasible (10^8 × 10^4 × N × 6), cold = seconds, warm = instant
- **Key**: `sha256("{hint}:{profile_constant}:{nonce}")` with randomized component ordering
- **AES-256-GCM** authenticated encryption per profile entry

### Short-Lived Interest Hashes

Interest issue titles use ephemeral hashes that rotate based on `interest_expiry`:

```
window = unix_timestamp / expiry_seconds
ephemeral_hash = sha256(match_hash + ":" + window)[:16]
```

Real `target_match_hash` is inside the encrypted issue body.

### Relay Auth (TOTP)

No JWT, no tokens, no login endpoint, no database:

```
ws://relay/ws?id=<id_hash>&match=<match_hash>&sig=<totp_signature>
```

Channel-bound to the relay host. ±1 window (5 min) for clock drift.

### Registration Flow

1. TUI pool onboard → schema-driven form → fill profile → submit
2. GitHub Issue created with `registration` label + encrypted profile blob
3. Action runs `action-tool register --schema pool.yaml` → validates, computes hashes, commits `.bin`, posts operator-signed comment
4. Client polls issue → verifies signature → decrypts → persists hashes

### Discovery Flow

1. Indexer Action (cron) builds chain-encrypted `index.db` → uploads as release asset
2. Pools screen syncs: pull repo + download `index.db`
3. Discover screen reads local `index.db` → grinds chains → shows profiles one at a time
4. User likes → creates interest issue with ephemeral hash title

### E2E Encrypted Chat

Messages encrypted with NaCl secretbox. Key derived via ECDH (ed25519→curve25519) + HKDF. Relay routes ciphertext only. Match notification IS the key exchange (peer pubkey delivered in encrypted match comment).

## Conventions

### Code Style

- No unnecessary abstractions — three similar lines is better than a premature helper
- Single responsibility — functions do one thing
- Separate view from logic — TUI screens are pure views, logic in dedicated classes
- No backward compatibility during pre-launch — delete old code, don't maintain fallbacks
- CLI commands must be fully scriptable

### Naming

- Files: `snake_case.go`, tests: `snake_case_test.go`
- Hash types: `IDHash`, `BinHash`, `MatchHash`

### Error Handling

- Return errors, don't panic
- Wrap with context: `fmt.Errorf("doing X: %w", err)`
- In TUI: show error in UI (toast), don't crash

## Testing

```bash
make test      # runs with isolated DATING_HOME (temp dir)
make coverage  # coverage report
```

**CRITICAL**: Always use `make test` or set `DATING_HOME` to a temp dir.

## Binaries

| Binary | Purpose | Distribution |
|--------|---------|-------------|
| `dating` | CLI app | End users |
| `relay` | Per-pool relay server | Pool operators (Railway) |
| `action-tool` | All Action operations (register, match, squash, index, managed-register, sign, decrypt, pubkey) | GitHub Actions |

## GitHub Actions (Pool Repo)

| Action | Trigger | Purpose |
|--------|---------|---------|
| `pool-register.yml` | Issue with `registration` label | `action-tool register --schema pool.yaml` |
| `pool-interest.yml` | Issue with `interest` label | `action-tool match` |
| `pool-indexer.yml` | Cron (every 5 min) | `action-tool index --upload` (chain-encrypted) |
| `pool-squash.yml` | Cron (hourly) | `action-tool squash` |

## Local Databases

```
~/.dating/
  conversations.db                    — messages, conversations, peer_keys (ChatClient)
  pools/<pool-name>/
    index.db                          — chain-encrypted profiles (downloaded from release)
    explorer.db                       — explorer state (constants, checkpoints, seen)
    profile.json                      — user's profile for this pool
    preferences.yaml                  — local discovery preferences (never committed)
```

## Debug Mode

```bash
DEBUG=1 dating
```

## Common Gotchas

- **Config corruption**: Tests MUST use `DATING_HOME` temp dir
- **Key events eaten**: Command input steals arrow/tab keys — block forwarding for interactive screens
- **CDN cache**: raw.githubusercontent.com caches for ~5 min — relay may have stale pubkeys after .bin update
