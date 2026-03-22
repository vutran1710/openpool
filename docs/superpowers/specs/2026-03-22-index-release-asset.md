# Index as Release Asset

## Goal

Move `index.pack` from git-committed file to GitHub release asset. Indexer uploads on rebuild, client downloads from release URL. Reduces repo bloat, decouples index from git history.

## Design

### Indexer (`action-tool index --rebuild --upload`)

New `--upload` flag. After building `index.pack`, uses CLIClient to upload as release asset:

```
action-tool index --rebuild --upload
  1. Read pool.json, decrypt .bin files, build index.pack (existing)
  2. If --upload: gh.UploadReleaseAsset("index-latest", "index.pack")
```

### GitHubClient Interface

```go
UploadReleaseAsset(ctx context.Context, tag, assetName, filePath string) error
```

CLIClient implementation:
```go
func (c *CLIClient) UploadReleaseAsset(ctx context.Context, tag, assetName, filePath string) error {
    // Create release if not exists
    c.gh("release", "view", tag, "-R", c.repo)  // check
    c.gh("release", "create", tag, "-R", c.repo, "--title", tag, "--notes", "Auto-updated")  // create if missing
    // Upload with --clobber to overwrite
    c.gh("release", "upload", tag, filePath, "--clobber", "-R", c.repo)
}
```

### Client Download

```go
// internal/github/download.go
func DownloadReleaseAsset(repoURL, tag, assetName, destPath string) error {
    url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, tag, assetName)
    resp, err := http.Get(url)
    // write to destPath
}
```

### Client Flow Change

```
Current:  repo.Sync() → read index.pack from repo.LocalDir
New:      DownloadReleaseAsset(pool.Repo, "index-latest", "index.pack", localPath)
```

The pool repo is still cloned for pool.json, .bin files, etc. Only index.pack moves to release.

### Action Template

```yaml
run: ./action-tool index --rebuild --upload
```

One line. No git commit step.

## Files

| File | Change |
|------|--------|
| `internal/github/github.go` | Add `UploadReleaseAsset` to interface |
| `internal/github/cli.go` | Implement UploadReleaseAsset |
| `internal/github/http.go` | Implement UploadReleaseAsset |
| Create: `internal/github/download.go` | DownloadReleaseAsset (HTTP GET) |
| `cmd/action-tool/index.go` | Add `--upload` flag |
| `internal/cli/sync.go` | Download from release |
| `internal/cli/tui/screens/discover.go` | Download from release |
| `templates/actions/pool-indexer.yml` | One line run |
