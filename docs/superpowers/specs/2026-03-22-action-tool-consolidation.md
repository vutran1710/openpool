# Action Tool Consolidation

## Goal

Merge `regcrypt`, `matchcrypt`, and `indexer` into a single `action-tool` binary. One download per Action, shared code linked once, ~6-7MB total instead of 18MB.

## Subcommands

```
action-tool register   # registration flow (was: regcrypt register)
action-tool match      # interest matching (was: matchcrypt match)
action-tool squash     # history squashing (was: bash in pool-squash.yml)
action-tool index      # rebuild index (was: indexer)
action-tool sign       # sign blob with operator key (was: regcrypt sign / matchcrypt sign)
action-tool decrypt    # decrypt encrypted body (was: matchcrypt decrypt)
action-tool pubkey     # extract pubkey from .bin (was: matchcrypt pubkey)
```

## File Structure

```
cmd/action-tool/
  main.go        → subcommand router (switch os.Args[1])
  register.go    → cmdRegister (from cmd/regcrypt)
  match.go       → cmdMatch (from cmd/matchcrypt)
  squash.go      → cmdSquash (new — replaces bash in pool-squash.yml)
  index.go       → cmdIndex (from cmd/indexer)
  sign.go        → cmdSign (deduplicated — was in both regcrypt + matchcrypt)
  decrypt.go     → cmdDecrypt (from matchcrypt)
  pubkey.go      → cmdPubkey (from matchcrypt)
  helpers.go     → shared: sha256Short, envOrArg, writeError
```

## Deleted

```
cmd/regcrypt/       → deleted entirely
cmd/matchcrypt/     → deleted entirely
cmd/indexer/        → deleted entirely
```

## Squash Subcommand (new Go implementation)

Replaces bash in `pool-squash.yml`:

```go
func cmdSquash() {
    maxStr := os.Getenv("MAX_COMMIT_COUNT")
    if maxStr == "" { maxStr = "50" }
    maxCount, _ := strconv.Atoi(maxStr)

    // Count commits
    out, _ := exec.Command("git", "rev-list", "--count", "HEAD").Output()
    count, _ := strconv.Atoi(strings.TrimSpace(string(out)))

    if count <= maxCount {
        log.Printf("commits: %d (max %d) — skipping", count, maxCount)
        return
    }

    log.Printf("squashing %d commits into 1...", count)

    exec.Command("git", "config", "user.name", "openpool-bot").Run()
    exec.Command("git", "config", "user.email", "bot@openpool.dev").Run()
    exec.Command("git", "checkout", "--orphan", "squashed").Run()
    exec.Command("git", "add", "-A").Run()
    exec.Command("git", "commit", "-m",
        fmt.Sprintf("Pool state (squashed %s)", time.Now().UTC().Format(time.RFC3339))).Run()
    exec.Command("git", "push", "--force", "origin", "squashed:main").Run()

    log.Printf("squashed: %d → 1", count)
}
```

## Action Templates (updated)

All templates download one binary:

### pool-register.yml

```yaml
- name: Download action-tool
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
    chmod +x action-tool

- name: Register
  env: { ... }
  run: ./action-tool register
```

### pool-interest.yml

```yaml
- name: Download action-tool
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
    chmod +x action-tool

- name: Match
  env: { ... }
  run: ./action-tool match
```

### pool-squash.yml

```yaml
- name: Download action-tool
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
    chmod +x action-tool

- name: Squash
  env:
    MAX_COMMIT_COUNT: ${{ vars.MAX_COMMIT_COUNT || '50' }}
    GH_TOKEN: ${{ github.token }}
  run: ./action-tool squash
```

### pool-indexer.yml

```yaml
- name: Download action-tool
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
    chmod +x action-tool

- name: Rebuild index
  env: { ... }
  run: ./action-tool index --rebuild --output index.pack
```

## Release

One asset: `action-tool-linux-amd64`

Published to `vutran1710/regcrypt` releases (same repo, replaces the three separate binaries). Old binaries (`regcrypt-linux-amd64`, `matchcrypt-linux-amd64`, `indexer-linux-amd64`) no longer published.

## Shared Code (deduplicated)

Currently duplicated across regcrypt + matchcrypt:

| Function | Currently in | action-tool location |
|----------|-------------|---------------------|
| `cmdSign` | regcrypt + matchcrypt | `sign.go` |
| `sha256Short` | regcrypt + matchcrypt | `helpers.go` |
| `envOrArg` / `envOrArgSign` | regcrypt + matchcrypt | `helpers.go` |
| `writeError` | regcrypt + matchcrypt | `helpers.go` |
| `encryptAndSign` | matchcrypt | `match.go` |

## Testing

- Build: `go build -o /tmp/action-tool ./cmd/action-tool/`
- Verify each subcommand: `./action-tool register --help` etc.
- Size check: should be ~6-7MB (vs 18MB for three separate binaries)
- E2E: deploy updated Actions to test pool, verify registration + matching + squash
- All existing tests still pass (internal packages unchanged)

## Migration

1. Create `cmd/action-tool/` with all subcommands
2. Update Action templates to download `action-tool-linux-amd64`
3. Delete `cmd/regcrypt/`, `cmd/matchcrypt/`, `cmd/indexer/`
4. Publish new release with single `action-tool-linux-amd64`
5. Deploy updated Actions to test pool
