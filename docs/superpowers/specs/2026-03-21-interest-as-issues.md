# Interest as Issues

## Goal

Replace interest PRs with interest Issues. Simpler flow — no branch management, same encrypted body, same matching logic. Introduce `ListIssues` on GitHubClient interface and `FindOperatorReplyInIssue` as a shared comment scanning helper.

## Why

Interest PRs create branches, require merge/close semantics, and are heavier than needed. We never merge interest PRs — they're just closed when a match is detected. Issues are the right primitive: create, close, comment.

## Design

### Like Command

```
Current:  CreateInterestPR(title=targetMatchHash, label=interest, body=encrypted, branch=...)
New:      CreateIssue(title=targetMatchHash, labels=[interest], body=message.Format("interest", encrypted))
```

No branch creation. Just `CreateIssue`.

### Interest Issue Format

```
Title: {target_match_hash}
Labels: [interest]
Body:
  <!-- openpool:interest -->
  ```
  {base64_encrypted_body}
  ```
```

Body encrypted to operator's pubkey. Contains `{author_bin_hash, author_match_hash, greeting}`.

### Action Trigger

```yaml
name: Process Interest

on:
  issues:
    types: [opened]

jobs:
  match:
    if: contains(github.event.issue.labels.*.name, 'interest')
    runs-on: ubuntu-latest
    permissions:
      contents: write
      issues: write
    steps:
      - uses: actions/checkout@v4

      - name: Download matchcrypt
        run: |
          curl -sL https://github.com/vutran1710/regcrypt/releases/latest/download/matchcrypt-linux-amd64 -o matchcrypt
          chmod +x matchcrypt

      - name: Match
        env:
          ISSUE_BODY: ${{ github.event.issue.body }}
          ISSUE_NUMBER: ${{ github.event.issue.number }}
          ISSUE_TITLE: ${{ github.event.issue.title }}
          REPO: ${{ github.repository }}
          OPERATOR_PRIVATE_KEY: ${{ secrets.OPERATOR_PRIVATE_KEY }}
          POOL_SALT: ${{ secrets.POOL_SALT }}
          GH_TOKEN: ${{ github.token }}
        run: ./matchcrypt match
```

One line. `matchcrypt match` uses `CLIClient.ListIssues` internally to find reciprocal.

### matchcrypt match (updated)

```
Inputs (env vars):
  ISSUE_BODY           current issue body (contains encrypted interest)
  ISSUE_NUMBER         current issue number
  ISSUE_TITLE          current issue title (target match_hash)
  REPO                 "owner/repo"
  OPERATOR_PRIVATE_KEY operator key hex
  POOL_SALT            pool salt
  GH_TOKEN             GitHub token (for CLIClient)

Logic:
  1. Parse ISSUE_BODY using message.Parse("interest") → extract encrypted blob
  2. Decrypt with operator key → {author_bin_hash, author_match_hash, greeting}
  3. Use CLIClient.ListIssues(ctx, "open", "interest") → find issue with title == author_match_hash
  4. If not found → exit 0 (no reciprocal)
  5. If found:
     a. Parse + decrypt reciprocal issue body
     b. Read pubkeys from users/{bin_hash}.bin
     c. Compute pair_hash, write matches/{pair_hash}.json
     d. Encrypt + sign notifications
     e. git add/commit/push match file
     f. Close both issues
     g. Post signed match comments on both issues
```

### GitHubClient Interface Addition

```go
// Add to GitHubClient interface in internal/github/github.go
ListIssues(ctx context.Context, state string, labels ...string) ([]Issue, error)
```

Issue type extended:

```go
type Issue struct {
    Number      int      `json:"number"`
    Title       string   `json:"title"`
    Body        string   `json:"body"`
    State       string   `json:"state"`
    StateReason string   `json:"state_reason"`
    Labels      []Label  `json:"labels"`
    User        User     `json:"user"`
}
```

Both CLIClient and HTTPClient implement `ListIssues`.

### FindOperatorReplyInIssue

Shared helper for scanning issue comments. Single responsibility — finds and returns verified content, does NOT decrypt.

```go
// internal/github/comments.go

// FindOperatorReplyInIssue scans comments newest-first, finds the first one
// matching the block type with a valid operator signature.
// Returns the raw signed content — caller handles decryption.
func FindOperatorReplyInIssue(
    ctx context.Context,
    client GitHubClient,
    issueNumber int,
    blockType string,
    operatorPub ed25519.PublicKey,
) (content string, err error) {
    comments, err := client.ListIssueComments(ctx, issueNumber)
    if err != nil {
        return "", err
    }
    for i := len(comments) - 1; i >= 0; i-- {
        bt, c, parseErr := message.Parse(comments[i].Body)
        if parseErr != nil || bt != blockType {
            continue
        }
        if verifySignature(c, operatorPub) {
            return c, nil
        }
    }
    return "", fmt.Errorf("no valid %s reply found", blockType)
}
```

### DecryptSignedBlob

Generic decryption — signature already verified by `FindOperatorReplyInIssue`. Returns raw plaintext, caller unmarshals.

```go
// internal/crypto/decrypt.go (or add to existing encrypt.go)

// DecryptSignedBlob decrypts a "base64.hexsig" blob. Signature assumed already verified.
func DecryptSignedBlob(signedBlob string, priv ed25519.PrivateKey) ([]byte, error) {
    parts := strings.SplitN(signedBlob, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid signed blob format")
    }
    ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, err
    }
    return Decrypt(priv, ciphertext)
}
```

Used by:
```go
// Registration
signedBlob, _ := FindOperatorReplyInIssue(ctx, client, issue, "registration", opPub)
plaintext, _ := crypto.DecryptSignedBlob(signedBlob, userPriv)
var hashes map[string]string
msgpack.Unmarshal(plaintext, &hashes)

// Match
signedBlob, _ := FindOperatorReplyInIssue(ctx, client, issue, "match", opPub)
plaintext, _ := crypto.DecryptSignedBlob(signedBlob, userPriv)
var match struct { MatchedMatchHash, Greeting, Pubkey string }
json.Unmarshal(plaintext, &match)
```

Replaces:
- `tryDecryptComment` + reverse iteration in `PollRegistrationResult`
- Comment scanning in `LoadMatchesCmd`
- Comment scanning in `decryptMatchComment`

### CLI Changes

| Component | Current | New |
|-----------|---------|-----|
| `like` command | `CreateInterestPR` | `CreateIssue(title, body, ["interest"])` |
| `matches` command | `ListPullRequests("closed")` filter interest | `ListIssues("closed", "interest")` |
| `inbox` command | `ListPullRequests("open")` filter interest | `ListIssues("open", "interest")` |
| TUI matches screen | `ListPullRequests("closed")` | `ListIssues("closed", "interest")` |
| Registration polling | custom loop + `tryDecryptComment` | `FindOperatorReplyInIssue` + decrypt |
| Match polling | custom loop + `decryptMatchNotification` | `FindOperatorReplyInIssue` + decrypt |

### What Doesn't Change

- Encrypted body format (NaCl box)
- Operator signature on match comments
- Match file creation (`matches/{pair_hash}.json`)
- Relay chat
- Registration flow (already uses issues)

## Files Modified

| File | Change |
|------|--------|
| Create: `internal/github/comments.go` | `FindOperatorReplyInIssue` helper |
| Modify: `internal/github/github.go` | Add `ListIssues` to interface, extend `Issue` type |
| Modify: `internal/github/cli.go` | Implement `ListIssues` |
| Modify: `internal/github/client.go` | Implement `ListIssues` on HTTPClient |
| Modify: `internal/github/pool.go` | `CreateInterestIssue` replaces `CreateInterestPR`, use `FindOperatorReplyInIssue` in polling |
| Modify: `internal/cli/like.go` | Use `CreateInterestIssue` |
| Modify: `internal/cli/matches.go` | Use `ListIssues` + `FindOperatorReplyInIssue` |
| Modify: `internal/cli/tui/screens/matches.go` | Use `ListIssues` + `FindOperatorReplyInIssue` |
| Modify: `cmd/matchcrypt/main.go` | `cmdMatch` uses `ListIssues` via CLIClient instead of `RECIP_PRS` env |
| Modify: `templates/actions/pool-interest.yml` | Trigger on `issues:opened`, one-line run |

## Testing

- Unit tests for `FindOperatorReplyInIssue` (valid, no match, forged signature)
- Unit tests for `ListIssues` on both CLIClient and HTTPClient
- Update existing `like` command tests
- Update existing `matches` command tests
- E2E: create interest issues, verify match detection
