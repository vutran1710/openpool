# Join Pool — Implementation Plan

## Overview

User selects a pool from the TUI, builds a dating profile from multiple sources, encrypts it, and submits it as a GitHub Issue. A GitHub Action processes the issue and commits the encrypted profile to the pool repo. After confirmation, the CLI stars the repo and fetches the user's hash from the relay.

---

## 1. DatingProfile Struct

**File:** `internal/github/profile.go`

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

Three sources, user selects which to include:

### Source 1: GitHub API

```
GET /user (with token)
→ display_name, bio, location, avatar_url, blog (website), twitter_username (social)
```

### Source 2: Identity Repo README

```
{username}/{username}/README.md (via git clone)
→ showcase (base64-encode the raw markdown)
```

### Source 3: Dating Repo README

```
{username}/dating/README.md (via git clone)
→ Parse YAML frontmatter: interests, looking_for, about (body after frontmatter)
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

```
Select profile sources:

  [✓] GitHub Profile     name, bio, location, avatar
  [✓] Identity Repo      vutran1710/vutran1710 → showcase
  [ ] Dating Repo         vutran1710/dating → interests, looking_for, about
```

### Field Toggle TUI

After fetching, user toggles individual fields on/off:

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

Only checked fields are included in the encrypted profile.

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

**File:** `internal/github/registration.go`

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
    description: "How you want to be known"
  - id: age_range
    label: "Age Range"
    type: select
    required: true
    options: ["18-25", "26-35", "36-45", "46+"]
  - id: vibe
    label: "Your Vibe"
    type: multiselect
    options: ["chill", "adventurous", "nerdy", "creative", "sporty"]
  - id: rules_accepted
    label: "I agree to the pool rules"
    type: checkbox
    required: true
  - id: intro
    label: "Quick intro"
    type: textarea
    required: false
```

### Template Field Types

| Type | TUI Component |
|---|---|
| `text` | textinput |
| `textarea` | multi-line textinput |
| `select` | single-select menu |
| `multiselect` | checkbox list |
| `checkbox` | single checkbox |

### CLI Flow

1. Clone pool repo (already cloned from pools screen)
2. Read `.github/registration.yml` — if missing, skip (default title + labels)
3. Render fields as TUI form
4. Validate required fields
5. Build issue body: rendered template fields + `<!-- blob:{hex} -->`

---

## 6. Submit Registration

**File:** `internal/cli/tui/screens/join.go`

1. Decrypt GitHub token from config (`encrypted_token` → user's private key)
2. Create GitHub Issue on pool repo via API:
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
| closed + completed | Registered | → post-registration steps |
| closed + not_planned | Rejected | show error |

Timeout after 2 minutes → save as pending, check later.

---

## 8. Post-Registration (Issue Closed as Completed)

### Step 1: Star the Pool Repo

```
PUT /user/starred/{pool_repo} (via GitHub API)
```

### Step 2: Get User Hash from Relay

New relay endpoint:

```
POST /identity
{
  "pool_repo": "vutran1710/dating-test-pool",
  "pub_key": "hex-encoded ed25519 pubkey",
  "signature": "sign('identity:' + pub_key_hex)"
}
→ { "user_hash": "fef9b374b0d6f4ad" }
```

Relay flow:
1. Verify signature against pubkey (prove ownership)
2. Compute `SHA256(pool_salt:pool_repo:github:user_id)[:16]` using its salt
3. Return hash

### Step 3: Save to Local Config

```toml
[[pools]]
name = "test-pool"
repo = "vutran1710/dating-test-pool"
operator_public_key = "c251e2cf..."
relay_url = "ws://localhost:8081"
status = "active"
user_hash = "fef9b374b0d6f4ad"
```

`user_hash` used for: discovery, chat, likes, profile lookup.

---

## 9. GitHub Action

**File:** `dating-test-pool/.github/workflows/register.yml`

### Pool Secrets Required

- `OPERATOR_PRIVATE_KEY` — for decrypting profiles
- `POOL_SALT` — for computing filenames

### User Hash Computation

```
SHA256(pool_salt:pool_repo:github:issue_author_id)[:16]
```

Computed by Action from:
- `POOL_SALT` — GitHub secret
- `pool_repo` — `github.repository`
- `github:issue_author_id` — `github.event.issue.user.id`

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

    # Compute filename
    REPO="${{ github.repository }}"
    HASH=$(echo -n "${POOL_SALT}:${REPO}:github:${ISSUE_AUTHOR_ID}" | sha256sum | cut -c1-16)

    # Write .bin
    mkdir -p users
    echo "$BLOB_HEX" | xxd -r -p > "users/${HASH}.bin"

    # Commit
    git config user.name "dating-bot"
    git config user.email "bot@dating-pool.dev"
    git add "users/${HASH}.bin"
    git commit -m "Register user ${HASH}"
    git push
```

---

## 10. Pool Status in Config & TUI

### Config

```toml
[[pools]]
status = "pending"    # "active" | "pending" | "rejected"
user_hash = ""        # populated after registration confirmed
```

### Pool List TUI Indicators

```
  ✓ test-pool        active
  ⠋ berlin-singles   pending
  ✗ tokyo-devs       rejected
```

### Auto-promote

`dating pool list` checks pending pools — if relay returns a user_hash, promote to active.

---

## 11. Local Profile Storage

Save the final DatingProfile locally:

**File:** `~/.dating/profile.json`

Archived by `dating reset` along with keys and config.

---

## 12. Service Interfaces

External services defined as interfaces for testability and modularity.

### GitHub API Interface

**File:** `internal/github/interfaces.go`

```go
type GitHubAPI interface {
    GetUser(ctx context.Context, token string) (*UserInfo, error)
    CreateIssue(ctx context.Context, repo, token, title, body string, labels []string) (int, error)
    GetIssue(ctx context.Context, repo, token string, number int) (*Issue, error)
    StarRepo(ctx context.Context, repo, token string) error
    CreateRepo(ctx context.Context, token, name string, private bool) error
    CommitFile(ctx context.Context, repo, token, path, message string, content []byte) error
    GetFileContent(ctx context.Context, repo, token, path string) ([]byte, error)
}
```

### Relay API Interface

**File:** `internal/relay/interfaces.go`

```go
type RelayAPI interface {
    GetIdentity(ctx context.Context, relayURL, poolRepo, pubKeyHex, signature string) (string, error)
    Discover(ctx context.Context, relayURL string, req DiscoverRequest) (*DiscoverResponse, error)
}
```

### Git Operations Interface

**File:** `internal/gitrepo/interfaces.go`

```go
type GitOps interface {
    Clone(repoURL string) (*Repo, error)
    CloneRegistry(repoURL string) (*Repo, error)
    ReadFile(repo *Repo, path string) ([]byte, error)
    ListDir(repo *Repo, path string) ([]string, error)
    FileExists(repo *Repo, path string) bool
}
```

### Crypto Interface

**File:** `internal/crypto/interfaces.go`

```go
type CryptoOps interface {
    GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
    LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
    Encrypt(recipientPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
    Decrypt(privKey ed25519.PrivateKey, ciphertext []byte) ([]byte, error)
    PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
}
```

All TUI screens and CLI commands depend on interfaces, not concrete implementations.

---

## 13. Testing Strategy

### Unit Tests

| File | Tests |
|---|---|
| `internal/github/profile_test.go` | DatingProfile serialization, field filtering, LookingFor validation |
| `internal/github/registration_test.go` | Template YAML parsing, field type validation, required field checking |
| `internal/cli/profile_source_test.go` | GitHub API response parsing, README frontmatter parsing, source merging |
| `internal/cli/dating_repo_test.go` | README generation from prompts, frontmatter formatting |
| `internal/cli/tui/screens/join_test.go` | All state transitions, source select, field toggle, submit, poll |
| `internal/cli/tui/components/checkbox_test.go` | Toggle, multi-select, keyboard navigation |
| `internal/crypto/encrypt_test.go` | Pack/unpack round-trip with DatingProfile |

### Integration Tests

| File | Tests |
|---|---|
| `internal/github/pool_integration_test.go` | Full registration flow with test pool repo |
| `internal/cli/join_integration_test.go` | End-to-end join flow with mock services |

### Mock Implementations

| Mock | Purpose |
|---|---|
| `internal/github/mock_github.go` | In-memory GitHub API (issues, repos, files) |
| `internal/relay/mock_relay.go` | In-memory relay (identity lookup, discover) |
| `internal/gitrepo/mock_gitops.go` | In-memory filesystem (no actual git) |
| `internal/crypto/mock_crypto.go` | Deterministic keys, no-op encryption |

### Test Patterns

- `_test.go` in same package for unit tests
- `_integration_test.go` with build tag `//go:build integration` for network tests
- Inject interfaces via constructor parameters, not globals
- TUI screen tests: construct → send message → assert state + command

---

## 14. Files to Create/Modify

### New Files (16)

| File | Purpose |
|---|---|
| `internal/github/profile.go` | DatingProfile struct, LookingFor enum |
| `internal/github/interfaces.go` | GitHubAPI interface |
| `internal/github/registration.go` | Parse registration.yml template |
| `internal/github/mock_github.go` | Mock GitHub API for tests |
| `internal/relay/interfaces.go` | RelayAPI interface |
| `internal/relay/mock_relay.go` | Mock relay for tests |
| `internal/relay/identity.go` | POST /identity endpoint |
| `internal/gitrepo/interfaces.go` | GitOps interface |
| `internal/gitrepo/mock_gitops.go` | Mock git operations for tests |
| `internal/crypto/interfaces.go` | CryptoOps interface |
| `internal/crypto/mock_crypto.go` | Mock crypto for tests |
| `internal/cli/profile_source.go` | Fetch from GitHub, identity repo, dating repo |
| `internal/cli/dating_repo.go` | Create private dating repo if missing |
| `internal/cli/tui/screens/join.go` | Join pool TUI screen |
| `internal/cli/tui/components/checkbox.go` | Checkbox / multi-select component |
| `~/.dating/profile.json` | Local profile storage (runtime) |

### Modified Files (9)

| File | Change |
|---|---|
| `internal/cli/config/config.go` | Add DecryptToken, UserHash in PoolConfig |
| `internal/cli/tui/app.go` | Wire join screen, PoolJoinMsg → join flow |
| `internal/cli/tui/screens/pools.go` | Enter on unjoined pool → join screen |
| `internal/cli/reset.go` | Archive profile.json |
| `internal/github/pool.go` | Update RegisterUserViaIssue (blob only) |
| `internal/relay/server.go` | Register /identity route |
| `internal/relay/discover.go` | Identity hash computation |
| `dating-test-pool/.github/workflows/register.yml` | Updated Action with POOL_SALT |
| `cmd/seedpool/main.go` | Use new DatingProfile struct |

### Test Files (9)

| File | Coverage |
|---|---|
| `internal/github/profile_test.go` | Profile serialization, filtering |
| `internal/github/registration_test.go` | Template parsing, validation |
| `internal/cli/profile_source_test.go` | Source fetching, merging |
| `internal/cli/dating_repo_test.go` | README generation |
| `internal/cli/tui/screens/join_test.go` | State transitions, happy path |
| `internal/cli/tui/components/checkbox_test.go` | Toggle, navigation |
| `internal/crypto/encrypt_test.go` | Round-trip with DatingProfile |
| `internal/github/pool_integration_test.go` | Full registration (integration) |
| `internal/cli/join_integration_test.go` | End-to-end with mocks |

---

## 15. Implementation Order

1. Service interfaces (github, relay, gitrepo, crypto)
2. DatingProfile struct + LookingFor enum
3. Mock implementations
4. Profile sources (GitHub API, identity repo, dating repo)
5. Dating repo creation flow
6. Registration template parser
7. TUI components (checkbox, multi-select)
8. Join screen (source select → field toggle → template form → submit → poll)
9. Token decryption
10. Post-registration (star repo, relay /identity endpoint, save hash)
11. Updated GitHub Action (with POOL_SALT)
12. Pool status indicators in TUI (pending/active/rejected)
13. Local profile storage
14. Unit tests
15. Integration tests
16. Update seedpool to use new DatingProfile struct
