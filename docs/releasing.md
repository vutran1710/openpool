# Releasing

## How to Release

1. Ensure `main` is clean and all tests pass:
   ```bash
   make test
   ```

2. Create and push a version tag:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. The release workflow triggers automatically:
   - Verifies tag is on `main` branch
   - Runs full test suite with `-race`
   - Cross-compiles all binaries
   - Generates SHA256 checksums
   - Creates GitHub release with auto-generated notes

4. Verify the release at `https://github.com/vutran1710/openpool/releases`

## Versioning

Follows [Semantic Versioning](https://semver.org/):

```
v{MAJOR}.{MINOR}.{PATCH}

MAJOR — breaking changes (schema format, wire protocol, config format)
MINOR — new features (new commands, new pool.yaml fields)
PATCH — bug fixes, performance improvements
```

Pre-1.0: breaking changes may happen on minor bumps.

## Release Artifacts

| Artifact | Platforms | Description |
|----------|-----------|-------------|
| `op-{os}-{arch}` | linux/darwin × amd64/arm64 | CLI/TUI app |
| `relay-{os}-{arch}` | linux × amd64/arm64 | Relay server |
| `action-tool-linux-amd64` | linux amd64 | GitHub Actions operator tool |
| `checksums.txt` | — | SHA256 checksums for all artifacts |

## action-tool for Pool Repos

Pool repos download `action-tool` from a separate release repo (`vutran1710/regcrypt`). After a release, update the action-tool there:

```bash
# Copy from the openpool release
gh release download v0.1.0 --repo vutran1710/openpool --pattern "action-tool-linux-amd64" --dir /tmp
gh release upload action-tool-v1.0.0 /tmp/action-tool-linux-amd64 --clobber --repo vutran1710/regcrypt
```

Or update pool Action templates to download from the openpool release directly.

## CI Workflow

Runs on every push to `main` and every PR:

- `go vet ./...`
- `go test ./... -race`
- Build verification (all 3 binaries)

## Hotfix Process

1. Fix on `main`
2. Tag with patch bump: `v0.1.1`
3. Push tag — release builds automatically
