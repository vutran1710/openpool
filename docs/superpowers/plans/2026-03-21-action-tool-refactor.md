# Action Tool Refactor Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move bash logic from Action templates into Go tools. Introduce `message` package for structured comment formatting. Action templates become thin wrappers.

**Architecture:** New `internal/message` package with `Format`/`Parse`. `regcrypt register` and `matchcrypt match` subcommands do all computation + file I/O. CLI uses `message.Format` for issue bodies and `message.Parse` for polling. Action YAML becomes download + run + git + gh.

**Tech Stack:** Go, ed25519, NaCl box, msgpack, GitHub Actions

---

## File Structure

### New Files
- `internal/message/message.go` — Format/Parse for structured message blocks
- `internal/message/message_test.go` — Unit tests

### Modified Files
- `cmd/regcrypt/main.go` — Add `register` subcommand
- `cmd/matchcrypt/main.go` — Add `match` subcommand
- `templates/actions/pool-register.yml` — Thin wrapper
- `templates/actions/pool-interest.yml` — Thin wrapper
- `internal/github/pool.go` — Use message.Parse in tryDecryptComment + SubmitRegistration
- `internal/cli/tui/screens/matches.go` — Use message.Parse in decryptMatchNotification
- `internal/cli/matches.go` — Use message.Parse in decryptMatchComment

---

### Task 1: Create message package

**Files:**
- Create: `internal/message/message.go`
- Create: `internal/message/message_test.go`

- [ ] **Step 1: Write tests**

```go
package message

import "testing"

func TestFormat_Roundtrip(t *testing.T) {
    msg := Format("registration", "some-content-here")
    blockType, content, err := Parse(msg)
    if err != nil {
        t.Fatal(err)
    }
    if blockType != "registration" {
        t.Fatalf("blockType = %q, want registration", blockType)
    }
    if content != "some-content-here" {
        t.Fatalf("content = %q", content)
    }
}

func TestFormat_MultilineContent(t *testing.T) {
    msg := Format("registration-request", "pubkey123\nblobhex456")
    blockType, content, err := Parse(msg)
    if err != nil {
        t.Fatal(err)
    }
    if blockType != "registration-request" {
        t.Fatalf("blockType = %q", blockType)
    }
    if content != "pubkey123\nblobhex456" {
        t.Fatalf("content = %q", content)
    }
}

func TestParse_InvalidFormat(t *testing.T) {
    _, _, err := Parse("just some random text")
    if err == nil {
        t.Fatal("expected error for invalid format")
    }
}

func TestParse_NoFencedBlock(t *testing.T) {
    _, _, err := Parse("<!-- openpool:registration -->")
    if err == nil {
        t.Fatal("expected error for missing fenced block")
    }
}

func TestParse_EmptyContent(t *testing.T) {
    msg := Format("error", "")
    blockType, content, err := Parse(msg)
    if err != nil {
        t.Fatal(err)
    }
    if blockType != "error" {
        t.Fatalf("blockType = %q", blockType)
    }
    if content != "" {
        t.Fatalf("content should be empty, got %q", content)
    }
}

func TestFormat_OutputShape(t *testing.T) {
    msg := Format("match", "blob.sig")
    expected := "<!-- openpool:match -->\n```\nblob.sig\n```"
    if msg != expected {
        t.Fatalf("got:\n%s\nwant:\n%s", msg, expected)
    }
}

func TestParse_WithSurroundingText(t *testing.T) {
    // Comment body might have other text around the block
    raw := "some noise\n<!-- openpool:registration -->\n```\ncontent\n```\nmore noise"
    blockType, content, err := Parse(raw)
    if err != nil {
        t.Fatal(err)
    }
    if blockType != "registration" || content != "content" {
        t.Fatalf("got type=%q content=%q", blockType, content)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/message/`
Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Implement message package**

```go
package message

import (
    "fmt"
    "regexp"
    "strings"
)

var markerRe = regexp.MustCompile(`<!-- openpool:([a-z-]+) -->`)

// Format creates a structured message block:
//
//   <!-- openpool:{blockType} -->
//   ```
//   {content}
//   ```
func Format(blockType, content string) string {
    return fmt.Sprintf("<!-- openpool:%s -->\n```\n%s\n```", blockType, content)
}

// Parse extracts block type and content from a structured message.
// The message may contain surrounding text — only the first openpool block is extracted.
func Parse(raw string) (blockType, content string, err error) {
    match := markerRe.FindStringSubmatchIndex(raw)
    if match == nil {
        return "", "", fmt.Errorf("no openpool marker found")
    }

    blockType = raw[match[2]:match[3]]

    // Find the fenced block after the marker
    rest := raw[match[1]:]
    fenceStart := strings.Index(rest, "```\n")
    if fenceStart < 0 {
        return "", "", fmt.Errorf("no fenced block after marker")
    }
    afterFence := rest[fenceStart+4:]
    fenceEnd := strings.Index(afterFence, "\n```")
    if fenceEnd < 0 {
        // Try without trailing newline (end of string)
        fenceEnd = strings.Index(afterFence, "```")
        if fenceEnd < 0 {
            return "", "", fmt.Errorf("unclosed fenced block")
        }
    }

    content = afterFence[:fenceEnd]
    return blockType, content, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test -v ./internal/message/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/message/
git commit -m "feat: message package — Format/Parse for structured comment blocks"
```

---

### Task 2: Add `regcrypt register` subcommand

**Files:**
- Modify: `cmd/regcrypt/main.go`

- [ ] **Step 1: Add register subcommand**

Add `register` case alongside the existing `sign` check at the top of `main()`:

```go
func main() {
    if len(os.Args) >= 2 {
        switch os.Args[1] {
        case "sign":
            cmdSign()
            return
        case "register":
            cmdRegister()
            return
        }
    }
    // ... existing flag-based hash logic
}
```

Implement `cmdRegister()`:

```go
func cmdRegister() {
    // Read env vars
    issueBody := os.Getenv("ISSUE_BODY")
    issueAuthorID := os.Getenv("ISSUE_AUTHOR_ID")
    issueAuthor := os.Getenv("ISSUE_AUTHOR")
    repo := os.Getenv("REPO")
    salt := os.Getenv("POOL_SALT")
    operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")

    if issueBody == "" || repo == "" || salt == "" || operatorKeyHex == "" {
        writeError("missing required env vars")
        return
    }

    operatorKey, err := hex.DecodeString(operatorKeyHex)
    if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
        writeError("invalid operator key")
        return
    }

    // Parse issue body using message.Parse
    blockType, content, err := message.Parse(issueBody)
    if err != nil || blockType != "registration-request" {
        writeError("invalid issue body format")
        return
    }

    // Content is: line 1 = user_hash, line 2 = pubkey, line 3 = blob_hex, line 4 = signature, line 5 = identity_proof
    lines := strings.Split(content, "\n")
    if len(lines) < 3 {
        writeError("missing fields in registration request")
        return
    }

    pubKeyHex := strings.TrimSpace(lines[1])
    blobHex := strings.TrimSpace(lines[2])

    if pubKeyHex == "" || blobHex == "" {
        writeError("missing pubkey or profile blob")
        return
    }

    // Decode user pubkey
    userPub, err := hex.DecodeString(pubKeyHex)
    if err != nil || len(userPub) != ed25519.PublicKeySize {
        writeError("invalid user pubkey")
        return
    }

    // Determine provider + user_id
    provider := "github"
    userID := issueAuthorID
    // Check for custom ID (operator registration)
    if len(lines) >= 5 {
        customID := strings.TrimSpace(lines[4])
        repoOwner := strings.Split(repo, "/")[0]
        if issueAuthor == repoOwner && strings.Contains(customID, ":") {
            parts := strings.SplitN(customID, ":", 2)
            provider = parts[0]
            userID = parts[1]
        }
    }

    // Compute hashes
    idH := crypto.UserHash(repo, provider, userID).String()
    binH := sha256Short(salt + ":" + idH)
    matchH := sha256Short(salt + ":" + binH)

    // Encrypt {bin_hash, match_hash} to user's pubkey
    payload, err := msgpack.Marshal(map[string]string{
        "bin_hash":   binH,
        "match_hash": matchH,
    })
    if err != nil {
        writeError("marshal: " + err.Error())
        return
    }

    encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
    if err != nil {
        writeError("encrypt: " + err.Error())
        return
    }

    // Sign with operator key
    sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), encrypted)
    signedBlob := base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)

    // Write .bin file
    blobBytes, err := hex.DecodeString(blobHex)
    if err != nil {
        writeError("invalid blob hex: " + err.Error())
        return
    }
    os.MkdirAll("users", 0755)
    if err := os.WriteFile("users/"+binH+".bin", blobBytes, 0644); err != nil {
        writeError("writing .bin: " + err.Error())
        return
    }

    // Git add/commit/push using GitHub CLI client
    issueNumber, _ := strconv.Atoi(os.Getenv("ISSUE_NUMBER"))
    gh, err := github.NewCLI(repo)
    if err != nil {
        writeError("github CLI: " + err.Error())
        return
    }
    if err := gh.AddCommitPush([]string{"users/"}, "Register user "+binH); err != nil {
        writeError("git push: " + err.Error())
        return
    }

    // Close issue, then post signed comment
    comment := message.Format("registration", signedBlob)
    ctx := context.Background()
    gh.CloseIssue(ctx, issueNumber, "completed")
    gh.CommentIssue(ctx, issueNumber, comment)

    log.Printf("registered: bin=%s match=%s", binH, matchH)
}

func writeError(msg string) {
    fmt.Fprintf(os.Stderr, "error: %s\n", msg)
    os.Exit(1)
}
```

Add imports: `"context"`, `"strconv"`, `"strings"`, `"github.com/vutran1710/dating-dev/internal/github"`, `"github.com/vutran1710/dating-dev/internal/message"`

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/regcrypt/`

- [ ] **Step 3: Commit**

```bash
git add cmd/regcrypt/main.go
git commit -m "feat: regcrypt register — full registration with git+gh via CLIClient"
```

---

### Task 3: Add `matchcrypt match` subcommand

**Files:**
- Modify: `cmd/matchcrypt/main.go`

- [ ] **Step 1: Add match case to switch**

```go
case "match":
    cmdMatch()
```

Implement `cmdMatch()`:

```go
func cmdMatch() {
    prBody := os.Getenv("PR_BODY")
    prNumber := os.Getenv("PR_NUMBER")
    recipPRsJSON := os.Getenv("RECIP_PRS")
    repo := os.Getenv("REPO")
    operatorKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
    poolSalt := os.Getenv("POOL_SALT")

    if prBody == "" || prNumber == "" || operatorKeyHex == "" {
        writeError("missing required env vars")
        return
    }

    operatorKey, err := hex.DecodeString(operatorKeyHex)
    if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
        writeError("invalid operator key")
        return
    }

    // Decrypt current PR body
    prBodyBytes, err := base64.StdEncoding.DecodeString(prBody)
    if err != nil {
        writeError("invalid PR body base64")
        return
    }
    authorPlaintext, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), prBodyBytes)
    if err != nil {
        writeError("decrypt PR body: " + err.Error())
        return
    }

    var authorInfo struct {
        AuthorBinHash   string `json:"author_bin_hash"`
        AuthorMatchHash string `json:"author_match_hash"`
        Greeting        string `json:"greeting"`
    }
    if err := json.Unmarshal(authorPlaintext, &authorInfo); err != nil {
        writeError("parse PR body: " + err.Error())
        return
    }

    // Search for reciprocal PR
    var recipPRs []struct {
        Number int    `json:"number"`
        Title  string `json:"title"`
        Body   string `json:"body"`
    }
    if err := json.Unmarshal([]byte(recipPRsJSON), &recipPRs); err != nil {
        log.Printf("no reciprocal PRs data")
        return // no match, exit silently
    }

    var recipPR *struct {
        Number int
        Title  string
        Body   string
    }
    for i, pr := range recipPRs {
        if pr.Title == authorInfo.AuthorMatchHash {
            recipPR = &recipPRs[i]
            break
        }
    }

    if recipPR == nil {
        log.Printf("no reciprocal interest found")
        return // no match, no output files
    }

    log.Printf("mutual interest detected! reciprocal PR #%d", recipPR.Number)

    // Decrypt reciprocal PR body
    recipBodyBytes, err := base64.StdEncoding.DecodeString(recipPR.Body)
    if err != nil {
        writeError("invalid reciprocal PR body")
        return
    }
    recipPlaintext, err := crypto.Decrypt(ed25519.PrivateKey(operatorKey), recipBodyBytes)
    if err != nil {
        writeError("decrypt reciprocal PR: " + err.Error())
        return
    }

    var recipInfo struct {
        AuthorBinHash   string `json:"author_bin_hash"`
        AuthorMatchHash string `json:"author_match_hash"`
        Greeting        string `json:"greeting"`
    }
    if err := json.Unmarshal(recipPlaintext, &recipInfo); err != nil {
        writeError("parse reciprocal PR: " + err.Error())
        return
    }

    // Read pubkeys from .bin files
    authorPub := readPubkey("users/" + authorInfo.AuthorBinHash + ".bin")
    recipPub := readPubkey("users/" + recipInfo.AuthorBinHash + ".bin")
    if authorPub == nil || recipPub == nil {
        writeError("could not read user pubkeys")
        return
    }

    // Compute pair hash
    a, b := authorInfo.AuthorMatchHash, recipInfo.AuthorMatchHash
    if a > b {
        a, b = b, a
    }
    ph := sha256Short(a + ":" + b)

    // Write match file
    os.MkdirAll("matches", 0755)
    matchJSON := fmt.Sprintf(`{"match_hash_1":"%s","match_hash_2":"%s","created_at":%d}`,
        authorInfo.AuthorMatchHash, recipInfo.AuthorMatchHash, time.Now().Unix())
    os.WriteFile("matches/"+ph+".json", []byte(matchJSON), 0644)

    // Encrypt + sign notifications
    authorNotif := encryptAndSign(recipPub, authorInfo.AuthorMatchHash, recipInfo.AuthorMatchHash,
        hex.EncodeToString(recipPub), recipInfo.Greeting, operatorKey)
    recipNotif := encryptAndSign(authorPub, recipInfo.AuthorMatchHash, authorInfo.AuthorMatchHash,
        hex.EncodeToString(authorPub), authorInfo.Greeting, operatorKey)

    // Git add/commit/push match file
    gh, err := github.NewCLI(repo)
    if err != nil {
        writeError("github CLI: " + err.Error())
        return
    }
    if err := gh.AddCommitPush([]string{"matches/"}, "Match: "+ph); err != nil {
        // Match file may already exist from concurrent Action — not fatal
        log.Printf("git push: %v (may be concurrent)", err)
    }

    // Close PRs, then post signed comments
    ctx := context.Background()
    prNum, _ := strconv.Atoi(prNumber)
    gh.ClosePR(ctx, prNum)
    gh.ClosePR(ctx, recipPR.Number)
    gh.CommentPR(ctx, prNum, message.Format("match", authorNotif))
    gh.CommentPR(ctx, recipPR.Number, message.Format("match", recipNotif))

    log.Printf("match created: pair=%s", ph)
}

func readPubkey(path string) []byte {
    data, err := os.ReadFile(path)
    if err != nil || len(data) < 32 {
        return nil
    }
    return data[:32]
}

func encryptAndSign(recipientPub []byte, recipientMatchHash, peerMatchHash, peerPubHex, greeting string, operatorKey []byte) string {
    payload, _ := json.Marshal(map[string]string{
        "matched_match_hash": peerMatchHash,
        "greeting":           greeting,
        "pubkey":             peerPubHex,
    })
    encrypted, err := crypto.Encrypt(ed25519.PublicKey(recipientPub), payload)
    if err != nil {
        log.Printf("encrypt notification: %v", err)
        return ""
    }
    sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), encrypted)
    return base64.StdEncoding.EncodeToString(encrypted) + "." + hex.EncodeToString(sig)
}

func sha256Short(input string) string {
    h := sha256.Sum256([]byte(input))
    return hex.EncodeToString(h[:])[:16]
}

func writeError(msg string) {
    fmt.Fprintf(os.Stderr, "error: %s\n", msg)
    os.Exit(1)
}
```

Add imports: `"context"`, `"crypto/sha256"`, `"strconv"`, `"time"`, `"github.com/vutran1710/dating-dev/internal/github"`, `"github.com/vutran1710/dating-dev/internal/message"`

Note: `sha256Short` and `writeError` are now in both `regcrypt` and `matchcrypt`. That's fine — they're 3-line functions in separate binaries.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/matchcrypt/`

- [ ] **Step 3: Commit**

```bash
git add cmd/matchcrypt/main.go
git commit -m "feat: matchcrypt match — full match flow with git+gh via CLIClient"
```

---

### Task 4: Update CLI — issue body uses message.Format

**Files:**
- Modify: `internal/github/pool.go`

- [ ] **Step 1: Update SubmitRegistration to use message.Format**

Find the `SubmitRegistration` function (around line 185). Replace the body construction:

```go
// Current:
body := fmt.Sprintf(
    "<!-- registration-request -->\n\n"+
        "**User Hash:**\n```\n%s\n```\n\n"+
        "**Public Key:**\n```\n%s\n```\n\n"+
        "**Profile Blob:**\n```\n%s\n```\n\n"+
        "**Signature:**\n```\n%s\n```\n\n"+
        "**Identity Proof:**\n```\n%s\n```",
    userHash, pubKeyHex, blobHex, signature, identityProof,
)

// New:
content := strings.Join([]string{userHash, pubKeyHex, blobHex, signature, identityProof}, "\n")
body := message.Format("registration-request", content)
```

Add import: `"github.com/vutran1710/dating-dev/internal/message"`

- [ ] **Step 2: Update tryDecryptComment to use message.Parse**

```go
func tryDecryptComment(body string, operatorPub ed25519.PublicKey, userPriv ed25519.PrivateKey) (binHash, matchHash string, err error) {
    blockType, content, err := message.Parse(body)
    if err != nil || blockType != "registration" {
        return "", "", fmt.Errorf("not a registration comment")
    }

    // content is the signed blob: base64.hex_sig
    parts := strings.SplitN(content, ".", 2)
    if len(parts) != 2 {
        return "", "", fmt.Errorf("unsigned comment")
    }

    ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return "", "", fmt.Errorf("invalid base64: %w", err)
    }

    sigBytes, err := hex.DecodeString(parts[1])
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return "", "", fmt.Errorf("invalid signature")
    }

    if !ed25519.Verify(operatorPub, ciphertext, sigBytes) {
        return "", "", fmt.Errorf("signature verification failed")
    }

    plaintext, err := crypto.Decrypt(userPriv, ciphertext)
    if err != nil {
        return "", "", err
    }

    var hashes map[string]string
    if err := msgpack.Unmarshal(plaintext, &hashes); err != nil {
        return "", "", fmt.Errorf("msgpack decode: %w", err)
    }
    bin, ok1 := hashes["bin_hash"]
    match, ok2 := hashes["match_hash"]
    if !ok1 || !ok2 {
        return "", "", fmt.Errorf("missing bin_hash or match_hash")
    }
    return bin, match, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 4: Update existing pool_test.go tests for new format**

The `TestTryVerifyAndDecrypt_*` tests currently create comments in `base64.hexsig` format. They need to wrap in `message.Format("registration", ...)`:

```go
// Before:
comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

// After:
signedBlob := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)
comment := message.Format("registration", signedBlob)
```

- [ ] **Step 5: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/github/ -count=1`

- [ ] **Step 6: Commit**

```bash
git add internal/github/pool.go internal/github/pool_test.go
git commit -m "refactor: use message.Format/Parse in registration flow"
```

---

### Task 5: Update match notification parsing

**Files:**
- Modify: `internal/cli/tui/screens/matches.go`
- Modify: `internal/cli/matches.go`

- [ ] **Step 1: Update decryptMatchNotification in matches.go (TUI)**

Use `message.Parse` instead of raw base64 decode:

```go
func decryptMatchNotification(body string, operatorPub ed25519.PublicKey, priv ed25519.PrivateKey) (*MatchItem, error) {
    blockType, content, err := message.Parse(body)
    if err != nil || blockType != "match" {
        return nil, fmt.Errorf("not a match comment")
    }

    parts := strings.SplitN(content, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("unsigned comment")
    }
    // ... rest of signature verification + decryption unchanged
```

Add import: `"github.com/vutran1710/dating-dev/internal/message"`

- [ ] **Step 2: Update decryptMatchComment in internal/cli/matches.go**

Same pattern:

```go
func decryptMatchComment(body string, operatorPub ed25519.PublicKey, priv ed25519.PrivateKey) (*Match, error) {
    blockType, content, err := message.Parse(body)
    if err != nil || blockType != "match" {
        return nil, fmt.Errorf("not a match comment")
    }
    // ... rest unchanged, use content instead of body
```

- [ ] **Step 3: Update matches_test.go**

Update `signedComment` helper to wrap in `message.Format("match", ...)`.

- [ ] **Step 4: Verify build + tests**

Run: `go build ./... && DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/cli/matches.go internal/cli/matches_test.go internal/cli/tui/screens/matches.go
git commit -m "refactor: use message.Parse in match notification decryption"
```

---

### Task 6: Update Action templates

**Files:**
- Modify: `templates/actions/pool-register.yml`
- Modify: `templates/actions/pool-interest.yml`

- [ ] **Step 1: Replace pool-register.yml**

The tool handles everything now — git+gh included:

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
        run: ./regcrypt register
```

- [ ] **Step 2: Replace pool-interest.yml**

The only bash is `gh pr list` to feed reciprocal PRs as env var:

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
```

- [ ] **Step 3: Commit**

```bash
git add templates/actions/
git commit -m "refactor: thin Action templates — all logic in Go tools"
```

---

### Task 7: Full test suite + E2E

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: Deploy updated Actions to test pool**

```bash
cp templates/actions/pool-register.yml /Users/vutran/Works/terminal-dating/dating-test-pool/.github/workflows/register.yml
cp templates/actions/pool-interest.yml /Users/vutran/Works/terminal-dating/dating-test-pool/.github/workflows/interest.yml
cd /Users/vutran/Works/terminal-dating/dating-test-pool && git add -A && git commit -m "update Actions with refactored templates" && git push
```

- [ ] **Step 3: Publish updated regcrypt + matchcrypt binaries**

Build and publish to `vutran1710/regcrypt` releases so the Actions can download them.

- [ ] **Step 4: Test registration E2E**

Register a test user via `pool join` (or managed account). Verify:
- Issue body uses `<!-- openpool:registration-request -->` format
- Action runs `regcrypt register`
- Comment uses `<!-- openpool:registration -->` format
- CLI polls and successfully decrypts

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: E2E verified action tool refactor"
```
