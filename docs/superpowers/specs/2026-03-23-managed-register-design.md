# Managed Account Registration — Design Spec

## Summary

Add `action-tool managed-register` command that lets pool operators create user accounts directly. Bypasses the issue-based registration flow — commits `.bin` file straight to the pool repo and outputs a ready-to-use `DATING_HOME` bundle.

## Motivation

- Testing requires multiple user accounts but registration is tied to GitHub identity
- Operators need to onboard users from other providers (Google, email, etc.)
- The issue-based flow is unnecessary when the operator has direct repo access and secrets

## Command

```bash
action-tool managed-register \
  --provider google \
  --userid user@example.com \
  --profile ./profile.json \
  --pool vutran1710/dating-test-pool \
  --schema ./pool.yaml \
  --output-dir /tmp/user-x
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--provider` | yes | Identity provider (e.g. `google`, `email`, `managed`) |
| `--userid` | yes | User identifier within that provider |
| `--profile` | yes | Path to JSON profile file |
| `--pool` | yes | Pool repo (`owner/repo`) |
| `--schema` | no | Path to pool.yaml (default: `pool.yaml` in current dir) |
| `--output-dir` | yes | Directory to write the DATING_HOME bundle |

### Environment Variables

| Var | Required | Description |
|-----|----------|-------------|
| `POOL_SALT` | yes | Pool's secret salt for hash computation |
| `OPERATOR_PRIVATE_KEY` | yes | Operator's ed25519 private key (128 hex chars) |

## Flow

1. **Parse** flags and env vars. Exit with usage if missing.
2. **Load schema** from `--schema` path (or `pool.yaml` in cwd).
3. **Validate profile** against schema. Exit with errors if invalid.
4. **Generate keypair** — ed25519, saved as hex to `<output-dir>/keys/identity.{pub,key}`.
5. **Compute hash chain**:
   - `id_hash = sha256(pool:provider:userid)` (64 hex chars)
   - `bin_hash = sha256(salt:id_hash)[:16]` (16 hex chars)
   - `match_hash = sha256(salt:bin_hash)[:16]` (16 hex chars)
6. **Encrypt profile** — `crypto.PackUserBin(pubkey, operatorPub, profileJSON)` → `.bin` data.
7. **Clone pool repo** (shallow) if not already cloned. Write `users/<bin_hash>.bin`.
8. **Commit and push** via `AddCommitPush(["users/"], "Register managed user <bin_hash>")`.
9. **Read pool.yaml** from cloned repo for relay_url and operator_public_key.
10. **Write bundle** to `<output-dir>/`:
    - `keys/identity.pub` — hex-encoded ed25519 public key
    - `keys/identity.key` — hex-encoded ed25519 private key
    - `pools/<pool-name>/profile.json` — copy of the validated profile
    - `setting.toml` — complete config with pool entry (name, repo, operator key, relay_url, bin_hash, match_hash, status=active)
11. **Print summary** — bin_hash, match_hash, output dir.

## Output Bundle

```
<output-dir>/
├── keys/
│   ├── identity.pub
│   └── identity.key
├── pools/
│   └── <pool-name>/
│       └── profile.json
└── setting.toml
```

Usage after creation:
```bash
DATING_HOME=<output-dir> dating
```

## Config Format

```toml
active_pool = '<pool-name>'
registries = []
active_registry = ''

[user]
id_hash = '<id_hash>'
display_name = '<from profile or userid>'
username = '<userid>'
provider = '<provider>'
provider_user_id = '<userid>'
encrypted_token = ''

[[pools]]
name = '<pool-name>'
repo = '<owner/repo>'
operator_public_key = '<from pool.yaml>'
relay_url = '<from pool.yaml>'
status = 'active'
bin_hash = '<bin_hash>'
match_hash = '<match_hash>'
```

Note: `encrypted_token` is empty — managed accounts don't need GitHub auth for chat (TOTP uses the ed25519 keypair directly). They can't create issues or use GitHub API features.

## Error Handling

| Error | Behavior |
|-------|----------|
| Missing flags/env vars | Print usage, exit 1 |
| Profile file not found | Print error, exit 1 |
| Schema validation fails | Print field-level errors, exit 1 |
| Pool repo clone fails | Print error, exit 1 |
| Git push fails | Print error, exit 1 (bin file not committed) |
| Output dir exists with keys | Print error, exit 1 (prevent overwrite) |

## Implementation

Single file: `cmd/action-tool/managed.go`

Registered in `cmd/action-tool/main.go` alongside `register`, `match`, `squash`, `index`.

Reuses:
- `internal/crypto` — `GenerateKeyPair`, `PackUserBin`, `UserHash`
- `internal/schema` — `Load`, `ValidateProfile`
- `internal/github` — `NewCLI`, `AddCommitPush`
- `internal/gitrepo` — `Clone`, `EnsureGitURL`

## Testing

- Unit test: validate profile → reject invalid, accept valid
- Unit test: hash chain computation matches expected values
- Integration test: run against test pool, verify `.bin` committed, bundle usable with `dating chat`
