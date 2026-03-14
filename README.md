# dating.dev

Decentralized, terminal-native dating. No servers — just GitHub repos, Pull Requests, and the command line.

## Architecture

```
┌──────────┐       ┌──────────────────┐       ┌─────────────┐
│  CLI     │──────▶│  Pool Repo       │       │  Registry   │
│  (Go)    │       │  (GitHub)        │◀─────▶│  (GitHub)   │
│          │──────▶│  users/          │       └─────────────┘
│          │       │  matches/        │
│          │──────▶│  relationships/  │
│          │       └──────────────────┘
│          │
│          │──────▶┌──────────────────┐
│          │       │  Relay Server    │
└──────────┘       │  (WebSocket)     │
                   └──────────────────┘
```

- **Pools** are GitHub repos — like Discord servers, but decentralized
- **Likes** are Pull Requests — matches are merged PRs
- **Relay** is a lightweight WebSocket server for real-time chat
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
# Join a pool (OAuth + profile creation + registration PR — all in one)
dating pool join berlin-singles

# Discover profiles
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
dating pool join <name>         Join a pool (OAuth → keys → profile → PR)
dating pool create <name>       Create and register a new pool
dating pool list                List joined pools
dating pool switch <name>       Set active pool
dating pool leave <name>        Leave a pool

dating fetch                    Discover profiles
dating view <hash>              View a user's encrypted blob
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

## Pool Structure

```
pool-repo/
  pool.json                    Pool metadata + operator public key
  users/{hash}.bin             [32 bytes ed25519 pubkey][encrypted profile]
  matches/{pair_hash}/         Matched pairs (.bin files)
  relationships/{pair_hash}/   Committed pairs (.bin + meta.json)
```

## Security

| Layer | Mechanism |
|-------|-----------|
| Identity | ed25519 key pairs, generated locally |
| User hash | `SHA256(pool_repo:provider:provider_user_id)` |
| Profile data | NaCl box encryption, only user decrypts |
| Registration | OAuth proves identity, encrypted proof for operator |
| Relay auth | Pubkey extracted from `.bin`, nonce challenge-response |
| Anonymity | PRs created via pool PAT, not user's account |
| Anti-spam | OAuth required, PR approval by operator |

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
