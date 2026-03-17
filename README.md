# dating.dev

Decentralized, terminal-native dating. GitHub repos as the database, encrypted profiles, and the command line.

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
└──────────┘       │  WS + Discovery  │
                   └──────────────────┘
```

- **Pools** are GitHub repos — like Discord servers, but decentralized
- **Profiles** are encrypted to the operator — only the relay can decrypt and serve them
- **Registration** via GitHub Issues — a GitHub Action commits the `.bin` file
- **Discovery** via relay — relay picks a random profile, re-encrypts it for you
- **Likes** are Pull Requests — matches are merged PRs
- **Relay** handles real-time chat (WebSocket) and profile discovery (HTTP)
- **CLI** is the only user interface — TUI for non-tech, commands for power users
- **Auth** via GitHub/Google OAuth — each pool brings its own OAuth app

## Install

```bash
curl -sSL https://dating.dev/install.sh | sh
```

Or build from source:

```bash
git clone https://github.com/vutran1710/dating-dev
cd dating-dev
make build    # builds bin/dating + bin/relay
```

## Quick Start

```bash
# Join a pool (OAuth → keys → profile → registration issue)
dating pool join berlin-singles

# Discover profiles (fetched and decrypted via relay)
dating fetch

# Like someone
dating like abc123def

# Check inbox
dating inbox

# Accept a match
dating accept 14

# Chat
dating chat abc123def
```

## CLI Commands

```
dating pool browse              Browse available pools
dating pool join <name>         Join a pool (OAuth → keys → profile → issue)
dating pool create <name>       Create and register a new pool
dating pool list                List joined pools
dating pool switch <name>       Set active pool
dating pool leave <name>        Leave a pool

dating fetch                    Discover profiles (via relay, decrypted)
dating view <hash>              View a user's encrypted blob size
dating like <hash>              Express interest (creates PR)
dating inbox                    View incoming interests
dating accept <pr>              Accept interest (merges PR = match)

dating chat <hash>              Chat with a match
dating commit propose <hash>    Propose a relationship (PR to relationships/)
dating commit proposals         View incoming proposals
dating commit accept <pr>       Accept proposal
dating commit status            Check relationship status

dating profile edit             Edit and publish profile
dating profile show             Show your profile (decrypted locally)
dating auth whoami              Show current identity
```

## How It Works

### Registration

1. CLI authenticates via OAuth (GitHub/Google)
2. CLI generates ed25519 keypair locally
3. CLI encrypts profile to **operator's pubkey** (NaCl box)
4. CLI creates a GitHub Issue with the encrypted blob + pubkey
5. GitHub Action commits `users/{hash}.bin` and closes the issue
6. No fork, no PR, no shared PAT — user just needs a GitHub account to open an issue

### Discovery

1. CLI sends signed request to relay's `POST /discover`
2. Relay picks a random `.bin` from the pool repo via GitHub API
3. Relay decrypts profile with operator private key
4. Relay re-encrypts profile to the requester's pubkey
5. CLI decrypts and displays the profile

Users can only see profiles the relay gives them — cloning the repo yields only opaque encrypted blobs.

### Matching & Chat

1. `dating like <hash>` creates a PR (like = open PR, match = merged PR)
2. `dating accept <pr>` merges the PR
3. `dating chat <hash>` connects via WebSocket relay (authenticated by nonce challenge)

## Pool Structure

```
pool-repo/
  pool.json                    Pool metadata + operator public key
  users/{hash}.bin             [32B ed25519 pubkey][profile encrypted to operator]
  matches/{pair_hash}/         Matched pairs (.bin files)
  relationships/{pair_hash}/   Committed pairs (.bin + meta.json)
```

## Security

| Layer | Mechanism |
|-------|-----------|
| Identity | ed25519 key pairs, generated locally |
| Signing | ed25519 signatures (native) |
| Encryption | NaCl box (Curve25519 + XSalsa20-Poly1305) with ed25519→curve25519 conversion |
| User hash | `SHA256(pool_salt:pool_repo:github:user_id)` — computed by Action, salt is secret |
| Profile data | Encrypted to **operator pubkey** — only relay decrypts |
| Discovery | Relay re-encrypts per-request — users can't browse the whole DB |
| Registration | GitHub Issue → Action commits (no identity in issue body) |
| Relay auth | Pubkey extracted from `.bin`, nonce challenge-response |
| Token storage | GitHub PAT encrypted with user's ed25519 pubkey, stored locally |
| Anti-spam | GitHub account required (issue rate limits) |

### Cryptography

All cryptographic operations use a single **ed25519 key pair** per user:

- **Encryption**: NaCl box with automatic ed25519 → curve25519 key conversion
  - Public key: Edwards→Montgomery point conversion (`filippo.io/edwards25519`)
  - Private key: SHA-512 seed derivation with clamping (RFC 8032)
  - Same approach as libsodium, Signal Protocol, and other established systems
- **Signing**: ed25519 (RFC 8032) — used only for relay WebSocket authentication
- **Wire format**: `[32B ephemeral curve25519 pubkey][24B nonce][ciphertext + Poly1305 tag]`
- **`.bin` format**: `[32B ed25519 pubkey][encrypted profile]`

### Authentication Model

| Operation | Auth Method | Why |
|-----------|-----------|-----|
| Registration (issue) | GitHub token | GitHub authenticates the user; Action derives identity from issue author |
| Likes, proposals (PRs) | GitHub token | GitHub authenticates who created the PR |
| Relay WebSocket | ed25519 signature | Nonce challenge-response proves private key ownership without exposing tokens |
| Profile discovery | Relay auth | Already authenticated via WebSocket before requesting profiles |

The pubkey is embedded in the `.bin` file (first 32 bytes). The relay reads it from the file and challenges the user to sign a nonce — proving they own the corresponding private key. No pubkey is sent separately in registration payloads.

### Environment Variables

#### CLI

| Variable | Default | Description |
|----------|---------|-------------|
| `DATING_HOME` | `~/.dating` | Local data directory (config, keys, profiles, repos) |
| `DEBUG` | off | Set to `1` or `true` to show debug logs in TUI status bar |

#### Relay

| Variable | Required | Description |
|----------|----------|-------------|
| `OPERATOR_PRIVATE_KEY` | Yes | Hex-encoded ed25519 private key for profile decryption + re-encryption |
| `POOL_SALT` | Yes | Secret salt for hash_id computation: `SHA256(salt:pool_url:provider:user_id)[:16]` |
| `POOL_TOKEN` | Yes | GitHub PAT for pool repo API access (avoids 60 req/hr anonymous limit) |
| `POOL_REPOS` | Yes | Comma-separated pool repo URLs to sync (e.g. `owner/pool-a,owner/pool-b`) |
| `PORT` | No | Server port (default: `8081`) |
| `TOKEN_TTL` | No | Auth token lifetime in seconds (default: `900` = 15 min) |
| `SYNC_INTERVAL` | No | Repo sync interval (default: `2m`) |

### Local Data

All local data is stored under `$DATING_HOME` (default `~/.dating/`):

```
~/.dating/
├── setting.toml              # Config: user identity, pools, registries, encrypted token
├── profile.json              # Complete profile (all sources merged)
├── keys/
│   ├── identity.pub          # ed25519 public key (hex)
│   └── identity.key          # ed25519 private key (hex, 0600 perms)
├── pools/{name}/
│   └── profile.json          # Per-pool profile (filtered fields)
├── repos/                    # Shallow git clones (cache, deletable)
└── archive/                  # Backups from `dating reset`
```

## Development

```bash
make build    # builds bin/dating + bin/relay
make cli      # builds bin/dating only
make test     # runs tests with isolated DATING_HOME (temp dir)
make coverage # test coverage report
make lint     # golangci-lint
```

Tests use `DATING_HOME` set to a temp directory — they never touch real config.

## Project Structure

```
cmd/dating/         CLI entry point
cmd/relay/          WebSocket relay server
internal/
  cli/              CLI commands, TUI screens, services
  cli/svc/          Service interfaces (Config, Crypto, Git, GitHub, Profile, Persistence, Polling)
  cli/tui/          Bubbletea TUI (app, screens, components, theme)
  cli/config/       Config file management (setting.toml)
  relay/            Relay server (auth, hub, client, protocol, discovery)
  crypto/           ed25519 keys, NaCl box encryption, signing
  github/           GitHub API client, pool, registry, profile, templates
  gitrepo/          Local git clone management, raw content fetcher
  debug/            Debug logger (DEBUG=1)
web/                Next.js documentation site
```

## Tech Stack

Go, Cobra, Bubbletea, Lipgloss, ed25519, NaCl box, gorilla/websocket

## License

MIT
