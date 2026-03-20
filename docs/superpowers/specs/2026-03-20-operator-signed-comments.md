# Operator-Signed Action Comments

## Goal

Prevent comment injection attacks on registration issues and interest PRs by signing all Action-posted comments with the operator's ed25519 private key. CLI verifies the signature before decrypting.

## Problem

Anyone can post comments on public GitHub issues/PRs. A user's ed25519 public key is visible in `.bin` files (first 32 bytes). An attacker can:

1. Watch for a registration issue, grab the user's pubkey from the issue body
2. Encrypt a fake `{bin_hash, match_hash}` to the user's pubkey
3. Post the fake comment before or alongside the Action's real comment
4. CLI polls, decrypts the fake comment successfully, saves wrong hashes

Same attack on interest PRs — attacker injects a fake match notification with their own pubkey, MITM-ing the key exchange.

## Fix

### Comment Format

```
Current:   base64(ciphertext)
New:       base64(ciphertext).hex(ed25519_signature)
```

- Separator: `.` (dot)
- Signature: `ed25519.Sign(operator_private_key, ciphertext_bytes)`
- Signature covers the raw ciphertext bytes, not the base64 string
- Operator private key is 64 bytes (128 hex chars) — Go's `ed25519.PrivateKey` extended form (seed + public key)

### Action Side

Add `sign` subcommand to both `regcrypt` and `matchcrypt`. Both currently use `switch os.Args[1]` for subcommand routing (`matchcrypt` already has this; `regcrypt` needs to be refactored from `flag.Parse()` to the same `switch` pattern).

```
echo -n "<raw_ciphertext_bytes>" | regcrypt sign --operator-key <hex>
→ stdout: hex signature
```

Implementation:
```go
func cmdSign() {
    operatorKeyHex := envOrArg("--operator-key", "OPERATOR_PRIVATE_KEY")
    operatorKey, err := hex.DecodeString(operatorKeyHex)
    if err != nil || len(operatorKey) != ed25519.PrivateKeySize {
        log.Fatal("invalid operator key (expected 128 hex chars)")
    }
    data, _ := io.ReadAll(os.Stdin)
    sig := ed25519.Sign(ed25519.PrivateKey(operatorKey), data)
    fmt.Print(hex.EncodeToString(sig))
}
```

Action templates — pipe base64-decoded blob directly to `sign` stdin (binary-safe, no shell variable for raw bytes):

```bash
# Registration (pool-register.yml)
SIGNATURE=$(echo -n "$ENCRYPTED_BLOB" | base64 -d | ./regcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
gh issue comment "$ISSUE_NUMBER" --body "${ENCRYPTED_BLOB}.${SIGNATURE}"

# Interest (pool-interest.yml) — same pattern for both notifications
SIGNATURE=$(echo -n "$AUTHOR_NOTIFICATION" | base64 -d | ./matchcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
gh pr comment "$PR_NUMBER" --body "${AUTHOR_NOTIFICATION}.${SIGNATURE}"
```

### CLI Side

Add `verifyAndDecrypt` helper (in `internal/crypto/` or inline):

```go
func verifyAndDecrypt(commentBody string, operatorPub ed25519.PublicKey, priv ed25519.PrivateKey) ([]byte, error) {
    parts := strings.SplitN(commentBody, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("unsigned comment")
    }

    ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
    if err != nil {
        return nil, fmt.Errorf("invalid base64")
    }

    sigBytes, err := hex.DecodeString(parts[1])
    if err != nil || len(sigBytes) != ed25519.SignatureSize {
        return nil, fmt.Errorf("invalid signature")
    }

    if !ed25519.Verify(operatorPub, ciphertext, sigBytes) {
        return nil, fmt.Errorf("signature verification failed")
    }

    return crypto.Decrypt(priv, ciphertext)
}
```

### API Surface Changes

**Registration polling** (`internal/github/pool.go`):
- `PollRegistrationResult(ctx, issueNumber, userPriv)` → `PollRegistrationResult(ctx, issueNumber, userPriv, operatorPub)`
- `tryDecryptComment(body, priv)` → `tryVerifyAndDecrypt(body, operatorPub, priv)`
- Callers in `internal/cli/pool.go` (`newPoolJoinCmd`, `newPoolListCmd`) must pass `operatorPub` decoded from `pool.OperatorPubKey`

**Match notification decryption** (`internal/cli/tui/screens/matches.go`):
- `decryptMatchNotification(body, priv)` → `decryptMatchNotification(body, operatorPub, priv)`
- `LoadMatchesCmd` must decode `pool.OperatorPubKey` and pass it through

### Operator Public Key

Already available in pool config:
```go
pool.OperatorPubKey // hex string, set during pool add/join
```

Decoded once per call site:
```go
operatorPub, err := hex.DecodeString(pool.OperatorPubKey)
```

## Comment Iteration Order

CLI iterates comments in **reverse order** (newest first, skipping the issue body at index 0). The Action posts its signed comment last (after closing the issue), so the real comment is at the end. This makes the CLI resilient to comment flooding — even with 100 fake comments injected before close, the CLI checks the last comment first and finds the signed one in O(1).

```go
for i := len(comments) - 1; i >= 0; i-- {
    result, err := verifyAndDecrypt(comments[i].Body, operatorPub, priv)
    if err == nil {
        return result
    }
}
```

The Action should **close the issue first, then post the signed comment** — this way the real comment is always the last one.

## Migration

Per project convention: **no backward compatibility during pre-launch**. Old unsigned comments are silently skipped. Any pending registrations from before this change must be re-submitted (close old issue, run `pool join` again). Old `.bin` files are unaffected (not signed).

## Files Modified

| File | Change |
|------|--------|
| `cmd/regcrypt/main.go` | Refactor to subcommand pattern, add `sign` subcommand |
| `cmd/matchcrypt/main.go` | Add `sign` subcommand |
| `templates/actions/pool-register.yml` | Sign comment before posting |
| `templates/actions/pool-interest.yml` | Sign both notification comments |
| `internal/github/pool.go` | `PollRegistrationResult` + `tryDecryptComment` accept `operatorPub` |
| `internal/cli/pool.go` | Pass `operatorPub` to polling functions |
| `internal/cli/tui/screens/matches.go` | `decryptMatchNotification` accepts `operatorPub`, `LoadMatchesCmd` passes it |

Out of scope: `internal/cli/chat.go` peer pubkey loading (future work, marked as TODO).

## Testing

- Unit test: `verifyAndDecrypt` with valid operator signature → decrypts successfully
- Unit test: `verifyAndDecrypt` with forged signature → rejected
- Unit test: `verifyAndDecrypt` with unsigned comment (no `.`) → rejected
- Unit test: `verifyAndDecrypt` with tampered ciphertext → signature fails
- Unit test: `regcrypt sign` / `matchcrypt sign` produces valid ed25519 signature verifiable with operator pubkey
- Integration: sign with operator key, verify with operator pubkey, decrypt with user key
- E2E: register a user via test pool with updated Action, verify CLI accepts the signed comment

## Defense in Depth

After this change, three layers protect against comment injection:

1. **Author check** — CLI only reads `github-actions[bot]` comments (existing, noise reduction — not a security boundary since any GitHub App can post as this user)
2. **Operator signature** — comment must be signed by operator private key (new, **the real security layer**)
3. **Decryption** — comment must be encrypted to the user's pubkey (existing)

An attacker would need the operator's private key to forge a valid signed comment.
