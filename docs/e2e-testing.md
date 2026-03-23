# E2E Testing Guide

End-to-end testing for the dating platform. Tests cover the full user journey from registration through matching to chat.

## Prerequisites

- `gh` CLI authenticated (`gh auth status`)
- Test pool: `vutran1710/dating-test-pool`
- Test registry: `vutran1710/dating-test-registry`
- Dev secrets: `dating-test-pool/.dev-secrets` (POOL_SALT, OPERATOR_PRIVATE_KEY)
- Relay server running locally (`go run ./cmd/relay/`)

## Environment Setup

```bash
# Load test secrets
source /Users/vutran/Works/terminal-dating/dating-test-pool/.dev-secrets

# Set test pool URL
export POOL_URL=vutran1710/dating-test-pool

# Optional: clean test pool before testing
cd /path/to/dating-test-pool
rm -f users/*.bin
git add -A && git commit -m "clean test users" && git push
```

## Creating Test Users

Use `action-tool managed-register` to create additional test users without a second GitHub account. This directly commits the `.bin` file to the pool repo and outputs a complete `DATING_HOME` bundle.

```bash
# Build action-tool
go build -o bin/action-tool ./cmd/action-tool/

# Create a profile
cat > /tmp/user-b-profile.json << 'EOF'
{
  "about": "Test user B",
  "interests": ["coding", "gaming"],
  "age": 25
}
EOF

# Register managed user
source /path/to/dating-test-pool/.dev-secrets
bin/action-tool managed-register \
  --provider managed \
  --userid test-user-b \
  --profile /tmp/user-b-profile.json \
  --pool vutran1710/dating-test-pool \
  --schema /path/to/dating-test-pool/pool.yaml \
  --output-dir /tmp/dating-user-b

# Use immediately
DATING_HOME=/tmp/dating-user-b dating
```

For chat testing, also create a match file between User A and User B:
```bash
# Note match_hash values from managed-register output and User A's config
# Compute pair_hash = sha256(min(a,b):max(a,b))[:12]
cd /path/to/dating-test-pool
mkdir -p matches
echo '{"match_hash_1":"<A_match>","match_hash_2":"<B_match>"}' > matches/<pair_hash>.json
git add -A && git commit -m "add test match" && git push
```

---

## User Journeys

### Journey 1: First-Time App Setup (App Onboarding)

**What it tests**: GitHub auth, key generation, registry clone, config persistence.

**Steps**:
1. Delete `~/.dating/` to start fresh
2. Run `dating` — should show Welcome screen
3. Press Enter — auto-detects `gh` CLI token
4. Keys are generated automatically
5. Enter registry: `vutran1710/dating-test-registry`
6. Registry clones, pools are discovered
7. Config saved — redirects to Pools screen

**Expected**:
- Timeline shows 4 green checkmarks
- Status bar shows username and registry
- Pools screen lists `test-pool`

**Verify**:
```bash
cat ~/.dating/setting.toml  # should have user, registry, encrypted_token
ls ~/.dating/keys/           # should have ed25519 keypair
```

---

### Journey 2: Pool Join (Pool Onboarding)

**What it tests**: Schema fetch via raw URL, role selection, profile form, registration issue submission, background polling.

**Steps**:
1. From Pools screen, select `test-pool` and press Enter
2. Role selection: choose "man" or "woman"
3. Fill profile fields (about, interests, phone, age)
4. Press Ctrl+D to submit
5. Press Enter on "Profile complete!" screen
6. Wait for GitHub Action to process the registration

**Expected**:
- Pool onboard screen loads instantly (schema via raw URL)
- Role selection with `man` / `woman` options
- Profile form shows all fields from `pool.yaml`
- After submit: toast "Registration submitted (Issue #N)"
- Status changes to "pending" in pools screen
- Background poller detects completion, sets status to "active"
- `bin_hash` and `match_hash` populated in config

**Verify**:
```bash
# Check the registration issue was created
gh issue list --repo $POOL_URL --label registration --state all

# Check the .bin file was committed
ls /path/to/dating-test-pool/users/

# Check config has hashes
grep -A5 'test-pool' ~/.dating/setting.toml
```

**Action log** (check if Action succeeded):
```bash
gh run list --repo $POOL_URL --limit 5
gh run view <run-id> --repo $POOL_URL --log
```

---

### Journey 3: Invalid Registration (Sad Path)

**What it tests**: Schema validation in the registration Action rejects invalid profiles.

**How to test**: Use the E2E test binary or manually create an issue with invalid profile data (e.g., age=150 when max is 100).

```bash
# Run programmatic test
cd dating-dev
go run ./cmd/e2etest/
# Test 2 creates an issue with age=150, expects rejection
```

**Expected**:
- Issue closed as "not_planned"
- Issue locked as "spam"
- No .bin file committed

---

### Journey 4: Interest Matching

**What it tests**: Two users express mutual interest, Action detects the match, posts encrypted notifications with peer pubkeys.

**Steps** (programmatic — use E2E test binary):
1. Create two fake users (A and B) with .bin files
2. A creates interest issue targeting B's match_hash
3. B creates interest issue targeting A's match_hash
4. Action detects mutual interest
5. Both issues closed + locked
6. Match notification comments posted (encrypted with peer pubkeys)

```bash
go run ./cmd/e2etest/
# Test 3 handles the full interest matching flow
```

**Expected**:
- Both interest issues closed + locked
- Match file created in `matches/` directory
- Encrypted match notification comments on both issues

**Verify**:
```bash
gh issue list --repo $POOL_URL --label interest --state closed
```

---

### Journey 5: Relay Chat (Two Users via tmux)

**What it tests**: WebSocket connection, TOTP auth, binary frame routing, E2E encrypted messages between two real users.

**Prerequisites**: Pool relay running (Railway or local), two GitHub accounts.

**Limitation**: Registration is tied to GitHub user ID. Testing two-user chat via TUI requires two distinct GitHub accounts. If you only have one, use the programmatic approach in Journey 5b below.

#### Step 1: Set up two user environments

Each user gets its own `DATING_HOME` with separate keys, config, and profile.

```bash
mkdir -p /tmp/dating-user-a /tmp/dating-user-b
```

#### Step 2: Register User A (GitHub Account 1)

```bash
DATING_HOME=/tmp/dating-user-a dating
```

1. App onboarding: authenticates with GitHub Account 1
2. Join test-pool → select role → fill profile → Ctrl+D to submit
3. Wait ~60s for Action to process registration
4. Open Settings > Identity → note `match_hash` (e.g. `4f2765a51a18d12e`)

#### Step 3: Register User B (GitHub Account 2)

```bash
DATING_HOME=/tmp/dating-user-b dating
```

Same flow as Step 2 but authenticated with a different GitHub account. Note User B's `match_hash`.

#### Step 4: Create mutual interest

In User A's TUI:
1. `/discover` → find User B → press `l` to like

In User B's TUI:
1. `/discover` → find User A → press `l` to like

Wait ~60s for the Action to detect the mutual match. Both interest issues should close + lock.

Alternatively, create interest issues via CLI:
```bash
# A likes B (from User A's environment)
DATING_HOME=/tmp/dating-user-a dating interest <B_match_hash>

# B likes A (from User B's environment)
DATING_HOME=/tmp/dating-user-b dating interest <A_match_hash>
```

#### Step 5: Chat via tmux

Replace `<B_match>` and `<A_match>` with actual match_hash values.

```bash
# Option 1: tmux side-by-side
tmux new-session -d -s chat \
  "DATING_HOME=/tmp/dating-user-a dating chat <B_match>"
tmux split-window -h -t chat \
  "DATING_HOME=/tmp/dating-user-b dating chat <A_match>"
tmux attach -t chat

# Option 2: two terminal tabs
# Tab 1:
DATING_HOME=/tmp/dating-user-a dating chat <B_match>
# Tab 2:
DATING_HOME=/tmp/dating-user-b dating chat <A_match>
```

#### Step 6: Send messages

- In User A's pane: type a message, press Enter
- User B should see the message appear in real-time
- In User B's pane: type a reply, press Enter
- User A should see the reply

**Expected**:
- WebSocket connects with TOTP auth (no login endpoint, no token)
- Messages are E2E encrypted (NaCl secretbox via ECDH)
- Relay routes binary frames by match_hash
- Messages persisted in each user's `conversations.db`
- Messages appear in real-time (no polling)

**Verify**:
```bash
# Relay health
curl -s https://relay-production-0b24.up.railway.app/health
# Should show "online": 2

# User A's messages
sqlite3 /tmp/dating-user-a/conversations.db \
  "SELECT direction, substr(content,1,40), created_at FROM messages ORDER BY created_at DESC LIMIT 5;"

# User B's messages
sqlite3 /tmp/dating-user-b/conversations.db \
  "SELECT direction, substr(content,1,40), created_at FROM messages ORDER BY created_at DESC LIMIT 5;"
```

**Cleanup**:
```bash
rm -rf /tmp/dating-user-a /tmp/dating-user-b
tmux kill-session -t chat 2>/dev/null
```

---

### Journey 5b: Relay Chat (Single GitHub Account — Managed User B)

When only one GitHub account is available, use `action-tool managed-register` to create User B.

#### Step 1: Verify User A is registered

```bash
grep match_hash ~/.dating/setting.toml
# Should show: match_hash = '<A_match>'
```

#### Step 2: Create User B via managed-register

```bash
cat > /tmp/user-b-profile.json << 'EOF'
{"about": "Test user B", "interests": ["coding", "gaming"], "age": 25}
EOF

source /path/to/dating-test-pool/.dev-secrets
bin/action-tool managed-register \
  --provider managed --userid chat-test-b \
  --profile /tmp/user-b-profile.json \
  --pool vutran1710/dating-test-pool \
  --schema /path/to/dating-test-pool/pool.yaml \
  --output-dir /tmp/dating-user-b

# Note User B's match_hash from the output
```

#### Step 3: Create match file

```bash
# Use User A's match_hash and User B's match_hash
# pair_hash = sha256(min(a,b) + ":" + max(a,b))[:12]
cd /path/to/dating-test-pool
mkdir -p matches
echo '{"match_hash_1":"<A_match>","match_hash_2":"<B_match>"}' > matches/<pair_hash>.json
git add -A && git commit -m "add test match" && git push
```

#### Step 4: Chat via tmux

```bash
tmux new-session -d -s chat "dating chat <B_match>"
tmux split-window -h -t chat \
  "DATING_HOME=/tmp/dating-user-b dating chat <A_match>"
tmux attach -t chat
```

#### Step 5: Send messages

Type in each pane — messages should appear in real-time in the other pane.

**Cleanup**:
```bash
rm -rf /tmp/dating-user-b
tmux kill-session -t chat 2>/dev/null
```

---

### Journey 6: Profile View

**What it tests**: Pool-specific profile loading from `schema.ProfilePath()`.

**Steps**:
1. Navigate to Profile (from home menu or `/profile` command)
2. Should show the active pool's profile fields

**Expected**:
- Title shows "Profile" with pool name hint
- All profile fields displayed with title-cased labels
- Private fields show lock emoji
- Array fields (interests) show as `#hiking #coding` tags
- No active pool: shows "No active pool. Join a pool first."

---

### Journey 7: Settings / Identity

**What it tests**: Identity expansion showing pubkey, pool hashes.

**Steps**:
1. Navigate to Settings (`/settings`)
2. Select Identity card, press Enter
3. Expanded view shows pubkey, active pool, hashes

**Expected**:
- Pubkey truncated: `7b30a956b1...ccefce61`
- Pool name, repo, bin_hash, match_hash displayed
- If registration pending: shows "Registration pending"

---

### Journey 8: Discovery (requires index)

**What it tests**: Index download, profile browsing, like flow.

**Prerequisites**: Indexer Action has run (creates `index.db` release asset).

**Steps**:
1. Navigate to Discover (`/discover`)
2. Browse profiles with arrow keys
3. Press `l` to like someone

**Expected**:
- Profiles loaded from `index.db` (downloaded from release asset)
- Card shows name, match %, bio, interests
- Like creates an interest issue

---

## Automated E2E Test

The `cmd/e2etest/` binary tests registration (valid + invalid) and interest matching programmatically:

```bash
export POOL_SALT=<from .dev-secrets>
export POOL_URL=vutran1710/dating-test-pool
export OPERATOR_PRIVATE_KEY=<from .dev-secrets>

go run ./cmd/e2etest/
```

This creates real GitHub issues, waits for Actions to process them, and verifies the results. Takes ~2-3 minutes due to Action processing time.

## Test Pool Cleanup

After testing, clean up the test pool:

```bash
cd /path/to/dating-test-pool

# Remove all user .bin files
rm -f users/*.bin
git add -A && git commit -m "clean test users" && git push

# Reset local config
cat > ~/.dating/setting.toml << 'EOF'
pools = []
active_pool = ''
registries = ['https://github.com/vutran1710/dating-test-registry']
active_registry = 'https://github.com/vutran1710/dating-test-registry'

[user]
id_hash = ''
display_name = ''
username = ''
provider = ''
provider_user_id = ''
encrypted_token = ''
EOF
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "registration pending" forever | Action failed or hasn't run | Check `gh run list --repo $POOL_URL` |
| "Pool missing pool.yaml" | Pool repo not cloned or pool.yaml missing | Verify pool.yaml exists in repo |
| "unstaged changes" in Action | Old action-tool binary | Republish: `gh release upload action-tool-v1.0.0 action-tool-linux-amd64 --clobber --repo vutran1710/regcrypt` |
| Chat won't connect | Relay not running or wrong URL | Check `curl localhost:8081/health` |
| "invalid operator key" | Wrong OPERATOR_PRIVATE_KEY | Check `.dev-secrets` matches pool repo secrets |
| Profile fields jumping | Map iteration order | Fixed — keys are sorted alphabetically |
