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

```bash
curl -fsSL https://raw.githubusercontent.com/vutran1710/homebrew-tap/main/install.sh | sh
```

**Or with Homebrew:**

```bash
brew install vutran1710/tap/op
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

## Running a Pool

### 1. Create the pool repo

Fork [openpool-base-pool](https://github.com/vutran1710/openpool-base-pool) (template repo). It includes `pool.yaml`, a `users/` directory, and all the GitHub Actions pre-configured.

Edit `pool.yaml` to define your pool's name, profile schema, roles, and indexing config.

### 2. Generate operator keys

```bash
# Generate an ed25519 keypair
action-tool pubkey --generate

# Output:
#   private: <128 hex chars>
#   public:  <64 hex chars>
```

Add to your pool repo's GitHub Actions secrets:
- `OPERATOR_PRIVATE_KEY` — the private key (128 hex chars)
- `POOL_SALT` — a random string (used for hash derivation)

Set `operator_public_key` in `pool.yaml` to the public key.

### 3. Deploy the relay

The relay is a stateless WebSocket server. Deploy anywhere (Railway, Fly, VPS):

```bash
# Environment variables:
#   POOL_URL   — GitHub repo (e.g. owner/pool-name)
#   POOL_SALT  — same salt as in GitHub Actions secrets
#   PORT       — listen port (default: 8081)

POOL_URL=owner/pool-name POOL_SALT=your-salt relay
```

Set `relay_url` in `pool.yaml` to the deployed URL (e.g. `wss://relay.example.com`).

### 4. Register with a registry (optional)

Registries are how users discover pools, but they're not required — users can always join a pool directly by repo URL (`op pool join owner/pool-name`). If you want your pool listed for discovery, add it to a registry's `registry.yaml`. The base registry is [openpool-base-registry](https://github.com/vutran1710/openpool-base-registry).

You can also create your own registry by forking [openpool-base-registry](https://github.com/vutran1710/openpool-base-registry) and curating your own pool list.

### GitHub Actions (included in template)

| Action | Trigger | Purpose |
|--------|---------|---------|
| `pool-register.yml` | Issue with `registration` label | Validate profile, encrypt, commit `.bin` |
| `pool-interest.yml` | Issue with `interest` label | Detect mutual interest, post match notifications |
| `pool-indexer.yml` | Cron (every 5 min) | Build chain-encrypted `index.db`, upload as release |
| `pool-squash.yml` | Cron (hourly) | Squash git history, clean stale interests |
| `pool-unmatch.yml` | Issue with `unmatch` label | Remove match files |

## Managed Accounts

For testing or non-GitHub identity providers, operators can create users directly without the Issue-based registration flow:

```bash
action-tool managed-register \
  --provider managed \
  --userid test-user \
  --profile profile.json \
  --pool owner/repo \
  --schema pool.yaml \
  --output-dir /tmp/user-home
```

This generates a complete user home directory (`keys/`, `setting.toml`, pool config) that can be distributed to the user. Set `OPENPOOL_HOME=/tmp/user-home op` to run as that user.

## Operator Tools

All operator commands are in `action-tool`:

| Command | Purpose |
|---------|---------|
| `register` | Process registration issues |
| `match` | Detect mutual interest, create matches |
| `index` | Build chain-encrypted index, optionally upload |
| `squash` | Squash repo history, clean stale interests |
| `unmatch` | Process unmatch issues |
| `managed-register` | Create user accounts directly |
| `sign` | Sign a message with operator key |
| `decrypt` | Decrypt a blob with operator key |
| `pubkey` | Show or generate ed25519 keypair |

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

- [x] Homebrew tap (`brew install vutran1710/tap/op`)
- [ ] AI assistance during chat via `/ai` command
- [ ] Self-hosted media via Cloudflare Tunnel
- [ ] Forward secrecy for chat (ratcheted key exchange)
- [ ] Web-based pool explorer (browse pools without CLI)
- [ ] Multi-registry federation
- [ ] Salt & operator private key rotation/migration

## License

MIT
