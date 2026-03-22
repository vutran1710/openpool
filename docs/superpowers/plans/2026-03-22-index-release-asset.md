# Index as Release Asset Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move index.pack from git-committed file to GitHub release asset. Indexer uploads, client downloads.

**Architecture:** Add `UploadReleaseAsset` to GitHubClient interface. Add `--upload` flag to `action-tool index`. Add `DownloadReleaseAsset` for client. Update sync/discover to download from release.

**Tech Stack:** Go, GitHub Releases API, `gh release` CLI

---

### Task 1: Add UploadReleaseAsset to GitHubClient + DownloadReleaseAsset

**Files:**
- Modify: `internal/github/github.go`
- Modify: `internal/github/cli.go`
- Modify: `internal/github/client.go`
- Create: `internal/github/download.go`
- Modify: `internal/github/comments_test.go` (mock stub)

- [ ] **Step 1: Add to interface**

```go
// In github.go, add to GitHubClient:
UploadReleaseAsset(ctx context.Context, tag, assetName, filePath string) error
```

- [ ] **Step 2: Implement on CLIClient**

```go
func (c *CLIClient) UploadReleaseAsset(_ context.Context, tag, assetName, filePath string) error {
    // Create release if it doesn't exist (ignore error if exists)
    c.gh("release", "create", tag, "-R", c.repo,
        "--title", tag, "--notes", "Auto-updated by indexer")
    // Upload with --clobber to overwrite existing asset
    _, err := c.gh("release", "upload", tag, filePath, "--clobber", "-R", c.repo)
    return err
}
```

- [ ] **Step 3: Implement on HTTPClient**

```go
func (c *HTTPClient) UploadReleaseAsset(ctx context.Context, tag, assetName, filePath string) error {
    // This is complex via REST API (multipart upload). For now, return error.
    return fmt.Errorf("UploadReleaseAsset not supported on HTTPClient — use CLIClient")
}
```

- [ ] **Step 4: Create download.go**

```go
package github

import (
    "fmt"
    "io"
    "net/http"
    "os"
)

// DownloadReleaseAsset downloads a release asset from a public GitHub repo.
func DownloadReleaseAsset(repoURL, tag, assetName, destPath string) error {
    url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, tag, assetName)
    resp, err := http.Get(url)
    if err != nil {
        return fmt.Errorf("downloading %s: %w", assetName, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == 404 {
        return fmt.Errorf("release asset not found: %s/%s/%s", repoURL, tag, assetName)
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
    }

    f, err := os.Create(destPath)
    if err != nil {
        return fmt.Errorf("creating file: %w", err)
    }
    defer f.Close()

    _, err = io.Copy(f, resp.Body)
    return err
}
```

- [ ] **Step 5: Add mock stub in comments_test.go**

```go
func (m *mockCommentsClient) UploadReleaseAsset(_ context.Context, _, _, _ string) error { panic("not implemented") }
```

- [ ] **Step 6: Verify build**

Run: `go build ./internal/github/`

- [ ] **Step 7: Commit**

```bash
git add internal/github/
git commit -m "feat: UploadReleaseAsset + DownloadReleaseAsset"
```

---

### Task 2: Add --upload flag to action-tool index

**Files:**
- Modify: `cmd/action-tool/index.go`

- [ ] **Step 1: Read current index.go**

Understand the existing flag parsing and index rebuild flow.

- [ ] **Step 2: Add --upload flag**

After the existing index build logic, add:

```go
// Parse --upload flag from os.Args[2:]
uploadFlag := false
for _, arg := range os.Args[2:] {
    if arg == "--upload" {
        uploadFlag = true
    }
}

// After building index.pack:
if uploadFlag {
    repo := os.Getenv("REPO")
    if repo == "" {
        writeError("REPO env var required for --upload")
    }
    gh, err := github.NewCLI(repo)
    if err != nil {
        writeError("github CLI: " + err.Error())
    }
    ctx := context.Background()
    if err := gh.UploadReleaseAsset(ctx, "index-latest", "index.pack", outputPath); err != nil {
        writeError("uploading index: " + err.Error())
    }
    log.Printf("uploaded index.pack as release asset")
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/action-tool/`

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/index.go
git commit -m "feat: action-tool index --upload — uploads index.pack as release asset"
```

---

### Task 3: Update client to download from release

**Files:**
- Modify: `internal/cli/sync.go`
- Modify: `internal/cli/tui/screens/discover.go`

- [ ] **Step 1: Update sync.go**

Replace reading index.pack from git clone with downloading from release:

```go
// Replace:
indexPackPath := filepath.Join(repo.LocalDir, "index.pack")
if _, err := os.Stat(indexPackPath); err == nil {
    loaded, err := pack.SyncFromIndexPack(indexPackPath)

// With:
indexPackPath := filepath.Join(config.Dir(), "pools", poolName, "index.pack")
if err := gh.DownloadReleaseAsset(pool.Repo, "index-latest", "index.pack", indexPackPath); err == nil {
    loaded, err := pack.SyncFromIndexPack(indexPackPath)
```

Import `github.com/vutran1710/dating-dev/internal/github` for `DownloadReleaseAsset`.

- [ ] **Step 2: Update discover.go**

Same change in the discover screen's `backgroundSync` and initial load:

```go
// Download index.pack from release
indexPath := filepath.Join(poolDir, "index.pack")
github.DownloadReleaseAsset(poolRepo, "index-latest", "index.pack", indexPath)
pack.SyncFromIndexPack(indexPath)
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Run full tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/cli/sync.go internal/cli/tui/screens/discover.go
git commit -m "refactor: client downloads index.pack from release asset"
```

---

### Task 4: Update Action template

**Files:**
- Modify: `templates/actions/pool-indexer.yml`

- [ ] **Step 1: Already updated** — verify the template is:

```yaml
run: ./action-tool index --rebuild --upload
```

No git commit step.

- [ ] **Step 2: Commit if changed**

```bash
git add templates/actions/pool-indexer.yml
git commit -m "refactor: indexer Action uploads release asset, no git commit"
```

---

### Task 5: Full test suite + E2E

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 2: Build + publish action-tool**

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/action-tool-linux-amd64 ./cmd/action-tool/
gh release upload action-tool-v1.0.0 /tmp/action-tool-linux-amd64 --clobber --repo vutran1710/regcrypt
```

- [ ] **Step 3: Deploy Actions to test pool**

```bash
cp templates/actions/pool-indexer.yml /path/to/dating-test-pool/.github/workflows/pool-indexer.yml
```

- [ ] **Step 4: Trigger indexer manually**

```bash
gh workflow run "Rebuild Index" --repo vutran1710/dating-test-pool
```

- [ ] **Step 5: Verify release asset exists**

```bash
gh release view index-latest --repo vutran1710/dating-test-pool --json assets
```

- [ ] **Step 6: Verify client can download**

```bash
curl -sI https://github.com/vutran1710/dating-test-pool/releases/download/index-latest/index.pack
```

- [ ] **Step 7: Commit**

```bash
git commit -m "test: E2E verified index as release asset"
```
