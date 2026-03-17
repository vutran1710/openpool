# TUI Settings: Active Pool & Registry

## Problem

The TUI treats pools as a browseable list but doesn't enforce "one active pool at a time." The header doesn't show which registry is active. There's no way to switch pool or registry without navigating through the pools screen.

## Design

### 1. Header (Two-Row Status Bar)

Row 1: `♥ dating.dev` left, `⬡ Alice ab12` right.
Row 2: `◈ berlin-singles` and `⊞ vutran1710/dating-registry` right-aligned.

Empty states: `◈ no pool` / `⊞ no registry` in muted text. Updates live on switch.

Add `Registry string` field to `StatusBar` component.

### 2. Home Menu — Settings Entry

Replace current menu items with:
- Discover, Matches, Profile, Pools, **Settings**

"Settings" opens a dedicated settings screen (submenu pattern). Remove "Identity" from home menu — it moves into Settings.

### 3. Settings Screen (new)

New screen: `internal/cli/tui/screens/settings.go`

Three cards, navigable with ↑↓:
- **Active Pool** — shows current pool name, Enter opens a picker (list of joined pools from config)
- **Active Registry** — shows current registry URL, Enter opens a picker (list of registries from config)
- **Identity** — shows user name + provider, Enter shows identity details

Each card shows a hint: "Switch with `/pool <name>`" etc.

When user selects a pool/registry from the picker:
- Update `config.Active` / `config.ActiveRegistry`
- Save config
- Emit a message to app.go to update header + trigger soft reset (screens refetch data lazily on next view)

### 4. Soft Reset on Pool Switch

When active pool changes:
- Update `app.pool` and status bar
- Mark discover/matches/chat screens as "needs refresh" (set `loaded = false`)
- Screens refetch data on next view (existing lazy-load pattern)
- No state is cleared immediately — just invalidated

### 5. Slash Commands

Add to command palette (separated section with divider):
- `/pool <name>` — switch active pool
- `/registry <name>` — switch active registry
- `/settings` — navigate to settings screen

These appear in the palette after a divider line, styled in a different color (orange) to distinguish from navigation commands.

When `/pool <name>` is typed:
- Look up pool by name in `config.Pools`
- If found and status == "active": switch, save config, update header, soft reset
- If not found: show toast error "Pool not found"
- If not active status: show toast error "Pool not joined"

Same logic for `/registry <name>` against `config.Registries`.

### 6. App State Changes

`app` struct gains:
- `settings screens.SettingsScreen` — new screen instance
- `screenSettings` added to `activeScreen` enum

`app.pool` and `app.registry` update from:
- Settings screen picker selection
- Slash command `/pool` and `/registry`
- Both paths emit the same message type (`PoolSwitchMsg` / `RegistrySwitchMsg`)

## File Changes

| File | Changes |
|------|---------|
| `internal/cli/tui/components/statusbar.go` | Add `Registry` field, render two-row layout |
| `internal/cli/tui/components/input.go` | Add `/pool`, `/registry`, `/settings` to command palette with divider |
| `internal/cli/tui/screens/settings.go` | Create — settings screen with pool/registry/identity cards + pickers |
| `internal/cli/tui/screens/home.go` | Replace "Identity" menu item with "Settings" |
| `internal/cli/tui/app.go` | Add settings screen, handle PoolSwitchMsg/RegistrySwitchMsg, soft reset logic, route `/pool`/`/registry` commands |
| `internal/cli/config/config.go` | No changes needed (ActivePool/ActiveRegistry already exist) |

## Testing

- Settings screen renders with current pool/registry
- Pool switch updates header + invalidates screens
- Registry switch updates header
- `/pool <name>` command works from input
- `/pool <unknown>` shows error toast
- `/settings` navigates to settings screen
- Esc from settings returns to home
