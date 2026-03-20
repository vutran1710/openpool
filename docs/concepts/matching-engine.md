# Matching Engine Redesign

## Status: Concept — pool schema shape TBD

## Core Ideas

- **Everything is a score [0, 1]** — filters are fields with `min: 1.0`, preferences are fields with `min: 0.0`
- **5 scoring primitives**: exact, complementary, proximity, similarity, overlap
- **3 ranking strategies**: top_score, tiered_shuffle, weighted_random (pool-configurable)
- **3 matching modes**: pool-controlled, user-controlled, hybrid (defaults + user overrides, lockable)
- **User match preferences** — users define their own rules against pool attributes
- **Pool schema uses YAML** (`pool.yaml`) — `attributes` is the root key for required fields
- Display fields (display_name, about) are just attributes, not special columns

## Storage

- Indexer produces `index.db` (SQLite) instead of `index.pack` (msgpack)
- Client maintains local `suggestions.db` with profiles, scores, seen tables
- Sync via `ATTACH DATABASE` — upsert profiles, invalidate stale scores
- Seen entries have timestamps + configurable cooldown (not permanent booleans)

## TUI

- Adaptive rendering — forms generated dynamically from pool schema
- Attribute types map to TUI components (enum → radio, multi → checkbox, range → number input)
- Match preference editor (user/hybrid mode), locked fields dimmed in hybrid mode

## Open

- Pool schema overall shape — needs to feel intuitive for non-technical pool operators
- How roles (employer/candidate) fit in the schema
- Relationship between attributes block and matching block

## Reference

- Draft spec: `docs/superpowers/specs/2026-03-20-openpool-matching-engine-draft.md`
- Current implementation: `internal/github/vector.go`, `internal/cli/suggestions/rank.go`
