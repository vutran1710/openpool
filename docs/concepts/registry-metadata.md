# Registry Metadata & Branding

## Status: Concept

## Core Ideas

- Registry defines the "app skin" — domain, branding, personality
- The CLI binary is `openpool`, but it morphs based on active registry
- TUI title bar: `openpool · <3 dating` or `openpool · DevHire`

## Registry Metadata

```yaml
# registry.yaml (in the registry GitHub repo)
name: "<3 dating"
tagline: "terminal-native dating for developers"
theme:
  accent: pink
pools:
  - owner/berlin-devs
  - owner/remote-nerds
```

## What Changes

- TUI title, accent color, tagline adapt per registry
- Pool list scoped to registry
- Same CLI binary, different experience per registry

## Scope

Small — mostly theme injection into existing TUI components. The registry repo already exists as a concept (`dating-pool-registry`), just needs a `registry.yaml` with metadata.
