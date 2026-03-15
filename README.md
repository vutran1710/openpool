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
| User hash | `SHA256(pool_repo:provider:provider_user_id)` |
| Profile data | NaCl box encrypted to **operator pubkey** — only relay decrypts |
| Discovery | Relay re-encrypts per-request — users can't browse the whole DB |
| Registration | GitHub Issue → Action commits (no link between GitHub user and hash via timing correlation accepted) |
| Relay auth | Pubkey extracted from `.bin`, nonce challenge-response |
| Anti-spam | OAuth required, GitHub's built-in issue rate limits |

### Relay Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `POOL_TOKEN` | Yes | GitHub PAT for pool repo API access (avoids 60 req/hr anonymous limit) |
| `OPERATOR_PRIVATE_KEY` | Yes | Hex-encoded ed25519 private key for profile decryption + re-encryption |
| `PORT` | No | Server port (default: 8081) |

## Project Structure

```
cmd/dating/         CLI entry point
cmd/relay/          WebSocket relay server
internal/
  cli/              CLI commands, TUI, config, OAuth
  relay/            Relay server (auth, hub, client, protocol)
  crypto/           ed25519 keys, NaCl encryption, Merkle tree, hashing
  github/           GitHub API client, pool, registry, PR templates
web/                Next.js documentation site
```

## Tech Stack

Go, Cobra, Bubbletea, Lipgloss, ed25519, NaCl box, gorilla/websocket

## License

MIT
