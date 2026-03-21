# Security Design

This document consolidates the security rationale for the terminal-dating platform. It covers the
trust model, cryptographic primitives, threat model, and known limitations. It is a living document
— sections marked **[planned]** describe improvements not yet implemented.

---

## 1. Overview

The platform stores user data in public GitHub repositories (pool repos). All sensitive data is
encrypted before it reaches GitHub. The relay server routes ciphertext only and never sees plaintext.
GitHub itself is used purely for storage and notification delivery, not for trust or access control.

### What the system protects

- **Identity unlinkability**: discovery identity (bin_hash), matching identity (match_hash), and
  registration identity (id_hash) are three separate unlinkable namespaces.
- **Profile confidentiality**: profiles are encrypted to the operator's public key; only the
  operator and the profile owner can read them.
- **Chat confidentiality**: messages are E2E encrypted with a key derived from ECDH; the relay
  routes ciphertext only.
- **Comment integrity**: Action-posted comments are signed by the operator's private key; clients
  reject unsigned or forged comments.

### Trust model

| Entity | Trusted for | Not trusted for |
|--------|-------------|-----------------|
| GitHub | Hosting, Action execution, raw content serving | Access control (repos are public), comment authenticity |
| Operator | Signing comments, holding salt, running relay | Reading plaintext chat (relay is zero-knowledge) |
| Relay | Routing ciphertext by match_hash | Decryption, key storage |
| Client | Local key material, decryption | Nothing on the server side |

---

## 2. Hash Chain

### Structure

```
real_id    = "github:<username>"
             │
             ▼  sha256(pool_url + ":" + provider + ":" + user_id)
id_hash    = <64 hex chars>          ← registration identity
             │
             ▼  sha256(salt + ":" + id_hash)[:16]
bin_hash   = <16 hex chars>          ← public discovery key, .bin filename
             │
             ▼  sha256(salt + ":" + bin_hash)[:16]
match_hash = <16 hex chars>          ← public chat handle, TOTP secret
```

Go type: `crypto.IDHash` for `id_hash`. Implementation: `internal/crypto/`.

### Unlinkability

Each step is a one-way hash keyed with a **pool salt**. The salt lives in exactly two places:

1. The pool repo's GitHub Actions secrets (`POOL_SALT`)
2. The relay server's environment variable (`POOL_SALT`)

Without the salt:

- Knowing `bin_hash` tells you nothing about `match_hash` (and vice versa).
- An attacker who observes both `bin_hash` and `match_hash` in the open — on a PR title or in a
  `.bin` filename — cannot link them to the same person.

### Limitations of unlinkability

**Computational linking** (blocked): without the salt, an observer can't derive `bin_hash → match_hash`. The chain is cryptographically sound.

**Timing correlation** (side-channel): when a new `.bin` file is committed and a new entry appears in `index.db` around the same time, an observer can correlate them. In a small pool (50 users), new registrations are obvious. The salt doesn't help here — the attacker bypasses the chain entirely.

**Mitigation: History squashing.** A periodic cron Action squashes the pool repo to a single commit. All `.bin` files + `index.db` appear simultaneously — no timeline to correlate. Gets stronger as pool grows.

```
Before squash:
  commit abc: Register user a1b2.bin     ← timestamp leaks
  commit def: Register user d4e5.bin     ← timestamp leaks
  commit ghi: Rebuild index.db          ← correlates with d4e5

After squash:
  commit xyz: Pool state                ← single commit, no timeline
```

Implemented as a cron Action that force-pushes a squashed history (e.g., weekly).

### Where each hash appears publicly

| Hash | Public location | Purpose |
|------|----------------|---------|
| `bin_hash` | `.bin` filename in pool repo | Discovery (keyed profile store) |
| `match_hash` | Interest PR title | Matching, chat routing |
| `id_hash` | Internal only (registration) | Compute bin_hash from GitHub identity |

### Salt placement

The salt is applied **before** each derivation, not after. This prevents length-extension attacks
and ensures the salt is fully mixed into the hash input.

---

## 3. Relay Authentication (TOTP)

### Mechanism

Time-based public key authentication — similar to SSH key auth with TOTP time windowing. No shared
secrets, no tokens, no sessions, no JWTs.

```
Client → connect: ws://relay/ws?id=<id_hash>&match=<match_hash>&sig=<totp_signature>

Relay:
  1. Derive bin_hash from id_hash using salt
  2. Derive match_hash from bin_hash using salt
  3. Assert derived match_hash == claimed match_hash
  4. Fetch pubkey from GitHub raw content (via bin_hash → .bin file)
  5. Verify: ed25519.Verify(pubkey, sha256(time_window), sig)
  6. Accept ±1 window (±300s = 15 min total tolerance for clock drift)
```

Implementation: `internal/crypto/totp.go`

```go
// Window = unix_timestamp / 300 (5-minute buckets)
func TOTPSign(priv ed25519.PrivateKey) string   // signs current window
func TOTPVerify(sigHex string, pub ed25519.PublicKey) bool  // checks now, now-1, now+1
```

### Strengths

- No shared secrets — asymmetric ed25519 only
- Private key never leaves the client
- Identity binding via chain validation: cannot claim another user's `match_hash` without knowing
  the salt
- Replay window limited to ~15 minutes
- Fully stateless relay — no session storage

### Known concerns

| Issue | Severity | Current mitigation |
|-------|----------|--------------------|
| Signature reuse within window | Low | TLS protects the WebSocket upgrade URL |
| No channel binding | Medium | TLS verifies relay server identity |
| Pubkey trust via GitHub | Medium | Only Actions can write `.bin` files (repo permissions) |

### Channel Binding [planned]

To prevent a signature obtained for one relay being accepted by another, include the relay hostname
in the signed message:

```
Current:  sign(sha256(time_window))
Proposed: sign(sha256(time_window + relay_host))
```

This requires:

- Client includes relay hostname (already in pool config)
- Relay includes its hostname (env var or `Host` header)
- API change: `TOTPSign(priv, relayHost)` / `TOTPVerify(sig, pub, relayHost)`

TLS already handles server identity verification; channel binding provides defense in depth.

---

## 4. E2E Encrypted Chat

### Key derivation

Chat keys are derived from ECDH using the users' existing ed25519 key pairs — no separate
encryption keys are needed.

```
1. Convert ed25519 private key  →  curve25519 scalar  (ed25519PrivToCurve25519)
2. Convert peer ed25519 pubkey  →  curve25519 point   (ed25519PubToCurve25519)
3. shared_secret = curve25519.X25519(myScalar, theirPoint)   (32 bytes)
4. conversation_key = HKDF-SHA256(shared_secret, info="dating-e2e:<hashA>:<hashB>:<poolURL>")
```

`hashA` and `hashB` are sorted lexicographically so both peers derive the identical key regardless
of argument order.

Implementation: `internal/crypto/conversation.go` — `SharedSecret`, `DeriveConversationKey`.

### Encryption

NaCl secretbox (XSalsa20-Poly1305):

```
SealRaw / OpenRaw   — binary wire format (relay binary frames)
SealMessage / OpenMessage — base64 encoded (stored messages)

Wire layout: version(1B) || nonce(24B) || secretbox_ciphertext
```

Random 24-byte nonce per message — no nonce reuse risk.

### Relay zero-knowledge

The relay sees only:

```
Binary frame: [target_match_hash (8 bytes)] [ciphertext]
```

The relay prepends the sender's `match_hash` before forwarding. It cannot read message content, and
it does not store messages beyond an in-memory offline queue (capacity 20).

---

## 5. Match Notification as Key Exchange

When a mutual match is detected, the GitHub Action (matchcrypt) encrypts a notification to each
user's public key. The notification contains the **peer's ed25519 public key**.

```
Mutual match detected
       │
       ├─ encrypt({peer_pubkey, peer_match_hash, ...}) to user_A's pubkey → post to PR
       └─ encrypt({peer_pubkey, peer_match_hash, ...}) to user_B's pubkey → post to PR
```

This means:

- The match notification IS the key exchange — no separate `key_request`/`key_response` protocol.
- Each user receives the peer's ed25519 pubkey encrypted specifically to their own pubkey.
- The client derives the ECDH shared secret immediately from the notification.
- The relay never needs to store or serve pubkeys.

Implementation: `cmd/matchcrypt/`, `internal/cli/tui/screens/matches.go`

---

## 6. Operator-Signed Comments

### Attack vector

Anyone can post comments on public GitHub issues and PRs. A user's ed25519 pubkey is visible in
`.bin` files (first 32 bytes). The attack:

1. Attacker watches for a new registration issue, reads the user's pubkey from the issue body.
2. Attacker encrypts fake `{bin_hash, match_hash}` to the user's pubkey.
3. Attacker posts the fake comment (before or alongside the real Action comment).
4. CLI polls, decrypts the fake comment successfully, saves wrong hashes.

The same attack applies to interest PRs: inject a fake match notification containing the attacker's
pubkey to MITM the key exchange.

### Fix: comment format

```
Old format:  base64(ciphertext)
New format:  base64(ciphertext).hex(ed25519_signature)
```

- Separator: `.` (dot)
- Signature: `ed25519.Sign(operator_private_key, ciphertext_bytes)`
- Signature covers raw ciphertext bytes (not the base64 string)

### Verification before decryption

The CLI must verify the operator signature **before** attempting decryption. A failed signature
immediately rejects the comment without calling any crypto decryption path.

```go
func verifyAndDecrypt(commentBody string, operatorPub ed25519.PublicKey, priv ed25519.PrivateKey) ([]byte, error) {
    parts := strings.SplitN(commentBody, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("unsigned comment")
    }
    ciphertext, _ := base64.StdEncoding.DecodeString(parts[0])
    sigBytes, _ := hex.DecodeString(parts[1])
    if !ed25519.Verify(operatorPub, ciphertext, sigBytes) {
        return nil, fmt.Errorf("signature verification failed")
    }
    return crypto.Decrypt(priv, ciphertext)
}
```

Implementation target: `internal/github/pool.go`, `internal/cli/tui/screens/matches.go`

### Close-before-comment ordering

The Action **closes the issue first, then posts the signed comment**. This ensures the real comment
is always the last one on the thread.

### Reverse iteration

The CLI iterates comments in reverse order (newest first):

```go
for i := len(comments) - 1; i >= 0; i-- {
    result, err := verifyAndDecrypt(comments[i].Body, operatorPub, priv)
    if err == nil {
        return result
    }
}
```

Even if an attacker floods 100 fake comments before close, the CLI finds the real signed comment
in O(1) by checking the last comment first.

### Action signing

The `regcrypt sign` and `matchcrypt sign` subcommands read ciphertext from stdin and emit a hex
signature to stdout:

```bash
# In pool-register.yml
SIGNATURE=$(echo -n "$ENCRYPTED_BLOB" | base64 -d | ./regcrypt sign --operator-key "$OPERATOR_PRIVATE_KEY")
gh issue comment "$ISSUE_NUMBER" --body "${ENCRYPTED_BLOB}.${SIGNATURE}"
```

Spec: `docs/superpowers/specs/2026-03-20-operator-signed-comments.md`

---

## 7. Defense in Depth

After the operator-signed comments change, three layers protect against comment injection:

```
Layer 1: Author filter
  CLI only reads github-actions[bot] comments
  → Noise reduction only. NOT a security boundary.
    Any GitHub App can post as this account.

Layer 2: Operator signature  ← THE REAL SECURITY BOUNDARY
  Comment must be signed by the operator's private key
  → Attacker needs operator private key to forge

Layer 3: Decryption
  Comment must decrypt successfully with the user's private key
  → Provides confidentiality, not integrity (attacker knows the pubkey)
```

An attacker who bypasses all three would need both the operator's private key and knowledge of the
user's pubkey (which is public). The operator key is the hard requirement.

---

## 8. Registration Security

### Identity control

The operator controls who gets registered via the GitHub Actions secret `OPERATOR_PRIVATE_KEY`.
Only Actions with access to this secret can:

- Sign valid registration comments
- Sign valid match notifications

The operator private key is **only** in GitHub Actions secrets. It is never committed to any
repository. See also: memory note `project_security_operator_key.md`.

### Pubkey trust model

`.bin` files are the source of truth for user pubkeys. Layout:

```
.bin file: [32B ed25519 pubkey][NaCl box encrypted profile]
```

The relay fetches pubkeys from GitHub raw content (`raw.githubusercontent.com`) and caches them.
Trust in pubkeys flows from:

1. Only the registration Action can commit `.bin` files (GitHub repo write permission is restricted
   to Actions).
2. The Action validates the registration issue before committing.
3. The operator signature on the response comment confirms the Action ran to completion.

### Key generation

Users generate their ed25519 key pair locally via `dating profile create`. The private key never
leaves `~/.dating/` (or `$DATING_HOME/`).

---

## 9. Private Key — The Ultimate Trust Boundary

The user's ed25519 private key is the single point of failure per user. If stolen (even without the config file), an attacker can:

1. **Find bin_hash** — match pubkey (derivable from private key) against all `.bin` files in the pool repo
2. **Decrypt registration comment** — scan issue comments, decrypt with stolen key → get bin_hash + match_hash
3. **Decrypt match notifications** — scan closed interest PRs, decrypt comments → get peer pubkeys
4. **Authenticate to relay** — connect with bin_hash + match_hash + valid signature → full impersonation
5. **Read/send chat messages** — derive ECDH shared secrets with matched peers

**No additional secrets help.** Any "link_secret" or extra hash layer would still be delivered via encrypted comment (encrypted to the user's pubkey). Stealing the private key unlocks everything.

**This is an acceptable threat model** — identical to SSH, PGP, Signal. Mitigations:

- Key file stored with `0600` permissions (`internal/crypto/keys.go`)
- Recovery: generate new keypair, re-register (`pool join` again). The new `.bin` file has the new pubkey — relay rejects old key signatures.
- Blast radius: one user only. No impact on other users, the pool, or the operator.

---

## 10. Threat Model

### Protected

| Threat | Protection |
|--------|-----------|
| Passive observer linking bin_hash ↔ match_hash | Hash chain (requires salt) + history squashing (blocks timing correlation) |
| Fake registration comment | Operator signature required |
| Fake match notification with attacker pubkey | Operator signature required |
| Chat content eavesdropping at relay | E2E encryption (relay is zero-knowledge) |
| Relay impersonation | TLS + [planned] channel binding |
| Comment flood to delay CLI | Reverse iteration finds real comment in O(1) |
| Clock skew at relay auth | ±1 TOTP window (15 min tolerance) |

### Not protected / known limitations

| Issue | Notes |
|-------|-------|
| GitHub as storage is public | Encrypted profiles are public. Metadata (file counts, timestamps) is visible. |
| Relay can observe traffic patterns | Relay sees who connects and when, but not message content. |
| No forward secrecy for chat | ECDH key is derived from static long-term keys. Compromise of private key exposes past chats. |
| GitHub Actions as operator trust anchor | If a pool repo's Actions secrets are compromised, all comments can be forged. |
| No channel binding (yet) | Relay-A signature could be replayed at Relay-B. TLS mitigates but does not eliminate. |
| Timing correlation (without history squashing) | New `.bin` commit + new `index.db` entry can be correlated by timestamp. Mitigated by periodic history squashing. |
| Stolen private key = full user compromise | Attacker can find bin_hash, decrypt comments, impersonate on relay. Same model as SSH/PGP. Recovery: re-register. |
| Peer pubkey loading in chat [TODO] | `internal/cli/chat.go` peer pubkey loading is not yet signed/verified (marked as future work). |

### Trust boundaries summary

```
┌─────────────────────────────────────────────────────────────────┐
│  Client (~/.dating/)                                            │
│  • Private key (never leaves this boundary)                     │
│  • Decrypted profile                                            │
│  • Decrypted match notifications                                │
└────────────────┬────────────────────────────────────────────────┘
                 │ ed25519 signatures, encrypted blobs
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│  GitHub (public storage)                                        │
│  • .bin files (pubkey + encrypted profile)                      │
│  • Interest PRs (match_hash in title, encrypted body)           │
│  • Issue comments (signed+encrypted registration results)       │
│  • index.pack (encrypted index records)                         │
│  • matches/*.json (pair_hash lookup for relay auth)             │
└────────────────┬────────────────────────────────────────────────┘
                 │ ciphertext only
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│  Relay (per-pool, stateless)                                    │
│  • Routes binary frames by match_hash                           │
│  • Holds salt (for hash chain validation at WS connect)         │
│  • Caches pubkeys from GitHub (5 min TTL)                       │
│  • Never sees plaintext                                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 11. Payload Size Limits **[planned]**

All message payloads must be size-limited to prevent abuse:

| Channel | Limit | Enforcement Point |
|---------|-------|--------------------|
| Issue body (registration/interest) | 64 KB | `message.Format` — reject content exceeding limit |
| Issue comments (signed blobs) | 64 KB | `message.Format` — reject content exceeding limit |
| Relay binary frames (chat) | 16 KB | Relay `session.go` — drop oversized frames |
| `.bin` file (encrypted profile) | 256 KB | `regcrypt register` — reject oversized blobs |

Enforcement should happen at the `message` package level (`message.Format` returns error if content exceeds max) and at the relay level (session drops frames > 16 KB). This prevents:

- Attackers flooding issues with massive payloads
- Oversized chat messages consuming relay memory (especially offline queue: 20 messages × 16 KB = 320 KB max per user)
- Git repo bloat from oversized `.bin` files

---

## Appendix: Key Files

| File | Relevance |
|------|-----------|
| `internal/crypto/totp.go` | TOTP sign/verify implementation |
| `internal/crypto/conversation.go` | ECDH key derivation, NaCl secretbox |
| `internal/crypto/` | All crypto primitives |
| `internal/relay/server.go` | TOTP auth at WS upgrade, hash chain validation |
| `internal/relay/github_cache.go` | Pubkey fetching and caching |
| `internal/github/pool.go` | Registration polling, comment verification |
| `internal/cli/tui/screens/matches.go` | Match notification decryption |
| `cmd/regcrypt/main.go` | Hash computation + comment signing for Actions |
| `cmd/matchcrypt/main.go` | Match notification generation + signing |
| `templates/actions/pool-register.yml` | Registration Action (sign + post) |
| `templates/actions/pool-interest.yml` | Interest/match Action (sign + post) |
| `docs/concepts/relay-auth-security.md` | Relay auth security analysis |
| `docs/superpowers/specs/2026-03-20-operator-signed-comments.md` | Operator-signed comments spec |
