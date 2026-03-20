# Operator-Signed Comments Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sign all Action-posted comments with the operator's ed25519 private key so CLI can verify authenticity before decrypting, preventing comment injection attacks.

**Architecture:** Action tools (`regcrypt`, `matchcrypt`) gain a `sign` subcommand. Action templates sign blobs before posting. CLI verify-then-decrypt using operator pubkey from pool config. Comments iterated in reverse order (newest first).

**Tech Stack:** Go, ed25519, base64, GitHub Actions

---

## File Structure

### Modified Files
- `cmd/regcrypt/main.go` — Refactor to subcommand pattern (`hash` + `sign`), add `sign` subcommand
- `cmd/matchcrypt/main.go` — Add `sign` subcommand
- `templates/actions/pool-register.yml` — Close issue first, then post signed comment
- `templates/actions/pool-interest.yml` — Sign both match notification comments
- `internal/github/pool.go` — `PollRegistrationResult` + `tryDecryptComment` accept `operatorPub`, verify signature, iterate reverse
- `internal/cli/pool.go` — Pass `operatorPub` to polling calls
- `internal/cli/tui/screens/matches.go` — `decryptMatchNotification` accepts `operatorPub`, verify signature
- `internal/cli/matches.go` — `decryptMatchComment` accepts `operatorPub` (CLI matches command)

### Test Files
- `cmd/regcrypt/main_test.go` — Test `sign` subcommand
- `internal/github/pool_test.go` — Test `tryVerifyAndDecrypt` (valid, forged, unsigned, tampered)

---

### Task 1: Add `sign` subcommand to regcrypt

**Files:**
- Modify: `cmd/regcrypt/main.go`

- [ ] **Step 1: Refactor regcrypt to subcommand pattern**

Currently `regcrypt` uses `flag.Parse()` with no subcommands. Refactor to `switch os.Args[1]` pattern (matching `matchcrypt`'s style):

```go
package main

import (
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "io"
    "log"
    "os"

    "github.com/vmihailenco/msgpack/v5"
    "github.com/vutran1710/dating-dev/internal/crypto"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "Usage: regcrypt <hash|sign> [flags]")
        os.Exit(1)
    }

    switch os.Args[1] {
    case "hash":
        cmdHash()
    case "sign":
        cmdSign()
    default:
        // Backward compat: if first arg starts with "--", treat as old flag-based usage
        cmdHashLegacy()
    }
}

func cmdHash() {
    // Move existing flag.Parse logic here, but parse os.Args[2:]
    fs := flag.NewFlagSet("hash", flag.ExitOnError)
    poolURL := fs.String("pool-url", "", "pool repo (owner/repo)")
    provider := fs.String("provider", "github", "auth provider")
    userID := fs.String("user-id", "", "provider user ID")
    salt := fs.String("salt", "", "pool salt")
    userPubHex := fs.String("user-pubkey", "", "user's ed25519 public key (hex)")
    fs.Parse(os.Args[2:])

    if *poolURL == "" || *userID == "" || *salt == "" || *userPubHex == "" {
        fs.Usage()
        os.Exit(1)
    }

    idH := crypto.UserHash(*poolURL, *provider, *userID).String()
    binH := sha256Short(*salt + ":" + idH)
    matchH := sha256Short(*salt + ":" + binH)

    userPub, err := hex.DecodeString(*userPubHex)
    if err != nil || len(userPub) != ed25519.PublicKeySize {
        log.Fatal("invalid user pubkey")
    }

    payload, _ := msgpack.Marshal(map[string]string{
        "bin_hash":   binH,
        "match_hash": matchH,
    })

    encrypted, err := crypto.Encrypt(ed25519.PublicKey(userPub), payload)
    if err != nil {
        log.Fatalf("encrypt: %v", err)
    }

    fmt.Println(binH)
    fmt.Println(matchH)
    fmt.Println(base64.StdEncoding.EncodeToString(encrypted))
}

func cmdHashLegacy() {
    // Same as cmdHash but uses flag.Parse() with os.Args (backward compat)
    // Existing behavior for Actions that call: ./regcrypt --pool-url ...
    poolURL := flag.String("pool-url", "", "pool repo")
    provider := flag.String("provider", "github", "auth provider")
    userID := flag.String("user-id", "", "provider user ID")
    salt := flag.String("salt", "", "pool salt")
    userPubHex := flag.String("user-pubkey", "", "user pubkey hex")
    flag.Parse()
    // ... same logic as cmdHash
}

func cmdSign() {
    operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
    operatorKey, err := hex.DecodeString(operatorKeyHex)
    if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
        log.Fatal("invalid operator key (expected 128 hex chars / 64 bytes)")
    }
    data, err := io.ReadAll(os.Stdin)
    if err != nil {
        log.Fatalf("reading stdin: %v", err)
    }
    sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), data)
    fmt.Print(hex.EncodeToString(sig))
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

func sha256Short(input string) string {
    h := sha256.Sum256([]byte(input))
    return hex.EncodeToString(h[:])[:16]
}
```

Actually — simpler approach: keep the existing flag-based behavior as default (Actions already call `./regcrypt --pool-url ...`), and just add `sign` as a subcommand check at the top:

```go
func main() {
    if len(os.Args) >= 2 && os.Args[1] == "sign" {
        cmdSign()
        return
    }
    // Existing flag-based hash logic unchanged
    poolURL := flag.String("pool-url", "", "pool repo (owner/repo)")
    // ... rest of existing code
}
```

This is the minimal change. No refactor needed.

- [ ] **Step 2: Add envOrArg helper (copy from matchcrypt)**

```go
func envOrArg(flagName, envVar string) string {
    for i, arg := range os.Args {
        if arg == flagName && i+1 < len(os.Args) {
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
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/regcrypt/`

- [ ] **Step 4: Test sign manually**

```bash
echo -n "test data" | go run ./cmd/regcrypt/ sign --operator-key $(cat /Users/vutran/Works/terminal-dating/dating-test-pool/.dev-secrets | grep OPERATOR_PRIVATE_KEY | cut -d= -f2)
```

Should output 128 hex chars.

- [ ] **Step 5: Commit**

```bash
git add cmd/regcrypt/main.go
git commit -m "feat: add sign subcommand to regcrypt"
```

---

### Task 2: Add `sign` subcommand to matchcrypt

**Files:**
- Modify: `cmd/matchcrypt/main.go`

- [ ] **Step 1: Add sign case to the existing switch**

`matchcrypt` already uses `switch os.Args[1]`. Just add:

```go
case "sign":
    cmdSign()
```

And the `cmdSign` function (identical to regcrypt's):

```go
func cmdSign() {
    operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
    operatorKey, err := hex.DecodeString(operatorKeyHex)
    if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
        log.Fatal("invalid operator key (expected 128 hex chars / 64 bytes)")
    }
    data, err := io.ReadAll(os.Stdin)
    if err != nil {
        log.Fatalf("reading stdin: %v", err)
    }
    sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), data)
    fmt.Print(hex.EncodeToString(sig))
}
```

Add `"io"` to imports.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/matchcrypt/`

- [ ] **Step 3: Commit**

```bash
git add cmd/matchcrypt/main.go
git commit -m "feat: add sign subcommand to matchcrypt"
```

---

### Task 3: Update `tryDecryptComment` to verify signature

**Files:**
- Modify: `internal/github/pool.go`

- [ ] **Step 1: Write tests for signature verification**

Add to `internal/github/pool_test.go` (or create it):

```go
func TestTryVerifyAndDecrypt_ValidSignature(t *testing.T) {
    // Generate operator keypair
    operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
    // Generate user keypair
    _, userPriv, _ := ed25519.GenerateKey(rand.Reader)
    userPub := userPriv.Public().(ed25519.PublicKey)

    // Create payload
    payload, _ := msgpack.Marshal(map[string]string{
        "bin_hash": "abc123", "match_hash": "def456",
    })

    // Encrypt to user
    ciphertext, _ := crypto.Encrypt(userPub, payload)

    // Sign with operator
    sig := ed25519.Sign(operatorPriv, ciphertext)
    comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

    bin, match, err := tryDecryptComment(comment, operatorPub, userPriv)
    if err != nil { t.Fatal(err) }
    if bin != "abc123" || match != "def456" { t.Fatalf("got %s %s", bin, match) }
}

func TestTryVerifyAndDecrypt_ForgedSignature(t *testing.T) {
    _, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
    operatorPub := operatorPriv.Public().(ed25519.PublicKey)
    _, userPriv, _ := ed25519.GenerateKey(rand.Reader)
    userPub := userPriv.Public().(ed25519.PublicKey)

    // Attacker encrypts to user's pubkey but signs with different key
    _, attackerPriv, _ := ed25519.GenerateKey(rand.Reader)
    payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "evil", "match_hash": "evil"})
    ciphertext, _ := crypto.Encrypt(userPub, payload)
    sig := ed25519.Sign(attackerPriv, ciphertext) // wrong key!
    comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

    _, _, err := tryDecryptComment(comment, operatorPub, userPriv)
    if err == nil { t.Fatal("forged signature should be rejected") }
}

func TestTryVerifyAndDecrypt_UnsignedComment(t *testing.T) {
    operatorPub, _, _ := ed25519.GenerateKey(rand.Reader)
    _, userPriv, _ := ed25519.GenerateKey(rand.Reader)

    // No dot separator — old format
    _, _, err := tryDecryptComment("c29tZWJhc2U2NA==", operatorPub, userPriv)
    if err == nil { t.Fatal("unsigned comment should be rejected") }
}

func TestTryVerifyAndDecrypt_TamperedCiphertext(t *testing.T) {
    operatorPub, operatorPriv, _ := ed25519.GenerateKey(rand.Reader)
    _, userPriv, _ := ed25519.GenerateKey(rand.Reader)
    userPub := userPriv.Public().(ed25519.PublicKey)

    payload, _ := msgpack.Marshal(map[string]string{"bin_hash": "abc", "match_hash": "def"})
    ciphertext, _ := crypto.Encrypt(userPub, payload)
    sig := ed25519.Sign(operatorPriv, ciphertext)

    // Tamper with ciphertext
    ciphertext[0] ^= 0xFF
    comment := base64.StdEncoding.EncodeToString(ciphertext) + "." + hex.EncodeToString(sig)

    _, _, err := tryDecryptComment(comment, operatorPub, userPriv)
    if err == nil { t.Fatal("tampered ciphertext should fail signature check") }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `DATING_HOME=$(mktemp -d) go test -v -run "TestTryVerify" ./internal/github/`
Expected: FAIL (wrong number of arguments to tryDecryptComment)

- [ ] **Step 3: Update tryDecryptComment**

```go
func tryDecryptComment(body string, operatorPub ed25519.PublicKey, userPriv ed25519.PrivateKey) (binHash, matchHash string, err error) {
    body = strings.TrimSpace(body)

    // Split into base64 blob + hex signature
    parts := strings.SplitN(body, ".", 2)
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

    // Verify operator signature
    if !ed25519.Verify(operatorPub, ciphertext, sigBytes) {
        return "", "", fmt.Errorf("signature verification failed")
    }

    // Decrypt
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

Add `"encoding/hex"` to imports if not already present.

- [ ] **Step 4: Update PollRegistrationResult signature and iteration order**

```go
func (p *Pool) PollRegistrationResult(ctx context.Context, issueNumber int, operatorPub ed25519.PublicKey, userPriv ed25519.PrivateKey) (binHash, matchHash string, err error) {
```

And change the comment iteration to reverse order:

```go
// Iterate comments in reverse (newest first — Action's comment is last)
for i := len(comments) - 1; i >= 0; i-- {
    c := comments[i]
    if c.User.Login != "github-actions[bot]" {
        continue
    }
    bin, match, decErr := tryDecryptComment(c.Body, operatorPub, userPriv)
    if decErr == nil {
        return bin, match, nil
    }
}
```

- [ ] **Step 5: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v -run "TestTryVerify" ./internal/github/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/github/pool.go internal/github/pool_test.go
git commit -m "feat: verify operator signature before decrypting comments"
```

---

### Task 4: Update CLI callers to pass operatorPub

**Files:**
- Modify: `internal/cli/pool.go`

- [ ] **Step 1: Update pool join caller (around line 392)**

```go
// Decode operator pubkey
operatorPubBytes, err := hex.DecodeString(entry.OperatorPubKey)
if err != nil {
    return fmt.Errorf("invalid operator pubkey: %w", err)
}

binHash, matchHash, err := poolGH.PollRegistrationResult(pollCtx, issueNumber, ed25519.PublicKey(operatorPubBytes), priv)
```

- [ ] **Step 2: Update pool list re-poll caller (around line 498)**

```go
opPub, opErr := hex.DecodeString(p.OperatorPubKey)
if opErr == nil {
    binHash, matchHash, pollErr := poolGH.PollRegistrationResult(pollCtx, p.PendingIssue, ed25519.PublicKey(opPub), priv)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/cli/pool.go
git commit -m "fix: pass operator pubkey to PollRegistrationResult"
```

---

### Task 5: Update decryptMatchNotification to verify signature

**Files:**
- Modify: `internal/cli/tui/screens/matches.go`
- Modify: `internal/cli/matches.go` (if it has a separate decryptMatchComment)

- [ ] **Step 1: Update decryptMatchNotification signature**

```go
func decryptMatchNotification(body string, operatorPub ed25519.PublicKey, priv ed25519.PrivateKey) (*MatchItem, error) {
    body = strings.TrimSpace(body)

    // Verify operator signature
    parts := strings.SplitN(body, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("unsigned comment")
    }

    blobBytes, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, err
    }

    sigBytes, err := hex.DecodeString(parts[1])
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return nil, fmt.Errorf("invalid signature")
    }

    if !ed25519.Verify(operatorPub, blobBytes, sigBytes) {
        return nil, fmt.Errorf("signature verification failed")
    }

    plaintext, err := crypto.Decrypt(priv, blobBytes)
    // ... rest unchanged
```

- [ ] **Step 2: Update LoadMatchesCmd to pass operatorPub**

In `LoadMatchesCmd`, decode `pool.OperatorPubKey` and pass to `decryptMatchNotification`:

```go
operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
if err != nil {
    return MatchesFetchedMsg{Err: fmt.Errorf("invalid operator pubkey")}
}
operatorPub := ed25519.PublicKey(operatorPubBytes)

// ... in the comment loop:
m, err := decryptMatchNotification(c.Body, operatorPub, priv)
```

- [ ] **Step 3: Update `internal/cli/matches.go` if it has a separate `decryptMatchComment`**

Same pattern — add `operatorPub` parameter, verify before decrypt.

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/cli/tui/screens/matches.go internal/cli/matches.go
git commit -m "feat: verify operator signature in match notifications"
```

---

### Task 6: Update Action templates — sign comments

**Files:**
- Modify: `templates/actions/pool-register.yml`
- Modify: `templates/actions/pool-interest.yml`

- [ ] **Step 1: Update pool-register.yml**

Change the comment posting to: close issue first, then sign + post comment.

In the "Parse and register" step, replace:
```bash
gh issue comment "$ISSUE_NUMBER" --body "$ENCRYPTED_BLOB"
```

With:
```bash
# Sign the encrypted blob
SIGNATURE=$(echo -n "$ENCRYPTED_BLOB" | base64 -d | ./regcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
```

Add `OPERATOR_PRIVATE_KEY: ${{ secrets.OPERATOR_PRIVATE_KEY }}` to the env block.

Move the "Close issue" step BEFORE the comment posting. Then post:
```bash
gh issue comment "$ISSUE_NUMBER" --body "${ENCRYPTED_BLOB}.${SIGNATURE}"
```

Reorder: commit .bin → close issue → post signed comment.

- [ ] **Step 2: Update pool-interest.yml**

For both notification comments, sign before posting:

```bash
# Sign notification for current PR author
AUTHOR_SIG=$(echo -n "$AUTHOR_NOTIFICATION" | base64 -d | ./matchcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
gh pr comment "$PR_NUMBER" --body "${AUTHOR_NOTIFICATION}.${AUTHOR_SIG}"

# Sign notification for reciprocal PR author
RECIP_SIG=$(echo -n "$RECIP_NOTIFICATION" | base64 -d | ./matchcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
gh pr comment "$RECIP_NUMBER" --body "${RECIP_NOTIFICATION}.${RECIP_SIG}"
```

- [ ] **Step 3: Commit**

```bash
git add templates/actions/pool-register.yml templates/actions/pool-interest.yml
git commit -m "feat: sign Action comments with operator key"
```

---

### Task 7: Run full test suite

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: Fix any failures**

Common issues:
- Tests that call old `tryDecryptComment(body, priv)` with 2 args → need 3 args now
- Tests that call `PollRegistrationResult` with old signature

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "fix: all tests pass with operator-signed comments"
```

---

### Task 8: E2E test with real test pool

- [ ] **Step 1: Update test pool Actions**

Copy the updated Action templates to the test pool repo and push:
```bash
cp templates/actions/pool-register.yml /Users/vutran/Works/terminal-dating/dating-test-pool/.github/workflows/register.yml
cp templates/actions/pool-interest.yml /Users/vutran/Works/terminal-dating/dating-test-pool/.github/workflows/interest.yml
cd /Users/vutran/Works/terminal-dating/dating-test-pool && git add -A && git commit -m "update Actions with operator-signed comments" && git push
```

- [ ] **Step 2: Register a test user via pool join**

```bash
DATING_HOME=$(mktemp -d) go run ./cmd/dating/ pool join test-pool
```

Wait for Action to run. CLI should verify the signed comment and extract hashes.

- [ ] **Step 3: Verify the comment format on GitHub**

Check the issue comment — should be `base64blob.hexsignature` (with a dot separator).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test: E2E verified operator-signed comments"
```

---

### Task 9: Create SECURITY_DESIGN.md

**Files:**
- Create: `SECURITY_DESIGN.md` (project root)

- [ ] **Step 1: Write comprehensive security design document**

Consolidate all security rationale into one document. Include:

1. **Hash Chain** — `real_id → id_hash → bin_hash → match_hash`, unlinkability, salt placement
2. **Relay Auth (TOTP)** — time-based public key authentication, chain validation, no shared secrets
3. **Channel binding** — proposed improvement: include relay hostname in signed message
4. **E2E Encrypted Chat** — ECDH key derivation, NaCl secretbox, relay zero-knowledge
5. **Match Notification as Key Exchange** — peer pubkey delivery, no relay key exchange protocol
6. **Operator-Signed Comments** — comment injection attack vector, signature verification, reverse iteration
7. **Defense in Depth** — author filter + operator signature + encryption (3 layers)
8. **Registration Security** — operator controls identity via GitHub Actions secrets, pubkey trust model
9. **Threat Model** — what's protected, what's not, trust boundaries

Source material:
- `docs/concepts/relay-auth-security.md`
- `docs/superpowers/specs/2026-03-20-operator-signed-comments.md`
- CLAUDE.md security rationale section
- `internal/crypto/` package documentation

- [ ] **Step 2: Commit**

```bash
git add SECURITY_DESIGN.md
git commit -m "docs: comprehensive security design document"
```
