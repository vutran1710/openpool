# Action Tool Refactor

## Goal

Move bash logic from GitHub Action templates into Go tools (`regcrypt`, `matchcrypt`). Introduce a reusable `message` package for structured comment formatting. Action templates become thin wrappers: download tool, run, git commit/push, post comment.

## Message Package

### `internal/message/message.go`

```go
// Format creates a structured message block.
//
//   <!-- openpool:{blockType} -->
//   ```
//   {content}
//   ```
func Format(blockType, content string) string

// Parse extracts block type and content from a structured message.
// Returns error if the message doesn't match the expected format.
func Parse(raw string) (blockType, content string, err error)
```

### Block Types

| Block Type | Producer | Consumer | Content |
|-----------|----------|----------|---------|
| `registration-request` | CLI (`pool join`) | Action (`regcrypt register`) | pubkey + profile blob |
| `registration` | `regcrypt register` | CLI (polling) | `{base64_encrypted}.{hex_sig}` |
| `match` | `matchcrypt match` | CLI (matches screen) | `{base64_encrypted}.{hex_sig}` |


Errors: tools write to stderr and `os.Exit(1)`. The Action fails visibly — no error comments needed.

### Usage

```go
// Producer (regcrypt register)
comment := message.Format("registration", signedBlob)
os.WriteFile(".output", []byte(comment), 0644)

// Consumer (CLI polling)
blockType, content, err := message.Parse(comment.Body)
if blockType == "registration" {
    // content is the signed blob — verify + decrypt
}

// Issue body (CLI pool join)
body := message.Format("registration-request", pubkey+"\n"+blobHex)
```

## regcrypt register

### Current

`regcrypt` takes flags (`--pool-url`, `--salt`, etc.), outputs 3 lines to stdout. All parsing, file writing, git, and gh calls are in bash.

### New

`regcrypt register` reads env vars, does all computation, writes output files. No stdout for results.

```
Inputs (env vars):
  ISSUE_BODY             raw issue body markdown
  ISSUE_AUTHOR_ID        GitHub user ID (numeric)
  ISSUE_AUTHOR           GitHub username
  REPO                   "owner/repo"
  POOL_SALT              pool salt
  OPERATOR_PRIVATE_KEY   operator ed25519 key hex

Logic:
  1. Parse ISSUE_BODY using message.Parse("registration-request") → extract pubkey + blob hex
  2. Determine provider/user_id:
     - If ISSUE_AUTHOR == repo owner AND custom ID present → use custom ID
     - Else → provider=github, user_id=ISSUE_AUTHOR_ID
  3. Compute id_hash → bin_hash → match_hash (using POOL_SALT)
  4. Encrypt {bin_hash, match_hash} to user's pubkey (msgpack + NaCl box)
  5. Sign ciphertext with operator key
  6. Write users/{bin_hash}.bin (pubkey + profile blob)
  7. Write .output using message.Format("registration", signedBlob)

On error:
  Write to stderr and os.Exit(1). Action fails visibly.

Outputs:
  File: users/{bin_hash}.bin       binary profile
  File: .output                    comment body to post (ready to use)
```

The existing `regcrypt` hash subcommand (flag-based) is kept for backward compat. The `sign` subcommand is kept. `register` is a new subcommand.

### Action Template (after)

```yaml
name: Process Registration

on:
  issues:
    types: [opened]

jobs:
  register:
    if: contains(github.event.issue.body, '<!-- openpool:registration-request -->')
    runs-on: ubuntu-latest
    permissions:
      contents: write
      issues: write
    steps:
      - uses: actions/checkout@v4

      - name: Download regcrypt
        run: |
          curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/regcrypt-linux-amd64 -o regcrypt
          chmod +x regcrypt

      - name: Register
        env:
          ISSUE_BODY: ${{ github.event.issue.body }}
          ISSUE_NUMBER: ${{ github.event.issue.number }}
          ISSUE_AUTHOR_ID: ${{ github.event.issue.user.id }}
          ISSUE_AUTHOR: ${{ github.event.issue.user.login }}
          REPO: ${{ github.repository }}
          POOL_SALT: ${{ secrets.POOL_SALT }}
          OPERATOR_PRIVATE_KEY: ${{ secrets.OPERATOR_PRIVATE_KEY }}
          GH_TOKEN: ${{ github.token }}
        run: |
          ./regcrypt register
          git config user.name "openpool-bot"
          git config user.email "bot@openpool.dev"
          git add users/
          git commit -m "Register user"
          for i in 1 2 3; do
            git pull --rebase origin main && git push && break
            sleep 2
          done
          gh issue close "$ISSUE_NUMBER" --reason completed
          gh issue comment "$ISSUE_NUMBER" --body "$(cat .output)"
```

## matchcrypt match

### Current

Multiple `matchcrypt` subcommand calls (`decrypt`, `pubkey`, `encrypt`, `sign`) orchestrated by bash with `jq` parsing, string manipulation, and env var juggling.

### New

`matchcrypt match` reads env vars, does all computation, writes output files.

```
Inputs (env vars):
  PR_BODY              current PR body (encrypted base64)
  PR_NUMBER            current PR number
  PR_TITLE             current PR title (target match_hash)
  RECIP_PRS            JSON array from gh pr list (open interest PRs)
  REPO                 "owner/repo"
  OPERATOR_PRIVATE_KEY operator key hex
  POOL_SALT            pool salt

Logic:
  1. Decrypt PR_BODY with operator key → {author_bin_hash, author_match_hash, greeting}
  2. Search RECIP_PRS JSON for PR with title == author_match_hash
  3. If not found → no output files, exit 0
  4. If found:
     a. Decrypt reciprocal PR body → {recip_bin_hash, recip_match_hash, greeting}
     b. Read pubkeys from users/{bin_hash}.bin files (first 32 bytes)
     c. Compute pair_hash
     d. Write matches/{pair_hash}.json
     e. Encrypt + sign notification for author (peer's match_hash + pubkey + greeting)
     f. Encrypt + sign notification for reciprocal
     g. Write .output.{PR_NUMBER} with message.Format("match", authorSignedBlob)
     h. Write .output.{RECIP_NUMBER} with message.Format("match", recipSignedBlob)

Exit code: 0 always
```

### Action Template (after)

```yaml
name: Process Interest

on:
  pull_request:
    types: [opened]

jobs:
  match:
    if: contains(github.event.pull_request.labels.*.name, 'interest')
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4

      - name: Download matchcrypt
        run: |
          curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/matchcrypt-linux-amd64 -o matchcrypt
          chmod +x matchcrypt

      - name: Match
        env:
          PR_BODY: ${{ github.event.pull_request.body }}
          PR_NUMBER: ${{ github.event.pull_request.number }}
          PR_TITLE: ${{ github.event.pull_request.title }}
          OPERATOR_PRIVATE_KEY: ${{ secrets.OPERATOR_PRIVATE_KEY }}
          POOL_SALT: ${{ secrets.POOL_SALT }}
          REPO: ${{ github.repository }}
          GH_TOKEN: ${{ github.token }}
        run: |
          RECIP_PRS=$(gh pr list --repo "$REPO" --state open --label interest --json number,title,body)
          RECIP_PRS="$RECIP_PRS" ./matchcrypt match
          if ! ls .output.* 1>/dev/null 2>&1; then
            echo "No reciprocal interest found."
            exit 0
          fi
          git config user.name "openpool-bot"
          git config user.email "bot@openpool.dev"
          git add matches/
          git commit -m "Match" || true
          for i in 1 2 3; do
            git pull --rebase origin main && git push && break
            sleep 2
          done
          for f in .output.*; do
            PR_NUM="${f##*.output.}"
            gh pr close "$PR_NUM" --repo "$REPO" || true
            gh pr comment "$PR_NUM" --repo "$REPO" --body "$(cat $f)"
          done
```

## CLI Changes

### Issue Body (pool join)

Currently uses `<!-- registration-request -->` + `**Public Key:**` headers. Change to:

```go
body := message.Format("registration-request", pubkeyHex+"\n"+blobHex)
```

The `regcrypt register` command parses this with `message.Parse`.

### Registration Polling

Currently checks `c.User.Login == "github-actions[bot]"` then tries base64 decode. Change to:

```go
blockType, content, err := message.Parse(c.Body)
if err != nil || blockType != "registration" {
    continue
}
// content is the signed blob — verify signature + decrypt
```

### Match Notification Decryption

Currently tries base64 decode on every bot comment. Change to:

```go
blockType, content, err := message.Parse(c.Body)
if err != nil || blockType != "match" {
    continue
}
// content is the signed blob — verify + decrypt
```

## Files Modified

| File | Change |
|------|--------|
| Create: `internal/message/message.go` | Format + Parse functions |
| Create: `internal/message/message_test.go` | Unit tests |
| Modify: `cmd/regcrypt/main.go` | Add `register` subcommand |
| Modify: `cmd/matchcrypt/main.go` | Add `match` subcommand |
| Modify: `templates/actions/pool-register.yml` | Thin wrapper |
| Modify: `templates/actions/pool-interest.yml` | Thin wrapper |
| Modify: `internal/github/pool.go` | Use message.Parse for polling |
| Modify: `internal/cli/pool.go` | Use message.Format for issue body |
| Modify: `internal/cli/tui/screens/matches.go` | Use message.Parse for notifications |
| Modify: `internal/cli/matches.go` | Use message.Parse for notifications |

## Testing

- Unit tests for `message.Format` / `message.Parse` (roundtrip, invalid format, missing marker)
- Unit tests for `regcrypt register` (valid input, missing fields, invalid pubkey)
- Unit tests for `matchcrypt match` (mutual found, no reciprocal, concurrent handling)
- Integration: full flow with real-format issue body → regcrypt register → output file → CLI parse
- E2E: deploy updated Actions to test pool, register user, verify new comment format
