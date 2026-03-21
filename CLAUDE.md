# CLAUDE.md

## Project Overview

Terminal-native, decentralized dating platform. GitHub repos as the database, encrypted profiles, CLI + TUI interface. Pre-launch (private repo).

## Tech Stack

- **Language**: Go 1.25
- **CLI**: Cobra (commands), Bubbletea (TUI), Lipgloss (styling)
- **Crypto**: ed25519 (signing/TOTP) + NaCl box with ed25519→curve25519 (encryption) + ECDH (chat keys)
- **Config**: TOML (`~/.dating/setting.toml`)
- **Protocol**: Binary frames over WebSocket (text frames for control)
- **Web**: Next.js (docs site at `web/`)

## Architecture

```
cmd/dating/       → CLI entry point
cmd/relay/        → Per-pool WebSocket relay server
cmd/action-tool/  → Unified Action binary (register, match, squash, index, sign, decrypt, pubkey)
internal/
  cli/            → CLI commands + TUI app
  cli/chat/       → ChatClient + ConversationDB (SQLite message persistence)
  cli/svc/        → Service interfaces (dependency injection)
  cli/tui/        → Bubbletea screens, components, theme
  cli/config/     → Config file management
  crypto/         → Encryption, signing, TOTP, key derivation
  github/         → GitHubClient interface (CLIClient + HTTPClient), pool operations
  gitrepo/        → Git clone management, raw content fetcher
  limits/         → Payload size constants
  message/        → Structured message format (Format/Parse for openpool blocks)
  relay/          → Relay server (stateless TOTP auth, hub, sessions, GitHub cache)
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

Go type: `crypto.IDHash` (for id_hash). Salt is a pool secret (only in GitHub Actions secrets + relay env).

### Security Rationale

Each hash layer is **one-way and unlinkable** without the salt:

- **bin_hash is public** — anyone can see it in `.bin` filenames during discovery
- **match_hash is public** — it appears on interest PRs (PR title = target's match_hash)
- **The key insight**: knowing bin_hash tells you nothing about match_hash (and vice versa) without the salt
- An attacker who sees both bin_hash and match_hash **cannot link them** to the same person without the salt
- The salt only exists in two places: pool repo's GitHub Actions secrets + relay server's env var
- **real_id → id_hash**: anyone can compute (deterministic from public info), but only needed at registration
- **id_hash → bin_hash**: only the salt holder can compute (one-way)
- **bin_hash → match_hash**: only the salt holder can compute (one-way)

This separation means: discovery (bin_hash), matching (match_hash), and identity (id_hash) are three unlinkable namespaces.

### Relay Auth (TOTP)

No JWT, no tokens, no login endpoint, no database. Client computes signature at connect time:

```
ws://relay/ws?id=<id_hash>&match=<match_hash>&sig=<totp_signature>
```

Relay validates the chain (`id_hash → bin_hash → match_hash` via salt), fetches pubkey from GitHub raw content, then verifies `ed25519.Verify(pubkey, sha256(time_window), sig)`. ±1 window (5 min) for clock drift.

Implemented in `internal/crypto/totp.go`: `TOTPSign(priv)`, `TOTPSignAt(priv, tw)`, `TOTPVerify(sigHex, pub)`.

### Registration Flow

1. `dating profile create <pool>` → scaffolds local `profile.json`
2. `dating pool join <pool>` → reads profile, encrypts, submits GitHub Issue
3. GitHub Action runs `regcrypt` → computes hashes → encrypts `{bin_hash, match_hash}` to user's pubkey → posts as issue comment → commits `.bin`
4. CLI polls issue comments → decrypts → persists hashes to config

### E2E Encrypted Chat

Messages encrypted with NaCl secretbox. Key derived via ECDH (ed25519→curve25519) + HKDF. Relay routes ciphertext only.

### Match Notification as Key Exchange

When a mutual match is detected, the GitHub Action encrypts a notification to each user containing the peer's **pubkey** (not just bin_hash). This means:
- Match notification IS the key exchange — no separate key_request/key_response protocol needed
- Each user receives their peer's ed25519 pubkey encrypted to their own pubkey
- The client can derive the ECDH shared secret immediately from the notification
- The relay never needs to store or serve pubkeys

### Discovery & Matching Flow

1. **Discovery**: Users browse `.bin` files (keyed by bin_hash), decrypt profiles with operator key
2. **Interest**: User creates an Issue with `title=<target_match_hash>`, `label=interest`, encrypted body
3. **Mutual match**: GitHub Action detects both directions, posts encrypted notifications with peer pubkeys, closes + locks both issues
4. **Chat**: `dating chat <match_hash>` — user identifies peers by match_hash (not bin_hash)

### Message Format

All structured content (issues, comments) uses the `message` package:

```
<!-- openpool:{block_type} -->
```​
{content}
```​
```

Block types: `registration-request`, `registration`, `interest`, `match`, `error`.
Parsed by `message.Parse(raw)`, created by `message.Format(blockType, content)`.

### ChatClient & Message Persistence

`internal/cli/chat/` provides:
- `ConversationDB` — SQLite at `~/.dating/conversations.db` (messages, conversations, peer_keys tables)
- `ChatClient` — composes relay client + DB. Handles send, receive, history, mark read, peer key storage.

Screens are pure views — they don't touch the relay or DB directly. ChatClient handles all logic.
The TUI connects to the relay on startup (background). Incoming messages persist and update the conversations panel in real-time.
The CLI `dating chat` command also uses ChatClient for persistence.

### GitHub Client

`internal/github/` has a `GitHubClient` interface with two implementations:
- `CLIClient` — shells out to `gh` CLI (used by Action tools + CLI when `gh` is available)
- `HTTPClient` — direct HTTP with PAT (fallback)

`NewCLIOrHTTP(repo, token)` prefers CLI, falls back to HTTP.

### Schema-Driven Matching

Pool schema (`pool.json`) defines fields with 4 match modes:

| Mode | Behavior | Example |
|------|----------|---------|
| `complementary` | Field A's value matches field B's target | gender: "male" seeks "female" |
| `approximate` | Values within tolerance range | age: 25 ± 5 |
| `exact` | Values must be identical | city: "Berlin" = "Berlin" |
| `similarity` | Cosine similarity on interest vectors | interests overlap score |

Operator-defined weights are private (not in schema). Ranking uses tier-shuffled ordering: top 5% → 5-20% → 20-50% → 50-100%, shuffled within each tier.

## Conventions

### Code Style

- No unnecessary abstractions — three similar lines is better than a premature helper
- Single responsibility — functions do one thing. Don't jam fetch + parse + verify + decrypt into one function. Compose small functions.
- Separate view from logic (React philosophy) — TUI screens are pure views, logic lives in dedicated classes (ChatClient, ConversationDB). Screens never import relay, DB, or crypto directly.
- No backward compatibility during pre-launch — delete old code, don't maintain fallbacks
- CLI commands must be fully scriptable — every interactive prompt needs a flag/env var alternative
- Interfaces live in `internal/cli/svc/svc.go` — single source of truth
- Real implementations in `internal/cli/services.go`, mocks in `internal/cli/services_mock.go`
- Service-level mocks in `internal/cli/svc/mocks.go`
- TUI screens in `internal/cli/tui/screens/`, components in `internal/cli/tui/components/`

### Naming

- Files: `snake_case.go`
- Test files: `snake_case_test.go` in the same package
- Benchmark files: `snake_case_bench_test.go`
- Mock files: `mock_*.go` or `*_mock.go`
- Service interfaces: `FooService` with methods like `Load`, `Save`, `Get`
- Hash types: `IDHash` (not `PublicID`), `BinHash`, `MatchHash`

### Error Handling

- Return errors, don't panic
- Wrap with context: `fmt.Errorf("doing X: %w", err)`
- In TUI screens: show error in UI (toast or error step), don't crash
- In async tea.Cmd: return error messages, handle in Update

### Imports

- Group: stdlib, then external, then internal
- Alias github packages: `gh "github.com/vutran1710/dating-dev/internal/github"`
- Alias debug: `dbg "github.com/vutran1710/dating-dev/internal/debug"`

## Testing

### Running Tests

```bash
make test      # runs with isolated DATING_HOME (temp dir)
make coverage  # generates coverage.out + coverage.html
```

**CRITICAL**: Always use `make test` or set `DATING_HOME` to a temp dir. Running bare `go test ./...` writes test data to real `~/.dating/setting.toml`.

### Test Patterns

- Unit tests in same package (`_test.go`)
- Test both happy and sad paths (success, error, empty, invalid)
- TUI screen tests: construct → send message → assert state + command
- Use `tea.KeyMsg{Type: tea.KeyEnter}` for key simulation
- Mock services via `svc/mocks.go` or `cli/services_mock.go`
- Benchmarks for performance-sensitive code (profile rendering, glamour)
- Use `-race` flag for concurrent code (TOTP, relay)

### Test Isolation

- `DATING_HOME` env var controls where config/keys/profiles are stored
- `make test` sets it to a temp dir automatically
- Never hardcode `~/.dating` in tests — use `config.Dir()` which reads `DATING_HOME`

### Adding Tests

Every code change must include tests:
- New feature → unit tests for all state transitions
- Bug fix → regression test proving the fix
- New service method → test happy path + error path
- New TUI component → test Update/View with key events

## TUI Development

### Screen Structure

Each screen implements:
```go
type FooScreen struct { ... }
func NewFooScreen(...) FooScreen
func (s FooScreen) Update(msg tea.Msg) (FooScreen, tea.Cmd)
func (s FooScreen) View() string
func (s FooScreen) HelpBindings() []components.KeyBind
```

### Bubbletea Components Used

- `bubbles/spinner` — loading indicators
- `bubbles/textinput` — single-line input
- `bubbles/textarea` — multi-line input (about text)
- `bubbles/viewport` — scrollable content
- `bubbles/table` — pool card info table
- `bubbles/help` — help bar with key bindings

### Key Input Routing

- Screens that handle their own input (onboarding, join, profile) must block forwarding to the command input. Check `updateActiveScreen` in `app.go` — add screen to the `screenOnboarding || screenJoin || screenProfile` guard.

### Performance

- Glamour markdown rendering is slow (~94ms). Cache rendered output, never re-render on mode switch.
- Profile card: cache Normal + Compact modes, swap cached strings on tab.
- Git operations: `Clone()` returns instantly if already cloned. Use `Sync()` for smart updates.

## Services (svc package)

7 services, each with interface + real impl + mock:

| Service | Scope |
|---------|-------|
| ConfigService | Read/write setting.toml |
| CryptoService | Encrypt/decrypt/sign |
| GitService | Clone/fetch git repos |
| GitHubService | GitHub API calls |
| ProfileService | Read/write profile files |
| PersistenceService | Orchestrates config + profile writes |
| PollingService | Background issue status polling |

## Relay Server

Per-pool stateless WebSocket server. Env vars: `POOL_URL`, `POOL_SALT`, `PORT`.

Deployed on Railway (~$0.40/month). No database — fully stateless.

Key components:
- `Server` — HTTP handler, stateless TOTP auth at WS upgrade, hash derivation from salt
- `GitHubCache` — Fetches pubkeys + match files from `raw.githubusercontent.com`, caches in memory
- `Hub` — active session registry (keyed by `match_hash`) + offline message queue (cap 20)
- `Session` — binary frame routing, text frame control messages

### Wire Format

- **Binary frames** (chat messages): `[target_match_hash_8B][ciphertext]` — relay prepends sender's match_hash when forwarding
- **Text frames** (control): `queued`, `not matched`, `match check failed`, `queue full`, `frame too large`, `unknown user`
- **Size limits**: 8 KB max binary frame, 4 KB max chat plaintext
- Encryption: `crypto.SealRaw`/`crypto.OpenRaw` (NaCl secretbox, raw bytes — no base64)

### Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /ws` | Stateless TOTP auth + WebSocket upgrade |
| `GET /health` | `{status, online, queued}` JSON |

### Match Authorization

`pair_hash = sha256(min(a,b) + ":" + max(a,b))[:12]` where a,b are match_hashes. Relay fetches `matches/{pair_hash}.json` from GitHub raw content. Cached for 5 minutes (positive + 404 negative). 5xx/network errors NOT cached.

## Binaries

| Binary | Purpose | Distribution |
|--------|---------|-------------|
| `dating` | CLI app | End users |
| `relay` | Per-pool relay server | Pool operators (Railway) |
| `action-tool` | All Action operations (register, match, squash, index, sign, decrypt, pubkey) | GitHub Actions, published to `vutran1710/regcrypt` |

## GitHub Actions (Pool Repo)

Three Actions in `templates/actions/` — deployed to each pool repo:

| Action | Trigger | Purpose |
|--------|---------|---------|
| `pool-register.yml` | Issue opened with `registration` label | `./regcrypt register` — one-line, all logic in Go |
| `pool-indexer.yml` | Cron (every 5 min) | Runs `indexer --rebuild`, commits `index.pack` |
| `pool-interest.yml` | Issue opened with `interest` label | `./matchcrypt match` — one-line, all logic in Go |

Action templates are thin wrappers — download binary + run. All parsing, crypto, git, and GitHub operations happen inside the Go tools. Issues are locked after processing to prevent re-open attacks.

### Indexing & Discovery

- `.bin` files: `[32B pubkey][NaCl box encrypted profile]` — committed by registration Action
- `.rec` files: Not committed separately — indexer reads `.bin`, extracts filters + vectors + display info
- `index.pack`: MessagePack bundle of all `IndexRecord`s — committed by cron indexer
- CLI downloads `index.pack` for local discovery/ranking
- Cron indexer is separate from registration to break `.bin`/`.rec` commit linkability

## Debug Mode

```bash
DEBUG=1 dating
```

Shows timestamped logs in TUI status bar. Use `dbg.Log()` and `dbg.Timer()` from `internal/debug`.

## Common Gotchas

- **Config corruption**: Tests MUST use `DATING_HOME` temp dir. Bare `go test` writes to real config.
- **Viewport blank**: Screens need `WindowSizeMsg` before rendering. Send it explicitly when navigating.
- **Key events eaten**: Command input steals arrow/tab keys. Block forwarding for screens that need them.
- **Glamour slow**: Never call glamour in `Update()` or on mode switch. Cache once, swap strings.
