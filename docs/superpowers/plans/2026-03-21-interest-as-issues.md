# Interest as Issues Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace interest PRs with interest Issues. Introduce `ListIssues` on GitHubClient, `FindOperatorReplyInIssue` for shared comment scanning, `DecryptSignedBlob` for generic decryption, and `SubmitIssue` for unified issue creation.

**Architecture:** Interest flow moves from PRs to Issues. Both registration and interest use the same `SubmitIssue` function. Comment scanning uses shared `FindOperatorReplyInIssue` (single responsibility — find + verify, no decrypt). `matchcrypt match` uses CLIClient internally to list issues.

**Tech Stack:** Go, GitHubClient interface, message package, ed25519

---

## File Structure

### New Files
- `internal/github/comments.go` — `FindOperatorReplyInIssue` helper
- `internal/github/comments_test.go` — tests
- `internal/crypto/blob.go` — `DecryptSignedBlob` helper

### Modified Files
- `internal/github/github.go` — Add `ListIssues` to interface, extend `Issue` type
- `internal/github/cli.go` — Implement `ListIssues`
- `internal/github/client.go` — Implement `ListIssues` on HTTPClient
- `internal/github/pool.go` — `SubmitIssue` replaces `SubmitRegistration` + `CreateInterestPR`, `ListInterestsForMe` uses issues
- `internal/cli/like.go` — Use `SubmitIssue` instead of `CreateInterestPR`
- `internal/cli/matches.go` — Use `ListIssues` + `FindOperatorReplyInIssue` + `DecryptSignedBlob`
- `internal/cli/tui/screens/matches.go` — Same
- `cmd/matchcrypt/main.go` — `cmdMatch` uses `ListIssues` via CLIClient
- `templates/actions/pool-interest.yml` — Trigger on `issues:opened`, one-line run

### Deleted Code
- `CreateInterestPR` in pool.go
- `CreateLikeIssue`, `CreateLikePR`, `AcceptLike`, `RejectLike` (legacy)
- Old PR-based interest scanning

---

### Task 1: Extend Issue type + add ListIssues to interface

**Files:**
- Modify: `internal/github/github.go`
- Modify: `internal/github/types.go`

- [ ] **Step 1: Extend Issue type in types.go**

Currently `Issue` only has `Number`, `State`, `StateReason`. Add fields needed for interest scanning:

```go
type Issue struct {
    Number      int      `json:"number"`
    Title       string   `json:"title"`
    Body        string   `json:"body"`
    State       string   `json:"state"`
    StateReason string   `json:"state_reason"`
    Labels      []Label  `json:"labels"`
    User        User     `json:"user"`
    CreatedAt   time.Time `json:"created_at"`
}

type User struct {
    Login string `json:"login"`
    ID    int    `json:"id"`
}
```

Check if `User` type already exists — if not, add it.

- [ ] **Step 2: Add ListIssues to GitHubClient interface**

```go
// In github.go, add to GitHubClient interface:
ListIssues(ctx context.Context, state string, labels ...string) ([]Issue, error)
```

- [ ] **Step 3: Commit**

```bash
git add internal/github/github.go internal/github/types.go
git commit -m "feat: extend Issue type, add ListIssues to GitHubClient interface"
```

---

### Task 2: Implement ListIssues on both clients

**Files:**
- Modify: `internal/github/cli.go`
- Modify: `internal/github/client.go`

- [ ] **Step 1: Implement on CLIClient**

```go
func (c *CLIClient) ListIssues(ctx context.Context, state string, labels ...string) ([]Issue, error) {
    args := []string{"issue", "list", "--repo", c.repo, "--state", state,
        "--json", "number,title,body,state,stateReason,labels,user,createdAt", "--limit", "100"}
    for _, l := range labels {
        args = append(args, "--label", l)
    }
    out, err := c.gh(args...)
    if err != nil {
        return nil, err
    }
    var issues []Issue
    if err := json.Unmarshal(out, &issues); err != nil {
        return nil, err
    }
    return issues, nil
}
```

- [ ] **Step 2: Implement on HTTPClient**

```go
func (c *HTTPClient) ListIssues(ctx context.Context, state string, labels ...string) ([]Issue, error) {
    query := fmt.Sprintf("/issues?state=%s&per_page=100", state)
    if len(labels) > 0 {
        query += "&labels=" + strings.Join(labels, ",")
    }
    resp, err := c.do(ctx, "GET", c.apiURL(query), nil)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var issues []Issue
    if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
        return nil, err
    }
    return issues, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/github/`

- [ ] **Step 4: Commit**

```bash
git add internal/github/cli.go internal/github/client.go
git commit -m "feat: implement ListIssues on CLIClient and HTTPClient"
```

---

### Task 3: Create DecryptSignedBlob

**Files:**
- Create: `internal/crypto/blob.go`

- [ ] **Step 1: Write tests**

```go
// internal/crypto/blob_test.go
func TestDecryptSignedBlob_Valid(t *testing.T) {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    plaintext := []byte("secret data")
    encrypted, _ := Encrypt(pub, plaintext)
    sig := ed25519.Sign(priv, encrypted) // priv used as "operator" here
    blob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

    result, err := DecryptSignedBlob(blob, priv)
    if err != nil {
        t.Fatal(err)
    }
    if string(result) != "secret data" {
        t.Fatalf("got %q", result)
    }
}

func TestDecryptSignedBlob_InvalidFormat(t *testing.T) {
    _, priv, _ := ed25519.GenerateKey(rand.Reader)
    _, err := DecryptSignedBlob("no-dot-separator", priv)
    if err == nil {
        t.Fatal("expected error")
    }
}
```

- [ ] **Step 2: Implement**

```go
// internal/crypto/blob.go
package crypto

import (
    "crypto/ed25519"
    "encoding/base64"
    "fmt"
    "strings"
)

// DecryptSignedBlob decrypts a "base64.hexsig" blob.
// Signature is assumed already verified by the caller.
// Returns raw plaintext bytes — caller unmarshals.
func DecryptSignedBlob(signedBlob string, priv ed25519.PrivateKey) ([]byte, error) {
    parts := strings.SplitN(signedBlob, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid signed blob format: no separator")
    }
    ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, fmt.Errorf("invalid base64: %w", err)
    }
    return Decrypt(priv, ciphertext)
}
```

- [ ] **Step 3: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v -run TestDecryptSignedBlob ./internal/crypto/`

- [ ] **Step 4: Commit**

```bash
git add internal/crypto/blob.go internal/crypto/blob_test.go
git commit -m "feat: DecryptSignedBlob — generic signed blob decryption"
```

---

### Task 4: Create FindOperatorReplyInIssue

**Files:**
- Create: `internal/github/comments.go`
- Create: `internal/github/comments_test.go`

- [ ] **Step 1: Implement**

```go
// internal/github/comments.go
package github

import (
    "context"
    "crypto/ed25519"
    "encoding/hex"
    "fmt"
    "strings"

    "github.com/vutran1710/dating-dev/internal/message"
)

// FindOperatorReplyInIssue scans issue comments newest-first for the first one
// matching the block type with a valid operator signature.
// Returns the raw signed content (base64.hexsig) — caller handles decryption.
func FindOperatorReplyInIssue(
    ctx context.Context,
    client GitHubClient,
    issueNumber int,
    blockType string,
    operatorPub ed25519.PublicKey,
) (string, error) {
    comments, err := client.ListIssueComments(ctx, issueNumber)
    if err != nil {
        return "", fmt.Errorf("listing comments: %w", err)
    }
    for i := len(comments) - 1; i >= 0; i-- {
        bt, content, parseErr := message.Parse(comments[i].Body)
        if parseErr != nil || bt != blockType {
            continue
        }
        if verifyBlobSignature(content, operatorPub) {
            return content, nil
        }
    }
    return "", fmt.Errorf("no valid %s reply found", blockType)
}

// verifyBlobSignature checks if a "base64.hexsig" content has a valid operator signature.
func verifyBlobSignature(content string, operatorPub ed25519.PublicKey) bool {
    parts := strings.SplitN(content, ".", 2)
    if len(parts) != 2 {
        return false
    }
    ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return false
    }
    sigBytes, err := hex.DecodeString(parts[1])
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return false
    }
    return ed25519.Verify(operatorPub, ciphertext, sigBytes)
}
```

Add `"encoding/base64"` to imports.

- [ ] **Step 2: Write tests using a mock client**

Test with a mock that returns predefined comments. Test cases:
- Valid signed comment found (newest first)
- No matching block type → error
- Forged signature → skipped
- Empty comments → error

- [ ] **Step 3: Commit**

```bash
git add internal/github/comments.go internal/github/comments_test.go
git commit -m "feat: FindOperatorReplyInIssue — shared comment scanning helper"
```

---

### Task 5: Create SubmitIssue + refactor registration

**Files:**
- Modify: `internal/github/pool.go`

- [ ] **Step 1: Add generic SubmitIssue**

```go
// SubmitIssue creates a GitHub issue with structured message body.
// Used for both registration requests and interest expressions.
func (p *Pool) SubmitIssue(ctx context.Context, title, blockType, content string, labels []string) (int, error) {
    body := message.Format(blockType, content)
    return p.client.CreateIssue(ctx, title, body, labels)
}
```

- [ ] **Step 2: Refactor RegisterUserViaIssue to use SubmitIssue**

```go
func (p *Pool) RegisterUserViaIssue(ctx context.Context, userHash string, encryptedBlob []byte, pubKeyHex, signature, identityProof string) (int, error) {
    blobHex := hex.EncodeToString(encryptedBlob)
    content := strings.Join([]string{userHash, pubKeyHex, blobHex, signature, identityProof}, "\n")
    return p.SubmitIssue(ctx, "Registration Request", "registration-request", content, []string{"registration"})
}
```

- [ ] **Step 3: Add CreateInterestIssue using SubmitIssue**

```go
func (p *Pool) CreateInterestIssue(ctx context.Context, myBinHash, myMatchHash, targetMatchHash, greeting string, operatorPubKey ed25519.PublicKey) (int, error) {
    payload, _ := json.Marshal(map[string]string{
        "author_bin_hash":   myBinHash,
        "author_match_hash": myMatchHash,
        "greeting":          greeting,
    })
    encrypted, err := crypto.Encrypt(operatorPubKey, payload)
    if err != nil {
        return 0, fmt.Errorf("encrypting interest: %w", err)
    }
    body := base64.StdEncoding.EncodeToString(encrypted)
    return p.SubmitIssue(ctx, targetMatchHash, "interest", body, []string{"interest"})
}
```

- [ ] **Step 4: Add ListInterestsForMeIssues**

```go
func (p *Pool) ListInterestsForMeIssues(ctx context.Context, myMatchHash string) ([]Issue, error) {
    issues, err := p.client.ListIssues(ctx, "open", "interest")
    if err != nil {
        return nil, err
    }
    var results []Issue
    for _, iss := range issues {
        if iss.Title == myMatchHash {
            results = append(results, iss)
        }
    }
    return results, nil
}
```

- [ ] **Step 5: Refactor PollRegistrationResult to use FindOperatorReplyInIssue**

Replace the manual comment iteration loop with:

```go
func (p *Pool) PollRegistrationResult(ctx context.Context, issueNumber int, operatorPub ed25519.PublicKey, userPriv ed25519.PrivateKey) (binHash, matchHash string, err error) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        issue, err := p.client.GetIssue(ctx, issueNumber)
        if err == nil && issue.State == "closed" && issue.StateReason == "not_planned" {
            return "", "", fmt.Errorf("registration rejected")
        }

        content, findErr := FindOperatorReplyInIssue(ctx, p.client, issueNumber, "registration", operatorPub)
        if findErr == nil {
            plaintext, decErr := crypto.DecryptSignedBlob(content, userPriv)
            if decErr == nil {
                var hashes map[string]string
                if err := msgpack.Unmarshal(plaintext, &hashes); err == nil {
                    return hashes["bin_hash"], hashes["match_hash"], nil
                }
            }
        }

        if issue != nil && issue.State == "closed" && issue.StateReason != "not_planned" {
            return "", "", fmt.Errorf("issue closed but no valid comment found")
        }

        select {
        case <-ctx.Done():
            return "", "", fmt.Errorf("registration timed out: %w", ctx.Err())
        case <-ticker.C:
        }
    }
}
```

- [ ] **Step 6: Delete old functions**

Remove: `CreateInterestPR`, `CreateLikeIssue`, `CreateLikePR`, `AcceptLike`, `RejectLike`, `tryDecryptComment`, `ListInterestsForMe` (PR version).

- [ ] **Step 7: Verify it compiles + run tests**

Run: `go build ./internal/github/ && DATING_HOME=$(mktemp -d) go test ./internal/github/ -count=1`

- [ ] **Step 8: Commit**

```bash
git add internal/github/pool.go
git commit -m "refactor: SubmitIssue + CreateInterestIssue replace PR-based interest flow"
```

---

### Task 6: Update CLI callers

**Files:**
- Modify: `internal/cli/like.go`
- Modify: `internal/cli/matches.go`
- Modify: `internal/cli/tui/screens/matches.go`
- Modify: `internal/cli/tui/screens/inbox.go` (if it references PRs)
- Modify: `internal/cli/tui/app.go` (sendLike, acceptLike, rejectLike)

- [ ] **Step 1: Update like command**

In `like.go`, replace `CreateInterestPR` with `CreateInterestIssue`:

```go
issueNumber, err := client.CreateInterestIssue(
    ctx,
    pool.BinHash,
    pool.MatchHash,
    targetMatchHash,
    greeting,
    ed25519.PublicKey(operatorPubBytes),
)
// ...
printSuccess(fmt.Sprintf("Interest sent! (Issue #%d)", issueNumber))
```

- [ ] **Step 2: Update matches command (CLI)**

In `matches.go`, replace `ListPullRequests("closed")` with `ListIssues("closed", "interest")`:

```go
issues, err := client.Client().ListIssues(ctx, "closed", "interest")
// For each issue, use FindOperatorReplyInIssue + DecryptSignedBlob
```

- [ ] **Step 3: Update matches screen (TUI)**

In `tui/screens/matches.go`, same change — `ListIssues` instead of `ListPullRequests`.

- [ ] **Step 4: Update inbox command + TUI inbox**

Replace `ListInterestsForMe` (PR) with `ListInterestsForMeIssues` (Issue).

- [ ] **Step 5: Update sendLike in app.go**

Replace `CreateInterestPR` call with `CreateInterestIssue`.

- [ ] **Step 6: Remove acceptLike/rejectLike from app.go**

These used `MergePullRequest`/`ClosePullRequest` — no longer applicable with issues. If the inbox still needs accept/reject, convert to `CloseIssue`.

- [ ] **Step 7: Verify full build + tests**

Run: `go build ./... && DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 8: Commit**

```bash
git add internal/cli/
git commit -m "refactor: CLI uses issues for interests, FindOperatorReplyInIssue for scanning"
```

---

### Task 7: Update matchcrypt match to use ListIssues

**Files:**
- Modify: `cmd/matchcrypt/main.go`

- [ ] **Step 1: Update cmdMatch**

Remove `RECIP_PRS` env var dependency. Instead, use CLIClient internally:

```go
func cmdMatch() {
    issueBody := os.Getenv("ISSUE_BODY")
    issueNumber := os.Getenv("ISSUE_NUMBER")
    repo := os.Getenv("REPO")
    operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
    // ...

    // Parse issue body (was PR body)
    blockType, encryptedBody, err := message.Parse(issueBody)
    if err != nil || blockType != "interest" {
        writeError("invalid issue body")
    }

    // Decrypt
    bodyBytes, _ := base64.StdEncoding.DecodeString(encryptedBody)
    authorPlaintext, _ := crypto.Decrypt(ed25519.PrivateKey(operatorKey), bodyBytes)
    // ... unmarshal authorInfo

    // Search for reciprocal using CLIClient
    gh, _ := github.NewCLI(repo)
    issues, _ := gh.ListIssues(context.Background(), "open", "interest")
    var recipIssue *github.Issue
    for i, iss := range issues {
        if iss.Title == authorInfo.AuthorMatchHash {
            recipIssue = &issues[i]
            break
        }
    }
    if recipIssue == nil {
        log.Println("no reciprocal interest found")
        return
    }

    // Parse + decrypt reciprocal issue body
    _, recipEncrypted, _ := message.Parse(recipIssue.Body)
    // ... rest of match logic (same as before)

    // Close issues instead of PRs
    issNum, _ := strconv.Atoi(issueNumber)
    gh.CloseIssue(ctx, issNum, "completed")
    gh.CloseIssue(ctx, recipIssue.Number, "completed")
    gh.CommentIssue(ctx, issNum, message.Format("match", authorNotif))
    gh.CommentIssue(ctx, recipIssue.Number, message.Format("match", recipNotif))
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/matchcrypt/`

- [ ] **Step 3: Commit**

```bash
git add cmd/matchcrypt/main.go
git commit -m "refactor: matchcrypt match uses ListIssues via CLIClient"
```

---

### Task 8: Update Action template

**Files:**
- Modify: `templates/actions/pool-interest.yml`

- [ ] **Step 1: Replace with issue-triggered template**

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

- [ ] **Step 2: Commit**

```bash
git add templates/actions/pool-interest.yml
git commit -m "refactor: interest Action triggers on issues, not PRs"
```

---

### Task 9: Full test suite + cleanup

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 2: Clean up dead code**

Search for any remaining PR references in the interest flow:
```bash
grep -rn "InterestPR\|ListInterestsForMe\b\|CreateLikePR\|AcceptLike\|RejectLike" --include='*.go'
```

Remove anything found.

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "clean: remove dead PR-based interest code"
```
