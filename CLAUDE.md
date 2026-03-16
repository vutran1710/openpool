# CLAUDE.md

## Project Overview

Terminal-native, decentralized dating platform. GitHub repos as the database, encrypted profiles, CLI + TUI interface.

## Tech Stack

- **Language**: Go 1.25
- **CLI**: Cobra (commands), Bubbletea (TUI), Lipgloss (styling)
- **Crypto**: ed25519 (signing) + NaCl box with ed25519→curve25519 conversion (encryption)
- **Config**: TOML (`~/.dating/setting.toml`)
- **Web**: Next.js 15 (docs site at `web/`)

## Architecture

```
cmd/dating/       → CLI entry point
cmd/relay/        → WebSocket relay server
internal/
  cli/            → CLI commands + TUI app
  cli/svc/        → Service interfaces (the dependency injection layer)
  cli/tui/        → Bubbletea screens, components, theme
  cli/config/     → Config file management
  crypto/         → Encryption, signing, key management
  github/         → GitHub API client, pool/registry operations, profile types
  gitrepo/        → Git clone management, raw content fetcher
  relay/          → WebSocket relay server
  debug/          → Debug logger (DEBUG=1)
```

## Conventions

### Code Style

- No unnecessary abstractions — three similar lines is better than a premature helper
- Interfaces live in `internal/cli/svc/svc.go` — single source of truth
- Real implementations in `internal/cli/services.go`, mocks in `internal/cli/services_mock.go`
- Service-level mocks in `internal/cli/svc/mocks.go`
- TUI screens are in `internal/cli/tui/screens/`, components in `internal/cli/tui/components/`
- Package-level functions for crypto, gitrepo — wrapped by service interfaces for injection

### Naming

- Files: `snake_case.go`
- Test files: `snake_case_test.go` in the same package
- Benchmark files: `snake_case_bench_test.go`
- Mock files: `mock_*.go` or `*_mock.go`
- Service interfaces: `FooService` with methods like `Load`, `Save`, `Get`

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
- Git operations: `Clone()` returns instantly if already cloned. Use `Sync()` for smart updates (compares HEAD before pulling).
- Only sync joined pool repos, skip unjoined ones.

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

No service reaches into another's domain. Components read via services, never call package functions directly (aspiration — migration in progress).

## Debug Mode

```bash
DEBUG=1 dating
```

Shows timestamped logs in the TUI status bar:
- Profile rendering times
- Cache build durations
- Polling activity
- Screen navigation
- Persistence operations

Use `dbg.Log()` and `dbg.Timer()` from `internal/debug`.

## Common Gotchas

- **Config corruption**: Tests MUST use `DATING_HOME` temp dir. Bare `go test` writes to real config.
- **Viewport blank**: Screens need `WindowSizeMsg` before rendering. Send it explicitly when navigating to a screen.
- **Key events eaten**: The command input steals arrow/tab keys. Block forwarding for screens that need them.
- **Glamour slow**: Never call glamour in `Update()` or on mode switch. Cache once, swap strings.
- **Git clone on every view**: `Clone()` no longer pulls. Use `Sync()` only when needed.
- **Stale pool data**: The old `pollPendingPools` in `app.go` and the new `svc/polling.go` coexist. The TUI uses the old one. Migration pending.
