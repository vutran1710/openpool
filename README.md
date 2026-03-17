# dating.dev

Decentralized, terminal-native dating. GitHub repos as the database, encrypted profiles, and the command line.

## Philosophy

- **GitHub is the database** — pools are repos, registration is an issue, matches are merged PRs
- **No central server owns your data** — your profile is encrypted, only the pool operator can decrypt it
- **Terminal-first** — TUI for everyday use, CLI flags for automation
- **Single key pair** — one ed25519 keypair handles signing, encryption (via curve25519), and identity
- **Zero trust relay** — relay routes messages but can't read them (E2E encrypted)

## Architecture

```
┌──────────┐       ┌──────────────────┐       ┌─────────────┐
│  CLI     │──────▶│  Pool Repo       │       │  Registry   │
│  (Go)    │       │  (GitHub)        │◀─────▶│  (GitHub)   │
│          │       │  users/{h}.bin   │       └─────────────┘
│          │       │  matches/        │
│          │       │  relationships/  │
│          │       └──────────────────┘
│          │              ▲
│          │──────▶┌──────┴───────────┐
│          │       │  Relay Server    │
└──────────┘       │  (per-pool)      │
                   └──────────────────┘
```

- **Pools** — GitHub repos, like Discord servers but decentralized
- **Profiles** — encrypted to the operator's pubkey, stored as `.bin` files
- **Registration** — GitHub Issue → Action computes hashes, posts encrypted reply, commits `.bin`
- **Relay** — per-pool WebSocket server. TOTP auth, E2E encrypted chat, profile discovery
- **CLI** — the only user interface. TUI for interactive use, flags for scripting

## Install

```bash
# From source
git clone https://github.com/vutran1710/dating-dev
cd dating-dev
make build    # builds bin/dating + bin/relay + bin/regcrypt
```

## Quick Start

```bash
# Set up registry
dating registry add owner/dating-registry

# Create a profile for a pool
dating profile create berlin-singles
# Edit ~/.dating/pools/berlin-singles/profile.json with your editor

# Join the pool (submits registration, polls for hashes)
dating pool join berlin-singles

# Discover profiles
dating fetch

# Like someone
dating like abc123def

# Check inbox
dating inbox

# Chat with a match (E2E encrypted)
dating chat abc123def
```

## CLI Commands

```
Profile
  dating profile create <pool>   Create local profile for a pool
  dating profile edit <pool>     Show profile path for editing
  dating profile show <pool>     Display profile in terminal

Pools
  dating pool browse             Browse available pools from registry
  dating pool join <name>        Submit registration (reads profile, polls for hashes)
    --profile <path>               Use custom profile JSON
    --no-wait                      Submit without polling for hashes
  dating pool list               List joined pools (auto-checks pending registrations)
  dating pool switch <name>      Set active pool
  dating pool leave <name>       Leave a pool

Discovery & Matching
  dating fetch                   Discover profiles (via relay)
  dating like <hash>             Express interest (creates issue)
  dating inbox                   View incoming interests
  dating accept <pr>             Accept interest (match)

Chat
  dating chat <hash>             Chat with a match (E2E encrypted via relay)

Relationships
  dating commit propose <hash>   Propose a relationship
  dating commit proposals        View incoming proposals
  dating commit accept <pr>      Accept proposal

Auth
  dating auth whoami             Show current identity
```

## How It Works

### Hash Derivation

Three hashes derived from a user's identity, each adding a layer of privacy:

| Hash | Derivation | Purpose |
|------|-----------|---------|
| `id_hash` | `sha256(pool_url:provider:user_id)` — 64 hex | Public identity for registration |
| `bin_hash` | `sha256(salt:id_hash)[:16]` — 16 hex | Relay routing key, `.bin` filename |
| `match_hash` | `sha256(salt:bin_hash)[:16]` — 16 hex | TOTP auth shared secret |

The salt is a pool secret (GitHub Actions secret). Without it, you can't derive `bin_hash` from `id_hash`.

### Registration

1. User creates profile locally: `dating profile create <pool>`
2. User submits: `dating pool join <pool>` → creates GitHub Issue with encrypted profile blob
3. GitHub Action runs `regcrypt` → computes `id_hash` → `bin_hash` → `match_hash`
4. Action encrypts `{bin_hash, match_hash}` to user's pubkey (ephemeral NaCl box)
5. Action posts encrypted blob as issue comment
6. Action commits `users/{bin_hash}.bin` to pool repo, closes issue
7. CLI polls issue comments, decrypts, persists hashes to local config

### Relay Auth (TOTP)

No login endpoint. No JWT. No tokens. Stateless.

```
time_window = floor(unix_timestamp / 300)
message     = sha256(bin_hash + match_hash + time_window)
sig         = ed25519.Sign(priv_key, message)

ws://relay/ws?bin=<bin_hash>&sig=<hex(sig)>
```

Relay verifies inline at WebSocket upgrade — checks ±1 time window for clock drift.

### E2E Encrypted Chat

Messages are encrypted with NaCl secretbox using a conversation key derived via:
1. ECDH shared secret (ed25519→curve25519 conversion)
2. HKDF key derivation with pool context

The relay routes ciphertext — it cannot read message content.

## Binaries

| Binary | Purpose |
|--------|---------|
| `dating` | CLI app (`cmd/dating/`) |
| `relay` | Per-pool WebSocket relay server (`cmd/relay/`) |
| `regcrypt` | Registration hash computation for GitHub Actions (`cmd/regcrypt/`) |

## Pool Structure

```
pool-repo/
  pool.json                    Pool metadata + operator public key
  users/{bin_hash}.bin         [32B ed25519 pubkey][profile encrypted to operator]
  matches/{pair_hash}/         Matched pairs
  relationships/{pair_hash}/   Committed relationships
  .github/workflows/
    register.yml               Registration Action (uses regcrypt)
```

Template Action: `templates/actions/pool-register.yml`

## Security

| Layer | Mechanism |
|-------|-----------|
| Identity | ed25519 key pairs, generated locally |
| Signing | ed25519 signatures |
| Encryption | NaCl box (Curve25519 + XSalsa20-Poly1305) with ed25519→curve25519 |
| Hash derivation | `id_hash` → `bin_hash` → `match_hash` (salt-protected) |
| Profile data | Encrypted to operator pubkey — only relay decrypts |
| Relay auth | TOTP: `ed25519.Sign(sha256(bin_hash + match_hash + time_window))` |
| Chat | E2E encrypted (NaCl secretbox, ECDH-derived key) |
| Registration | Ephemeral NaCl box encrypted hash delivery via issue comment |
| Anti-spam | GitHub account required (issue rate limits) |

### Cryptography

Single **ed25519 key pair** per user:

- **Encryption**: NaCl box with ed25519→curve25519 conversion
- **Signing**: ed25519 (TOTP auth, identity proof)
- **Key exchange**: ECDH via curve25519 (chat keys)
- **Wire format**: `[32B ephemeral pubkey][24B nonce][ciphertext + Poly1305 tag]`
- **`.bin` format**: `[32B ed25519 pubkey][encrypted profile]`

## Environment Variables

### CLI

| Variable | Default | Description |
|----------|---------|-------------|
| `DATING_HOME` | `~/.dating` | Local data directory |
| `DEBUG` | off | `1` for debug logs in TUI |

### Relay

| Variable | Required | Description |
|----------|----------|-------------|
| `POOL_URL` | Yes | Pool repo (e.g. `owner/pool-name`) |
| `POOL_SALT` | Yes | Secret salt for hash derivation |
| `PORT` | No | Server port (default: `8081`) |

## Local Data

```
~/.dating/
├── setting.toml              Config: user identity, pools, registries
├── keys/
│   ├── identity.pub          ed25519 public key (hex)
│   └── identity.key          ed25519 private key (hex, 0600)
├── pools/{name}/
│   └── profile.json          Per-pool profile
└── repos/                    Git clones (cache)
```

## Development

```bash
make build    # bin/dating + bin/relay
make test     # tests with isolated DATING_HOME
make coverage # coverage report
make lint     # golangci-lint
```

## Project Structure

```
cmd/
  dating/           CLI entry point
  relay/            WebSocket relay server
  regcrypt/         Registration hash tool for GitHub Actions
internal/
  cli/              CLI commands + TUI app
  cli/svc/          Service interfaces (DI layer)
  cli/tui/          Bubbletea screens, components, theme
  cli/config/       Config management (setting.toml)
  relay/            Relay server (TOTP auth, hub, sessions)
  crypto/           ed25519, NaCl box, TOTP, key derivation
  github/           GitHub API, pool, registry, profile types
  gitrepo/          Git clone management
  debug/            Debug logger
templates/
  actions/          GitHub Action templates for pool repos
web/                Next.js docs site
```

## License

MIT
