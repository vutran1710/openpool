# Project Rename: dating-dev → openpool

## Status: Concept

## Scope

- Go module: `github.com/vutran1710/openpool`
- CLI binary: `openpool`
- Config dir: `~/.openpool/`
- GitHub repo: `vutran1710/openpool`

## What Changes

- All import paths (`github.com/vutran1710/dating-dev` → `github.com/vutran1710/openpool`)
- Config directory (`~/.dating/` → `~/.openpool/`)
- Binary name (`dating` → `openpool`)
- DATING_HOME env var → OPENPOOL_HOME
- All docs, README, comments referencing "dating"

## What Stays

- Architecture (relay, crypto, GitHub-as-database)
- All existing functionality
- Test pool repo (can stay as `dating-test-pool` or rename later)

## When

After matching engine redesign is settled — rename is a clean break, better to do it when the API surface is stable.
