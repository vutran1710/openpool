<p align="center">
  <img src="assets/logo.png" width="200" alt="openpool">
</p>

<h1 align="center">openpool</h1>

<p align="center">
  Decentralized, terminal-native matching platform.<br>
  GitHub repos as the database, encrypted profiles, chain-encrypted discovery, E2E encrypted chat.
</p>

## Philosophy

- **GitHub is the database** — pools are repos, registration is an issue, matches are files
- **No central server owns your data** — profiles encrypted to operator, chat E2E encrypted
- **Terminal-first** — TUI for everyday use, CLI for automation
- **Rate-limited discovery** — chain encryption prevents mass scraping
- **Single key pair** — one ed25519 keypair for signing, encryption, and identity
- **Zero trust relay** — routes ciphertext, can't read messages

## Architecture

```
┌──────────┐       ┌──────────────────┐       ┌─────────────┐
│  CLI/TUI │──────▶│  Pool Repo       │       │  Registry   │
│  (Go)    │       │  (GitHub)        │◀─────▶│  (GitHub)   │
│          │       │  pool.yaml       │       └─────────────┘
│          │       │  users/{h}.bin   │
│          │       │  matches/        │
│          │       └──────────────────┘
│          │              ▲
│          │──────▶┌──────┴───────────┐
│          │       │  Relay Server    │
└──────────┘       │  (per-pool)      │
                   └──────────────────┘
```

## How It Works

### Hash Chain

```
id_hash    = sha256(pool:provider:user_id)     64 hex chars
bin_hash   = sha256(salt:id_hash)[:16]         16 hex chars — .bin filename
match_hash = sha256(salt:bin_hash)[:16]        16 hex chars — chat handle
```

Each layer is one-way without the salt. Discovery, matching, and identity are unlinkable namespaces.

### Registration

1. TUI: fill schema-driven profile form → submit
2. GitHub Issue created with encrypted profile blob
3. Action validates against `pool.yaml`, computes hashes, commits `.bin`, posts signed comment
4. Client polls, decrypts hashes, persists to config

### Discovery (Chain Encryption)

Profiles are organized into buckets (by role, age range, skills, etc.) and chain-encrypted:

- **Indexer** (Action cron): buckets profiles → builds AES-256-GCM encrypted chains → uploads `index.db`
- **Explorer** (client): selects bucket from preferences → grinds chain → unlocks one profile at a time
- **Cold solve**: ~seconds per profile (brute-force `10^4 × N × 6` AES attempts)
- **Warm solve**: ~instant for previously seen profiles (known constant carries across permutations)
- **Without hint**: infeasible (`10^8 × 10^4 × N × 6` — days to years)

### Interest & Matching

1. User likes someone → creates issue with **ephemeral hash** as title (rotates every N days)
2. Action detects mutual interest → posts encrypted match notifications with peer pubkeys
3. Match file committed → relay authorizes chat between the pair

### Chat

- E2E encrypted (NaCl secretbox, ECDH-derived key)
- Relay authenticates via TOTP at WebSocket connect — no login, no tokens
- Binary frames: `[8B target_match_hash][ciphertext]`
- Messages persisted locally in SQLite

## Pool Configuration (`pool.yaml`)

```yaml
name: "<3 dating"
description: "terminal-native dating"
relay_url: wss://relay.example.com
operator_public_key: c251e2cf...
interest_expiry: 3d

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

## Install

**Download the latest release:**

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/vutran1710/openpool/releases/latest/download/op-darwin-arm64 -o op
chmod +x op && sudo mv op /usr/local/bin/

# macOS (Intel)
curl -fsSL https://github.com/vutran1710/openpool/releases/latest/download/op-darwin-amd64 -o op
chmod +x op && sudo mv op /usr/local/bin/

# Linux (x86_64)
curl -fsSL https://github.com/vutran1710/openpool/releases/latest/download/op-linux-amd64 -o op
chmod +x op && sudo mv op /usr/local/bin/

# Linux (ARM64)
curl -fsSL https://github.com/vutran1710/openpool/releases/latest/download/op-linux-arm64 -o op
chmod +x op && sudo mv op /usr/local/bin/
```

**Or build from source:**

```bash
git clone https://github.com/vutran1710/openpool
cd openpool
make build    # builds bin/op + bin/relay + bin/action-tool
```

## Quick Start

```bash
# Launch TUI — guided onboarding
op

# Or use CLI commands:
op pool join <pool-name>
op like <match_hash>
op chat <match_hash>
```

## Operator Tools

```bash
# Create managed users (for testing or non-GitHub identity providers)
action-tool managed-register \
  --provider managed --userid test-user \
  --profile profile.json --pool owner/repo \
  --output-dir /tmp/user-home

# Build chain-encrypted index
action-tool index --schema pool.yaml --users-dir users/ --output index.db --upload
```

## Binaries

| Binary | Purpose |
|--------|---------|
| `op` | CLI/TUI app for end users |
| `relay` | Per-pool stateless WebSocket relay server |
| `action-tool` | Operator toolbox: register, match, squash, index, managed-register |

## Local Data

```
~/.openpool/
├── setting.toml                     Config: user, pools, registries
├── keys/identity.{pub,key}          ed25519 keypair (hex)
├── conversations.db                 Chat messages (SQLite)
└── pools/<name>/
    ├── profile.json                 User's profile for this pool
    ├── preferences.yaml             Discovery preferences (local only)
    ├── index.db                     Chain-encrypted profiles (downloaded)
    └── explorer.db                  Explorer state (constants, seen)
```

## Security

| Layer | Mechanism |
|-------|-----------|
| Identity | ed25519 key pairs, generated locally |
| Profiles | Encrypted to operator pubkey (NaCl box) |
| Discovery | Chain-encrypted (AES-256-GCM), rate-limited |
| Interest | Ephemeral hash titles (rotate every N days) |
| Relay auth | TOTP: `sign(sha256(time_window + host))`, channel-bound |
| Chat | E2E encrypted (NaCl secretbox, ECDH key) |
| Comments | Operator-signed (ed25519), verified before decryption |

## Development

```bash
make build     # bin/op + bin/relay + bin/action-tool
make test      # tests with isolated OPENPOOL_HOME
make coverage  # coverage report
```

## Roadmap

- [ ] AI assistance during chat via `/ai` command
- [ ] Homebrew tap (`brew install vutran1710/tap/op`)
- [ ] Self-hosted media via Cloudflare Tunnel
- [ ] Forward secrecy for chat (ratcheted key exchange)
- [ ] Web-based pool explorer (browse pools without CLI)
- [ ] Multi-registry federation
- [ ] Salt & operator private key rotation/migration

## License

MIT
