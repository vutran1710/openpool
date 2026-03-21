# Action Tool Consolidation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge regcrypt, matchcrypt, and indexer into a single `action-tool` binary with subcommands. One download per Action, ~6-7MB.

**Architecture:** Create `cmd/action-tool/` with one file per subcommand. Move existing code from `cmd/regcrypt/`, `cmd/matchcrypt/`, `cmd/indexer/` — no logic changes, just reorganization + deduplication of shared helpers. Add `squash` subcommand (Go replacement for bash). Delete old `cmd/` directories.

**Tech Stack:** Go, os/exec (for git/gh), existing internal packages

---

## File Structure

### New Files
- `cmd/action-tool/main.go` — subcommand router
- `cmd/action-tool/register.go` — from regcrypt cmdRegister
- `cmd/action-tool/match.go` — from matchcrypt cmdMatch
- `cmd/action-tool/squash.go` — new Go implementation
- `cmd/action-tool/index.go` — from indexer main
- `cmd/action-tool/sign.go` — deduplicated cmdSign
- `cmd/action-tool/decrypt.go` — from matchcrypt cmdDecrypt
- `cmd/action-tool/pubkey.go` — from matchcrypt cmdPubkey
- `cmd/action-tool/helpers.go` — shared: sha256Short, envOrArg, writeError

### Deleted
- `cmd/regcrypt/` — entire directory
- `cmd/matchcrypt/` — entire directory
- `cmd/indexer/` — entire directory

### Modified
- `templates/actions/pool-register.yml` — download action-tool
- `templates/actions/pool-interest.yml` — download action-tool
- `templates/actions/pool-squash.yml` — download action-tool, run subcommand
- `templates/actions/pool-indexer.yml` — download action-tool

---

### Task 1: Create action-tool scaffolding + helpers

**Files:**
- Create: `cmd/action-tool/main.go`
- Create: `cmd/action-tool/helpers.go`

- [ ] **Step 1: Create main.go with subcommand router**

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "Usage: action-tool <register|match|squash|index|sign|decrypt|pubkey>")
        os.Exit(1)
    }

    switch os.Args[1] {
    case "register":
        cmdRegister()
    case "match":
        cmdMatch()
    case "squash":
        cmdSquash()
    case "index":
        cmdIndex()
    case "sign":
        cmdSign()
    case "decrypt":
        cmdDecrypt()
    case "pubkey":
        cmdPubkey()
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
        os.Exit(1)
    }
}
```

- [ ] **Step 2: Create helpers.go with shared functions**

Extract from regcrypt/matchcrypt — deduplicate:

```go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "os"
)

func sha256Short(input string) string {
    h := sha256.Sum256([]byte(input))
    return hex.EncodeToString(h[:])[:16]
}

func envOrArg(flag, envVar string) string {
    for i, arg := range os.Args {
        if arg == flag && i+1 < len(os.Args) {
            return os.Args[i+1]
        }
    }
    if envVar != "" {
        if v := os.Getenv(envVar); v != "" {
            return v
        }
    }
    return ""
}

func writeError(msg string) {
    fmt.Fprintf(os.Stderr, "error: %s\n", msg)
    os.Exit(1)
}
```

- [ ] **Step 3: Verify scaffolding compiles**

Create stub functions for all subcommands so it compiles:

```go
// stubs.go (temporary — removed as real implementations are added)
package main

func cmdRegister() { writeError("not implemented") }
func cmdMatch()    { writeError("not implemented") }
func cmdSquash()   { writeError("not implemented") }
func cmdIndex()    { writeError("not implemented") }
func cmdSign()     { writeError("not implemented") }
func cmdDecrypt()  { writeError("not implemented") }
func cmdPubkey()   { writeError("not implemented") }
```

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/
git commit -m "feat: action-tool scaffolding — subcommand router + helpers"
```

---

### Task 2: Move sign, decrypt, pubkey subcommands

**Files:**
- Create: `cmd/action-tool/sign.go`
- Create: `cmd/action-tool/decrypt.go`
- Create: `cmd/action-tool/pubkey.go`

- [ ] **Step 1: Copy cmdSign from regcrypt (or matchcrypt — they're identical)**

Read `cmd/regcrypt/main.go` and `cmd/matchcrypt/main.go`. Copy `cmdSign` to `sign.go`. It reads operator key from `--operator-key` flag or `OPERATOR_PRIVATE_KEY` env, reads stdin, signs, outputs hex.

- [ ] **Step 2: Copy cmdDecrypt from matchcrypt**

Read `cmd/matchcrypt/main.go`. Copy `cmdDecrypt` to `decrypt.go`.

- [ ] **Step 3: Copy cmdPubkey from matchcrypt**

Copy `cmdPubkey` to `pubkey.go`.

- [ ] **Step 4: Remove stubs for sign/decrypt/pubkey**

Delete the stub functions from stubs.go.

- [ ] **Step 5: Verify it compiles + test sign**

```bash
go build -o /tmp/action-tool ./cmd/action-tool/
echo -n "test" | /tmp/action-tool sign --operator-key $OPERATOR_PRIVATE_KEY
```

- [ ] **Step 6: Commit**

```bash
git add cmd/action-tool/
git commit -m "feat: action-tool sign/decrypt/pubkey subcommands"
```

---

### Task 3: Move register subcommand

**Files:**
- Create: `cmd/action-tool/register.go`

- [ ] **Step 1: Copy cmdRegister from regcrypt**

Read `cmd/regcrypt/main.go`. Copy `cmdRegister` to `register.go`. It reads env vars (ISSUE_BODY, ISSUE_NUMBER, etc.), parses issue body, computes hashes, encrypts, signs, writes .bin, git commits, closes + locks issue, posts comment.

Note: `cmdRegister` uses `sha256Short` and `envOrArg` — these are now in `helpers.go`. Remove any local definitions.

Also keep the legacy flag-based hash command — add it as a `hash` subcommand or keep the flag-based code as a fallback in `main.go` (for backward compat with old Actions that call `./regcrypt --pool-url ...`).

Actually — old Actions no longer exist (we updated them all). No backward compat needed. Just the `register` subcommand.

- [ ] **Step 2: Remove register stub**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/register.go
git commit -m "feat: action-tool register subcommand"
```

---

### Task 4: Move match subcommand

**Files:**
- Create: `cmd/action-tool/match.go`

- [ ] **Step 1: Copy cmdMatch + encryptAndSign from matchcrypt**

Read `cmd/matchcrypt/main.go`. Copy `cmdMatch` and `encryptAndSign` to `match.go`. It reads env vars, decrypts issue body, searches for reciprocal via CLIClient, creates match file, encrypts + signs notifications, git commits, closes + locks issues, posts comments.

Note: `sha256Short` is in `helpers.go` — remove local definition. Same for `writeError`.

- [ ] **Step 2: Remove match stub**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/match.go
git commit -m "feat: action-tool match subcommand"
```

---

### Task 5: Create squash subcommand (new Go implementation)

**Files:**
- Create: `cmd/action-tool/squash.go`

- [ ] **Step 1: Implement cmdSquash**

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
)

func cmdSquash() {
    maxStr := os.Getenv("MAX_COMMIT_COUNT")
    if maxStr == "" {
        maxStr = "50"
    }
    maxCount, err := strconv.Atoi(maxStr)
    if err != nil {
        writeError("invalid MAX_COMMIT_COUNT: " + maxStr)
    }

    // Count commits
    out, err := exec.Command("git", "rev-list", "--count", "HEAD").Output()
    if err != nil {
        writeError("counting commits: " + err.Error())
    }
    count, _ := strconv.Atoi(strings.TrimSpace(string(out)))

    if count <= maxCount {
        log.Printf("commits: %d (max %d) — skipping", count, maxCount)
        return
    }

    log.Printf("squashing %d commits into 1...", count)

    exec.Command("git", "config", "user.name", "openpool-bot").Run()
    exec.Command("git", "config", "user.email", "bot@openpool.dev").Run()

    if out, err := exec.Command("git", "checkout", "--orphan", "squashed").CombinedOutput(); err != nil {
        writeError("checkout orphan: " + string(out))
    }
    exec.Command("git", "add", "-A").Run()
    msg := fmt.Sprintf("Pool state (squashed %s)", time.Now().UTC().Format(time.RFC3339))
    exec.Command("git", "commit", "-m", msg).Run()

    if out, err := exec.Command("git", "push", "--force", "origin", "squashed:main").CombinedOutput(); err != nil {
        writeError("force push: " + string(out))
    }

    log.Printf("squashed: %d → 1", count)
}
```

- [ ] **Step 2: Remove squash stub**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/squash.go
git commit -m "feat: action-tool squash subcommand (replaces bash)"
```

---

### Task 6: Move index subcommand

**Files:**
- Create: `cmd/action-tool/index.go`

- [ ] **Step 1: Copy indexer main logic**

Read `cmd/indexer/main.go`. The indexer uses `flag.Parse()` with multiple flags. Wrap it as `cmdIndex()` that parses `os.Args[2:]` (skip the "index" subcommand).

Use `flag.NewFlagSet("index", flag.ExitOnError)` and parse `os.Args[2:]`.

- [ ] **Step 2: Remove index stub**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/index.go
git commit -m "feat: action-tool index subcommand"
```

---

### Task 7: Delete old cmd directories + remove stubs.go

**Files:**
- Delete: `cmd/regcrypt/`
- Delete: `cmd/matchcrypt/`
- Delete: `cmd/indexer/`
- Delete: `cmd/action-tool/stubs.go` (if still exists)

- [ ] **Step 1: Delete old directories**

```bash
rm -rf cmd/regcrypt cmd/matchcrypt cmd/indexer
rm -f cmd/action-tool/stubs.go
```

- [ ] **Step 2: Verify full build**

Run: `go build ./...`

- [ ] **Step 3: Verify binary size**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/action-tool-linux ./cmd/action-tool/
ls -lh /tmp/action-tool-linux
# Should be ~6-7MB
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "delete: remove cmd/regcrypt, cmd/matchcrypt, cmd/indexer"
```

---

### Task 8: Update Action templates

**Files:**
- Modify: `templates/actions/pool-register.yml`
- Modify: `templates/actions/pool-interest.yml`
- Modify: `templates/actions/pool-squash.yml`
- Modify: `templates/actions/pool-indexer.yml`

- [ ] **Step 1: Update all templates**

Change the download step in each to:

```yaml
- name: Download action-tool
  run: |
    curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
    chmod +x action-tool
```

And the run step:
- `pool-register.yml`: `./action-tool register`
- `pool-interest.yml`: `./action-tool match`
- `pool-squash.yml`: `./action-tool squash` (remove all bash logic, just run the subcommand)
- `pool-indexer.yml`: `./action-tool index --rebuild --output index.pack` (check current indexer flags)

For `pool-squash.yml`, simplify to:

```yaml
name: Squash History

on:
  schedule:
    - cron: '0 * * * *'
  workflow_dispatch:

jobs:
  squash:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Download action-tool
        run: |
          curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/action-tool-linux-amd64 -o action-tool
          chmod +x action-tool

      - name: Squash
        env:
          MAX_COMMIT_COUNT: ${{ vars.MAX_COMMIT_COUNT || '50' }}
        run: ./action-tool squash
```

- [ ] **Step 2: Commit**

```bash
git add templates/actions/
git commit -m "refactor: Action templates use action-tool binary"
```

---

### Task 9: Update CLAUDE.md + Binaries table

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update Architecture tree**

Replace `cmd/regcrypt/`, `cmd/indexer/`, `cmd/matchcrypt/` with `cmd/action-tool/`.

- [ ] **Step 2: Update Binaries table**

```markdown
| Binary | Purpose | Distribution |
|--------|---------|-------------|
| `dating` | CLI app | End users |
| `relay` | Per-pool relay server | Pool operators (Railway) |
| `action-tool` | All Action operations (register, match, squash, index) | GitHub Actions |
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md — action-tool replaces regcrypt/matchcrypt/indexer"
```

---

### Task 10: Full test suite + build verification

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`
Expected: PASS (internal packages unchanged)

- [ ] **Step 2: Build and verify size**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/action-tool-linux ./cmd/action-tool/
ls -lh /tmp/action-tool-linux
```

- [ ] **Step 3: Test each subcommand locally**

```bash
./action-tool register --help  # or just run with no env → should error gracefully
./action-tool match             # should error gracefully (no env vars)
./action-tool squash            # should say "commits: N (max 50) — skipping"
./action-tool sign              # should error (no key)
./action-tool decrypt           # should error (no key)
./action-tool pubkey            # should error (no bin file)
./action-tool index             # should show usage
```

- [ ] **Step 4: Commit**

```bash
git commit -m "test: all tests pass, action-tool binary verified"
```

---

### Task 11: Publish release + deploy Actions + E2E test

- [ ] **Step 1: Publish release**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/action-tool-linux-amd64 ./cmd/action-tool/
gh release create action-tool-v1.0.0 \
  /tmp/action-tool-linux-amd64 \
  --repo vutran1710/regcrypt \
  --title "action-tool v1.0.0 — unified Action binary" \
  --notes "Replaces regcrypt, matchcrypt, indexer. Subcommands: register, match, squash, index, sign, decrypt, pubkey." \
  --latest
```

- [ ] **Step 2: Deploy Actions to test pool**

```bash
cp templates/actions/*.yml /path/to/dating-test-pool/.github/workflows/
cd /path/to/dating-test-pool && git add -A && git commit -m "update Actions: action-tool v1.0.0" && git push
```

- [ ] **Step 3: E2E test — registration**

Create a registration issue on test pool, verify Action runs `action-tool register` successfully.

- [ ] **Step 4: E2E test — interest matching**

Create two interest issues, verify `action-tool match` detects mutual match.

- [ ] **Step 5: E2E test — squash (manual trigger)**

Trigger squash workflow manually, verify it squashes if above threshold.

- [ ] **Step 6: Commit**

```bash
git commit -m "test: E2E verified action-tool in production"
```
