# Documentation

Technical reference for developers building with and on Dating CLI.

---

## Architecture

Dating CLI is fully decentralized. There are no central servers, no databases, and no hosted infrastructure. The entire platform runs on three primitives: **GitHub repos** (state), **GitHub PRs** (actions), and **Telegram bots** (chat transport).

### System overview

```
┌──────────────┐         ┌──────────────────────┐
│   CLI        │────────▶│   Registry Repo      │
│   (Go bin)   │         │   (pool discovery)   │
│              │         └──────────────────────┘
│              │                │
│              │         ┌──────▼──────────────┐
│              │────────▶│   Pool Repo         │
│              │         │   (profiles, index, │
│              │         │    matches, commits) │
│              │         └─────────────────────┘
│              │
│              │         ┌─────────────────────┐
│              │────────▶│   Telegram Bot API  │
│              │         │   (chat transport)  │
└──────────────┘         └─────────────────────┘
```

### Components

| Component | Role | Who controls it |
|-----------|------|-----------------|
| **CLI** | The only user interface. A single Go binary that handles identity, discovery, matching, chat, and profile management. | The user |
| **Pool repo** | A GitHub repository that stores all state for a dating community: user profiles, symlinked indexes, match directories, commitment artifacts, and PR templates. | The pool operator |
| **Registry repo** | A GitHub repository that acts as a directory of pools. Pools register here via PRs. Users browse the registry to discover and join pools. | The registry maintainer |
| **Telegram bot** | An invisible message relay. The CLI sends and receives messages through the Telegram Bot HTTP API. Users never interact with Telegram directly. | The pool operator (creates the bot) |

### Data flow

**Discovery (read path):**

```
CLI → GitHub Contents API → pool repo → /index/by-status/open/
  → list symlinks → resolve to /users/{id}/public.json → display profile
```

**Joining a pool (write path):**

```
CLI → GitHub API → creates branch on pool repo
  → commits users/{id}/public.json + index symlink
  → opens PR "Join: alice (8ac21)"
  → pool operator reviews and merges
```

**Liking someone (write path):**

```
CLI → GitHub API → creates branch on pool repo
  → commits both profiles to matches/{hash}/
  → opens PR "Like: 8ac21 -> 3f90a" with label like:3f90a
  → target user sees it via `dating inbox`
  → target runs `dating accept <pr>` → PR merges → match created
```

**Chat (real-time):**

```
CLI (user A) → HTTP POST → Telegram Bot API → bot stores message
CLI (user B) → HTTP GET (long poll) → Telegram Bot API → receives message
```

### Design principles

- **GitHub as database** — all state is files in git. Profiles are JSON, indexes are symlinks, matches are directories. `git log` is the audit trail.
- **PRs as actions** — every mutation (join, like, accept) is a Pull Request. This gives you review, approval, rollback, and automation via GitHub Actions.
- **Shared PAT model** — pool operators publish a fine-grained GitHub token scoped to their repo. Users don't need GitHub accounts. The token is the "API key" for the pool.
- **Zero infrastructure** — no servers to run, no databases to maintain, no cloud bills. GitHub and Telegram free tiers handle everything.
- **Federated** — anyone can run their own registry and pools. There is no single point of control. The default registry (`vutran1710/dating-pool-registry`) is just one of many possible registries.

### Security model

| Concern | Solution |
|---------|----------|
| **Identity spoofing** | Every action is signed with the user's ed25519 private key. Pool operators can verify signatures. |
| **Unauthorized writes** | All writes go through PRs. Pool operators control merge permissions. |
| **Token scope** | Pool PATs are fine-grained: scoped to a single repo with only Contents and Pull Request write permissions. |
| **Key compromise** | Users can rotate keys by re-registering. Old signed artifacts remain verifiable against the old public key. |
| **Spam** | PR-based approval model. Every join and like requires operator review (or automated checks via GitHub Actions). |

---

## Identity & Keys

Each user has a local **ed25519** key pair generated at registration.

```
~/.dating/
  keys/
    identity.pub    # ed25519 public key
    identity.key    # ed25519 private key (never leaves your machine)
  config.toml       # pools, active pool, user info
```

**Public ID** — derived from first 5 hex chars of the public key (e.g. `8ac21`).

**Signing** — every action (like, join, profile update) is signed with your private key. The signature is included in the PR body so pool operators can verify authenticity.

---

## Pools

A pool is a GitHub repository that acts as a dating community — like a Discord server, but fully decentralized.

### Repo structure

```
pool-repo/
  pool.json                          # pool metadata
  users/{public_id}/public.json      # user profiles
  index/
    by-status/open/{public_id}       # symlinks to profiles
    by-city/{city}/{public_id}
    by-interest/{interest}/{public_id}
  matches/{hash}/                    # matched pairs
  commitments/{hash}.json            # commitment artifacts
  .github/
    PULL_REQUEST_TEMPLATE/
      join.md                        # join PR template (monetization)
      like.md                        # like PR template
    workflows/                       # automation on merge
```

### Creating a pool

```bash
dating pool create my-pool \
  --repo owner/my-pool-repo \
  --gh-token ghp_xxx \
  --bot-token 123:ABC \
  --registry-token ghp_yyy \
  --desc "My dating community"
```

This creates a PR to the registry. Once the registry maintainer merges it, the pool is live.

---

## Registry

The registry is a GitHub repo where pools register for discovery.

### Structure

```
registry-repo/
  pools/{pool-name}/
    pool.json       # name, repo, description, created_at
    tokens.bin      # serialized GitHub PAT + Telegram bot token
```

### Joining flow

1. `dating pool browse` — lists pools from the registry
2. `dating pool join <name>` — fetches pool config from registry, then creates a join PR to the pool repo
3. Pool operator reviews and merges

Anyone can run their own registry. The default is `vutran1710/dating-pool-registry`.

```bash
# Use a custom registry
dating pool browse --registry your-org/your-registry
dating pool join my-pool --registry your-org/your-registry
```

---

## Matching (PRs)

Likes and matches use GitHub Pull Requests as the mechanism.

### Like flow

1. `dating like 8ac21` — CLI creates a PR on the pool repo
2. PR adds both profiles to `matches/{hash}/`
3. PR is labeled `like:8ac21` for inbox filtering
4. The other user sees it via `dating inbox`

### Accept flow

1. `dating accept <pr_number>` — merges the PR
2. Match directory is created on main branch
3. Both users can now chat

> **Match hash** — deterministic SHA256 of both public IDs (canonically ordered), truncated to 12 chars. Prevents duplicate match directories.

### Why PRs?

- Mutual consent (both parties involved)
- Full git history / audit trail
- PR comments as icebreakers
- GitHub Actions can automate post-match workflows
- Labels and templates enable custom admission criteria

---

## Chat (Telegram)

Chat uses a Telegram bot as invisible transport. Users never interact with Telegram directly.

```
CLI (user A) → Telegram Bot API → Bot stores message
CLI (user B) → Telegram Bot API → Bot delivers message
```

The pool creator provides a Telegram bot token when registering the pool. It's distributed to users via the registry.

### In the CLI

```bash
dating chat 8ac21
  dating:8ac21> hello!
  [14:22] 8ac21: hey back
  dating:8ac21> /exit
```

### Chat commands

| Command | Description |
|---------|-------------|
| `/exit` | Leave the chat |
| `/history` | View message history |
| `/profile` | View the other person's profile |

---

## Monetization (PR Templates)

Pool operators and registry maintainers can monetize via GitHub PR templates.

### How it works

1. Operator creates `.github/PULL_REQUEST_TEMPLATE/join.md` in the pool repo
2. Template can include custom fields using `{{ field_name }}` placeholders
3. When a user runs `dating pool join`, the CLI fetches the template, displays requirements, and prompts the user to fill in fields
4. Filled template is included in the PR body

### Example template

```markdown
## Join Requirements

To join this pool, you must be a GitHub Sponsor ($5/mo).
Sponsor link: https://github.com/sponsors/pool-owner

Sponsor username: {{ sponsor_username }}
Transaction date: {{ transaction_date }}
```

Pool operators can set up **GitHub Actions** that verify payment/sponsorship before auto-merging the PR.

> **Revenue model:** The pool operator keeps 100%. Zero platform cut.

---

## CLI Reference

### Authentication

```bash
dating auth register      # create identity (ed25519 keys)
dating auth whoami        # show current identity
```

### Pools

```bash
dating pool browse        # browse registry
dating pool create <name> # register a new pool
dating pool join <name>   # join a pool (creates PR)
dating pool leave <name>  # leave a pool
dating pool list          # list joined pools
dating pool switch <name> # set active pool
```

### Discovery

```bash
dating fetch              # random profile from active pool
dating view <public_id>   # view a profile
```

### Matching

```bash
dating like <public_id>   # express interest (creates PR)
dating inbox              # view incoming interests
dating accept <pr_number> # accept interest (merge PR = match)
```

### Chat

```bash
dating chat <public_id>   # chat with a match
```

### Commitment

```bash
dating commit <public_id> # propose commitment
dating status             # check relationship status
```

### Profile

```bash
dating profile edit       # edit and publish profile
dating profile show       # show current profile
```

---

## Configuration

All config lives in `~/.dating/config.toml`:

```toml
[user]
public_id = "8ac21"
display_name = "alice"

[[pools]]
name = "berlin-singles"
repo = "owner/berlin-singles"
token = "github_pat_xxx"
bot_token = "123456:ABC"
status = "active"

[[pools]]
name = "tokyo-devs"
repo = "owner/tokyo-devs"
token = "github_pat_yyy"
bot_token = "789012:DEF"
status = "pending"

active_pool = "berlin-singles"
```

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `DATING_CONFIG_DIR` | Override config directory (default: `~/.dating`) |

---

## Contributing

### Build from source

```bash
git clone https://github.com/vutran1710/dating-dev
cd dating-dev
make build        # builds bin/dating
make test         # runs tests
make lint         # runs golangci-lint
```

### Project structure

```
cmd/dating/           # CLI entry point
internal/
  cli/                # CLI commands and TUI
  cli/config/         # ~/.dating config management
  cli/tui/            # Interactive TUI mode
  crypto/             # ed25519 key management
  github/             # GitHub API client, pool, registry
  telegram/           # Telegram bot client
web/                  # Next.js docs site
```

**Tech stack:** Go, Cobra, Bubbletea, Lipgloss, ed25519
