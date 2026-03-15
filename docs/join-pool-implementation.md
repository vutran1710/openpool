# Join Pool — Implementation Plan

## Overview

User selects a pool from the TUI, builds a dating profile from multiple sources, encrypts it, and submits it as a GitHub Issue. A GitHub Action processes the issue and commits the encrypted profile to the pool repo.

---

## 1. DatingProfile Struct

**File:** `internal/github/types.go`

```go
type LookingFor = string

const (
    LookingForFriendship   LookingFor = "friendship"
    LookingForDating       LookingFor = "dating"
    LookingForRelationship LookingFor = "relationship"
    LookingForNetworking   LookingFor = "networking"
    LookingForOpen         LookingFor = "open"
)

type DatingProfile struct {
    DisplayName string       `json:"display_name"`
    Bio         string       `json:"bio"`
    Location    string       `json:"location"`
    AvatarURL   string       `json:"avatar_url,omitempty"`
    Website     string       `json:"website,omitempty"`
    Social      []string     `json:"social,omitempty"`
    Showcase    string       `json:"showcase,omitempty"`    // base64-encoded identity README
    Interests   []string     `json:"interests,omitempty"`
    LookingFor  []LookingFor `json:"looking_for,omitempty"`
    About       string       `json:"about,omitempty"`
}
```

Not in the profile: pubkey (in .bin prefix), status (always "open"), email (privacy).

---

## 2. Profile Sources

**File:** `internal/cli/profile_source.go`

### Source 1: GitHub API

```
GET /user (with token)
→ display_name, bio, location, avatar_url, blog (website), twitter_username (social)
```

### Source 2: Identity Repo README

```
GET {username}/{username}/README.md (via git clone)
→ showcase (base64-encode the raw markdown)
```

### Source 3: Dating Repo README

```
GET {username}/dating/README.md (via git clone)
→ Parse YAML frontmatter for: interests, looking_for, about
→ If repo doesn't exist: prompt user, create private repo via API, commit generated README
```

**Dating README format:**
```markdown
---
interests: [rust, hiking, coffee]
looking_for: [dating, friendship]
---

# About

Your about text here...
```

### Source Selection TUI

**File:** `internal/cli/tui/screens/join.go`

```
Select profile sources:

  [✓] GitHub Profile     name, bio, location, avatar
  [✓] Identity Repo      vutran1710/vutran1710 → showcase
  [ ] Dating Repo         vutran1710/dating → interests, looking_for, about
```

Space to toggle. After selection, fetch all checked sources (with spinners).

### Field Toggle TUI

After fetching, show all populated fields. User toggles with space:

```
  [✓] Name          Vu Tran
  [✓] Bio           making softwares for human
  [✓] Location      Vietnam, Hanoi
  [✓] Avatar        https://avatars...
  [ ] Website       me@vutr.io
  [✓] Showcase      (342 chars, from identity README)
  [ ] Interests     (not set — no dating repo)
  [ ] Looking for   (not set)

  space toggle · enter submit
```

Only checked fields are included in the final profile.

---

## 3. Dating Repo Creation

**File:** `internal/cli/dating_repo.go`

When `{username}/dating` doesn't exist:

1. Prompt interests — multi-select from common tags
2. Prompt looking_for — multi-select from enum
3. Prompt about — free text input
4. Generate README.md with YAML frontmatter
5. Create **private** repo `{username}/dating` via GitHub API
6. Commit README.md

---

## 4. Encrypt & Pack

**File:** `internal/crypto/encrypt.go` (existing)

```
1. JSON-serialize DatingProfile (only checked fields)
2. PackUserBin(userPub, operatorPub, profileJSON)
   → [32B user pubkey][NaCl box encrypted profile]
3. Hex-encode the .bin for issue body
```

---

## 5. Registration Template

**File:** `internal/github/template.go`

### Template Schema

Pool repo has `.github/registration.yml`:

```yaml
title: "Registration Request"
labels: ["registration"]
fields:
  - id: display_name
    label: "Display Name"
    type: text
    required: true
  - id: age_range
    label: "Age Range"
    type: select
    required: true
    options: ["18-25", "26-35", "36-45", "46+"]
  - id: vibe
    label: "Your Vibe"
    type: multiselect
    options: ["chill", "adventurous", "nerdy", "creative"]
  - id: rules_accepted
    label: "I agree to pool rules"
    type: checkbox
    required: true
  - id: intro
    label: "Quick intro"
    type: textarea
    required: false
```

### Template Types

| Type | TUI Component |
|---|---|
| `text` | textinput |
| `textarea` | multi-line textinput |
| `select` | single-select menu |
| `multiselect` | checkbox list |
| `checkbox` | single checkbox |

### CLI Flow

1. Clone pool repo (already cloned for pools screen)
2. Read `.github/registration.yml` — if missing, skip (default title + labels)
3. Render fields as TUI form
4. Validate required fields
5. Build issue body: rendered template fields + `<!-- blob:{hex} -->`

---

## 6. Submit Registration

**File:** `internal/cli/tui/screens/join.go`

1. Decrypt GitHub token from config (`encrypted_token` → user's private key)
2. Create GitHub Issue on pool repo:
   - Title: from template (or "Registration Request")
   - Body: template fields + `<!-- blob:{hex_encoded_bin} -->`
   - Labels: from template (or ["registration"])
3. Show issue number

### Token Decryption

**File:** `internal/cli/config/config.go`

```go
func (c *Config) DecryptToken(privKey ed25519.PrivateKey) (string, error) {
    encrypted, _ := hex.DecodeString(c.User.EncryptedToken)
    plaintext, err := crypto.Decrypt(privKey, encrypted)
    return string(plaintext), err
}
```

---

## 7. Poll Issue Status

**File:** `internal/cli/tui/screens/join.go`

After issue creation, poll every 5 seconds:

```
GET /repos/{pool_repo}/issues/{number}
→ state: "open" | "closed"
→ state_reason: "completed" | "not_planned"
```

| State | Meaning | Action |
|---|---|---|
| open | Waiting | spinner: "Processing..." |
| closed + completed | Registered | save pool as active |
| closed + not_planned | Rejected | show error |

Timeout after 2 minutes → save as pending, check later.

---

## 8. Pool Status in Config & TUI

### Config

```toml
[[pools]]
name = "test-pool"
repo = "vutran1710/dating-test-pool"
operator_public_key = "c251e2cf..."
relay_url = "ws://localhost:8081"
status = "pending"    # "active" | "pending" | "rejected"
```

### Pool List TUI

```
  ✓ test-pool        active
  ⠋ berlin-singles   pending
  ✗ tokyo-devs       rejected
```

### Pool List CLI (`dating pool list`)

Check pending pools on each list — if `.bin` exists in pool repo, auto-promote to active.

---

## 9. GitHub Action Update

**File:** `dating-test-pool/.github/workflows/register.yml`

### Pool Secrets Required

- `OPERATOR_PRIVATE_KEY` — for decrypting profiles
- `POOL_SALT` — for computing filenames

### Updated Action

```yaml
- name: Process registration
  env:
    ISSUE_BODY: ${{ github.event.issue.body }}
    ISSUE_NUMBER: ${{ github.event.issue.number }}
    ISSUE_AUTHOR_ID: ${{ github.event.issue.user.id }}
    POOL_SALT: ${{ secrets.POOL_SALT }}
    GH_TOKEN: ${{ github.token }}
  run: |
    # Extract hex blob from issue body
    BLOB_HEX=$(echo "$ISSUE_BODY" | sed -n 's/.*<!-- blob:\(.*\) -->/\1/p')

    # Compute filename: SHA256(salt:repo:github:author_id)[:16]
    REPO="${{ github.repository }}"
    HASH=$(echo -n "${POOL_SALT}:${REPO}:github:${ISSUE_AUTHOR_ID}" | sha256sum | cut -c1-16)

    # Write .bin
    echo "$BLOB_HEX" | xxd -r -p > "users/${HASH}.bin"

    # Commit
    git add "users/${HASH}.bin"
    git commit -m "Register user ${HASH}"
    git push
```

---

## 10. Local Profile Storage

Save the final DatingProfile locally for reference/editing:

**File:** `~/.dating/profile.json`

Archived by `dating reset` along with keys and config.

---

## 11. Files to Create/Modify

### New Files

| File | Purpose |
|---|---|
| `internal/github/profile.go` | DatingProfile struct, LookingFor enum |
| `internal/cli/profile_source.go` | Fetch from GitHub, identity repo, dating repo |
| `internal/cli/dating_repo.go` | Create private dating repo if missing |
| `internal/cli/tui/screens/join.go` | Join pool TUI: source select, field toggle, template form, submit, poll |
| `internal/cli/tui/components/checkbox.go` | Checkbox / multi-select TUI component |
| `internal/github/registration.go` | Parse registration.yml template |

### Modified Files

| File | Change |
|---|---|
| `internal/cli/config/config.go` | Add DecryptToken method |
| `internal/cli/tui/app.go` | Wire join screen, handle PoolJoinMsg |
| `internal/cli/tui/screens/pools.go` | Enter on unjoined pool → join screen |
| `internal/cli/reset.go` | Archive profile.json |
| `internal/github/pool.go` | Update RegisterUserViaIssue (blob only) |
| `.github/workflows/register.yml` | New Action with POOL_SALT |
| `dating-test-pool/.github/workflows/register.yml` | Updated Action |

### Test Files

| File | Coverage |
|---|---|
| `internal/cli/tui/screens/join_test.go` | Join flow state transitions |
| `internal/github/profile_test.go` | Profile struct, serialization |
| `internal/github/registration_test.go` | Template parsing |
| `internal/cli/profile_source_test.go` | Source fetching, merging |

---

## 12. Implementation Order

1. `DatingProfile` struct + `LookingFor` enum
2. Profile sources (GitHub API, identity repo, dating repo)
3. Dating repo creation flow
4. Registration template parser
5. TUI components (checkbox, multi-select)
6. Join screen (source select → field toggle → template form → submit → poll)
7. Token decryption
8. Updated GitHub Action
9. Pool status indicators in TUI
10. Local profile storage
11. Tests
12. Update seedpool to use new DatingProfile struct
