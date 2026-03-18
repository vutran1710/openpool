# Discovery, Interest & Matching

**Status**: Spec

---

## Summary

Users discover profiles via client-side vector similarity (from `index.db`). Interests are expressed as PRs — one PR per like, title is the target's match_hash. A GitHub Action detects mutual interest by searching for reciprocal PRs, commits a match file, posts encrypted comments with each user's counterpart bin_hash + greeting, and notifies the relay. CLI polls its own PR for the encrypted reply.

---

## 1. Discovery (Client-Side)

The client reads `index.db` from the cloned pool repo. No relay needed.

1. Read `index.db` — flat binary: `[match_hash 8B][vector D×f32]` per user
2. Find own vector by own match_hash
3. Compute cosine similarity against all other vectors
4. Group into tiers (top 10%, next 10%, ...)
5. Shuffle within each tier
6. Present in order

The user browses suggested profiles. To see a full profile, the client fetches it from the relay (which decrypts and re-encrypts per-user).

---

## 2. Expressing Interest (Like)

A like is a PR in the pool repo.

### PR Format

```
Title:  <target_match_hash>
Label:  interest
Branch: like/<hash(author_match_hash + target_match_hash)>
Body:   <base64 encrypted payload to operator pubkey>
```

### Encrypted Payload

```json
{
  "author_bin_hash": "abc...",
  "author_match_hash": "def...",
  "greeting": "Hey, I liked your profile!"
}
```

Encrypted with `crypto.Encrypt(operatorPub, msgpack(payload))` — ephemeral NaCl box. Only the operator/Action can decrypt.

### What Outsiders See

- A GitHub user created a PR with a random-looking title
- Can't tell who the target is (match_hash is meaningless without context)
- Can't read the payload (encrypted)
- Can't correlate with other PRs (different branches, different titles)

### CLI

```bash
dating like <match_hash>
```

CLI resolves match_hash from discovery, creates the PR.

---

## 3. Checking Inbox

The client checks if anyone likes them:

```bash
dating inbox
```

CLI searches: `label:interest title:<my_match_hash> state:open`

Returns a count: "You have 3 new interests." No names, no identities — just a count. The user doesn't know who likes them until mutual match.

---

## 4. Mutual Match (Action)

A GitHub Action fires on PR creation (`pull_request: opened`).

### Flow

1. Decrypt current PR body → extract `{author_bin_hash, author_match_hash, greeting}`
2. Search open PRs: `label:interest title:<author_match_hash>` — is anyone targeting the current author?
3. **Not found** → leave PR open. No match yet.
4. **Found** → mutual interest detected:

### On Mutual Match

Given:
- PR A (earlier): Alice targeting Bob. Body contains `{alice_bin_hash, alice_match_hash, alice_greeting}`
- PR B (current): Bob targeting Alice. Body contains `{bob_bin_hash, bob_match_hash, bob_greeting}`

The Action:

1. **Commit match file**: `matches/{hash(sort(alice_match_hash, bob_match_hash))}.bin`
   - Content: encrypted `{alice_match_hash, bob_match_hash, created_at}` to operator

2. **Post encrypted comment on PR A** (Alice's PR):
   - Encrypted to Alice's pubkey: `{matched_bin_hash: bob_bin_hash, greeting: bob_greeting}`
   - Alice polls her PR, decrypts, learns Bob's bin_hash + greeting

3. **Post encrypted comment on PR B** (Bob's PR):
   - Encrypted to Bob's pubkey: `{matched_bin_hash: alice_bin_hash, greeting: alice_greeting}`
   - Bob polls his PR, decrypts, learns Alice's bin_hash + greeting

4. **Close both PRs**

5. **Notify relay**: `POST /match` with `{bin_hash_1: alice_bin, bin_hash_2: bob_bin}`
   - Relay adds match to store, enabling chat between them

### How Action Gets User Pubkeys

The PR body contains `author_bin_hash`. The `.bin` file at `users/{bin_hash}.bin` starts with 32 bytes of the user's ed25519 pubkey. Action reads the `.bin` file to get the pubkey for encryption.

---

## 5. Receiving Match (CLI)

After creating an interest PR, the CLI can poll for a match:

```bash
dating inbox
```

Or the TUI polls in the background.

### Flow

1. List user's open PRs with `label:interest`
2. For each PR, check comments
3. If a comment from `github-actions[bot]` exists → decrypt with user's privkey
4. Extract `{matched_bin_hash, greeting}`
5. Store matched bin_hash locally
6. Display: "You matched! They said: <greeting>"
7. User can now chat: `dating chat <matched_bin_hash>`

Same polling pattern as registration hash delivery — already implemented.

---

## 6. Relay Match Notification

The relay needs a webhook endpoint for the Action to notify about new matches.

### Endpoint

```
POST /match
Content-Type: application/json
Authorization: HMAC-SHA256(pool_salt, request_body)

{
  "bin_hash_1": "abc...",
  "bin_hash_2": "def..."
}
```

The relay verifies the HMAC using its `POOL_SALT` (shared secret with the Action).

### What the Relay Does

1. `Store.AddMatch(bin1, bin2)`
2. If either user is online, push a notification frame (new protocol frame type)

---

## 7. Privacy Summary

| What | Who can see |
|------|------------|
| PR exists with a hash title | Public (GitHub) |
| PR author (GitHub username) | Public |
| Who the PR targets | Only operator (encrypted body) |
| Greeting message | Only operator + recipient (after match) |
| Match pair | Only operator (encrypted match file) |
| That two users matched | Only the two users + operator |
| bin_hash of match partner | Only the recipient (encrypted PR comment) |

The weakest link: PR author is a real GitHub user. But all anyone knows is "this user expressed interest in *someone*" — not who.

---

## 8. File Summary

| File | Location | Purpose |
|------|----------|---------|
| `index.db` | Pool repo | Public vector index for client-side discovery |
| `matches/{pair_hash}.bin` | Pool repo | Encrypted match record |
| Interest PRs | Pool repo (GitHub PRs) | Open = pending interest, closed = matched or withdrawn |

### Match File

```
Filename: matches/{hash(sort(match_hash_1, match_hash_2))}.bin
Content:  encrypted to operator: {match_hash_1, match_hash_2, created_at}
```

---

## 9. Action Workflow

```yaml
on:
  pull_request:
    types: [opened]

jobs:
  process-interest:
    if: contains(github.event.pull_request.labels.*.name, 'interest')
    steps:
      - checkout
      - download regcrypt/indexer tools
      - decrypt PR body
      - search for reciprocal PR (label:interest title:<author_match_hash>)
      - if not found: exit (leave PR open)
      - if found:
          - decrypt reciprocal PR body
          - read both users' pubkeys from .bin files
          - commit match file
          - post encrypted comment on both PRs
          - close both PRs
          - notify relay POST /match
```
