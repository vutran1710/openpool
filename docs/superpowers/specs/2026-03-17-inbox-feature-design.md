# Inbox Feature

## Problem

Users can like others (`dating like`) but there's no TUI screen to view and act on incoming likes. The CLI `dating inbox` exists but the TUI shows "coming soon." Additionally, the current like mechanism exposes both users directly via PRs — a bot should mediate.

## Like Flow

### Step 1: Liker Creates Issue

`dating like <hash>` creates a GitHub Issue (not a PR) on the pool repo:

```
Title: "Like: {liker_hash[:8]}"
Labels: ["like"]
Body:
{liker_hash}
{encrypted_message_hex}
{signature}
```

- `encrypted_message_hex`: initial message encrypted to the operator's pubkey (NaCl box, same as profile blobs). Only the relay can decrypt it for the recipient.
- `signature`: ed25519 signature of `{"action":"like","liker_id":"{liker_hash}","liked_id":"{liked_hash}"}` — proves identity.

### Step 2: Pool Action Processes Issue

A GitHub Action on the pool repo:
1. Validates the signature
2. Verifies both users exist in `users/`
3. Computes sorted pair: `hashA_hashB` (alphabetical)
4. Creates a PR with:
   - Branch: `like/{hashA}_{hashB}`
   - Title: `"Like: {liker[:8]} -> {liked[:8]}"`
   - Body: contains `liker_hash`, `liked_hash`, `encrypted_message_hex`, `signature`
   - Labels: `["like:{liked_hash[:12]}"]`
   - Files: `matches/{hashA}_{hashB}.json` containing `{"created_at": <unix_timestamp>}`
5. Closes the Issue with a link to the PR

The bot creates the PR — the two users never interact directly on GitHub.

### Step 3: Inbox Queries Open PRs

`ListIncomingLikes(userHash)` queries open PRs with label `like:{userHash[:12]}`. Returns `[]PullRequest` with PR number, title, body (contains encrypted message + liker hash).

### Step 4: Accept or Reject

- **Accept**: merge the PR → `matches/{hashA}_{hashB}.json` lands in main → relay syncs and enables chat routing
- **Reject**: close the PR without merging → no match created

## Match Storage

```
matches/
  abc123ab_def456de.json    # sorted hashes in filename
  abc123ab_ghi789gh.json
```

File content:
```json
{"created_at": 1710720000}
```

No blob duplication — profiles stay in `users/`. The filename contains both hashes (sorted), so finding a user's matches = list directory, filter filenames containing their hash.

Relay syncs these files during periodic repo sync to populate its in-memory match store.

## Inbox TUI Screen

### Layout

Modest centered panel, ~50-60 chars wide. One like at a time.

```
                    ┌─────────────────────────────────┐
                    │  ♥ Someone likes you    (1/5)   │
                    │                                 │
                    │  Alice                          │
                    │  Berlin · Looking for dating    │
                    │                                 │
                    │  "Hey! I noticed we both like   │
                    │   hiking. Want to chat?"        │
                    │                                 │
                    │  y accept · n reject · esc back │
                    └─────────────────────────────────┘
```

### Behavior

1. On enter: fetch all incoming likes via `ListIncomingLikes`
2. For current like: extract liker hash from PR body, fetch profile from relay (discovery endpoint), decrypt initial message
3. Show spinner while loading
4. User presses `y` → merge PR, toast "It's a match!", advance to next
5. User presses `n` → close PR, advance to next
6. When all processed: "No more incoming interests"
7. Empty state: "No incoming interests yet"
8. Counter in top-right of panel: `(1/5)`

### Profile Data

Fetched from relay via discovery endpoint (relay decrypts `.bin` with operator key, re-encrypts for requester). Shows:
- Display name
- Location + intent
- Initial message (decrypted from PR body by relay)

## File Changes

| File | Changes |
|------|---------|
| `internal/cli/tui/screens/inbox.go` | Create — inbox screen: one-at-a-time like cards, relay profile fetch, accept/reject |
| `internal/cli/tui/app.go` | Wire inbox screen: add `screenInbox`, handle in menu select + `/inbox` command + view/update |
| `internal/github/pool.go` | Add `CreateLikeIssue(ctx, likerHash, likedHash, encryptedMsg, signature)`. Add `RejectLike(ctx, prNumber)` (close PR). Update match file format to `{hashA}_{hashB}.json` with `{"created_at":...}` |
| `internal/cli/like.go` | Update `newLikeCmd` to create Issue (encrypt message to operator, sign payload, call `CreateLikeIssue`). Update `newInboxCmd` CLI output. |
| `internal/github/types.go` | No changes needed — `PullRequest` struct already has what we need |

## Testing

- Inbox screen renders empty state
- Inbox screen shows like card with profile + message
- Accept merges PR, advances to next
- Reject closes PR, advances to next
- Counter updates correctly
- `/inbox` slash command navigates to inbox screen
- `CreateLikeIssue` creates Issue with correct format
- `RejectLike` closes PR
- Match file format: `{hashA}_{hashB}.json` with timestamp
