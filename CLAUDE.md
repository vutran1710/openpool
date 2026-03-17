# CLAUDE.md

## Project Overview

Terminal-native, decentralized dating platform. GitHub repos as the database, encrypted profiles, CLI + TUI interface. Pre-launch (private repo).

## Tech Stack

- **Language**: Go 1.25
- **CLI**: Cobra (commands), Bubbletea (TUI), Lipgloss (styling)
- **Crypto**: ed25519 (signing/TOTP) + NaCl box with ed25519→curve25519 (encryption) + ECDH (chat keys)
- **Config**: TOML (`~/.dating/setting.toml`)
- **Protocol**: MessagePack over WebSocket
- **Web**: Next.js (docs site at `web/`)

## Architecture

```
cmd/dating/       → CLI entry point
cmd/relay/        → Per-pool WebSocket relay server
cmd/regcrypt/     → Registration hash tool (used by GitHub Actions)
internal/
  cli/            → CLI commands + TUI app
  cli/svc/        → Service interfaces (dependency injection)
  cli/tui/        → Bubbletea screens, components, theme
  cli/config/     → Config file management
  crypto/         → Encryption, signing, TOTP, key derivation
  github/         → GitHub API client, pool/registry operations, profile types
  gitrepo/        → Git clone management, raw content fetcher
  relay/          → Relay server (TOTP auth, hub, sessions, store)
  protocol/       → MessagePack wire protocol types + codec
  debug/          → Debug logger (DEBUG=1)
templates/
  actions/        → GitHub Action templates for pool repos
```

## Key Concepts

### Hash Chain

```
id_hash    = sha256(pool_url:provider:user_id)     → 64 hex chars (public identity)
bin_hash   = sha256(salt:id_hash)[:16]             → 16 hex chars (relay routing key)
match_hash = sha256(salt:bin_hash)[:16]             → 16 hex chars (TOTP shared secret)
```

Go type: `crypto.IDHash` (for id_hash). Salt is a pool secret.

### Relay Auth (TOTP)

No JWT, no tokens, no login endpoint. Client computes signature at connect time:

```
ws://relay/ws?bin=<bin_hash>&sig=<totp_signature>
```

Relay verifies `ed25519.Verify(pubkey, sha256(bin_hash + match_hash + time_window), sig)` inline at WebSocket upgrade. ±1 window (5 min) for clock drift.

Implemented in `internal/crypto/totp.go`: `TOTPSign`, `TOTPSignAt`, `TOTPVerify`.

### Registration Flow

1. `dating profile create <pool>` → scaffolds local `profile.json`
2. `dating pool join <pool>` → reads profile, encrypts, submits GitHub Issue
3. GitHub Action runs `regcrypt` → computes hashes → encrypts `{bin_hash, match_hash}` to user's pubkey → posts as issue comment → commits `.bin`
4. CLI polls issue comments → decrypts → persists hashes to config

### E2E Encrypted Chat

Messages encrypted with NaCl secretbox. Key derived via ECDH (ed25519→curve25519) + HKDF. Relay routes ciphertext only.

## Conventions

### Code Style

- No unnecessary abstractions — three similar lines is better than a premature helper
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

Per-pool WebSocket server. Env vars: `POOL_URL`, `POOL_SALT`, `PORT`.

Key components:
- `Server` — HTTP handler, TOTP verify at WS upgrade
- `Store` — in-memory user index (keyed by `bin_hash`) + match pairs
- `Hub` — active session registry
- `Session` — message routing, key exchange, ack handling

No JWT, no tokens, no auth endpoints. Auth happens at WS upgrade via TOTP signature in query params.

## Binaries

| Binary | Purpose | Distribution |
|--------|---------|-------------|
| `dating` | CLI app | End users |
| `relay` | Per-pool relay server | Pool operators |
| `regcrypt` | Hash computation for Actions | Published to `vutran1710/regcrypt` (public) |

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
