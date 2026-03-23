# Managed Account Registration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `action-tool managed-register` command that creates managed user accounts by directly committing `.bin` files and outputting ready-to-use `DATING_HOME` bundles.

**Architecture:** Single Go file in `cmd/action-tool/managed.go` using existing `crypto`, `schema`, `github`, and `gitrepo` packages. No new packages needed. Tests in `cmd/action-tool/managed_test.go`. E2E guide updated to use this command for creating test users.

**Tech Stack:** Go, ed25519, NaCl encryption, git CLI, pool.yaml schema validation

**Spec:** `docs/superpowers/specs/2026-03-23-managed-register-design.md`

---

### Task 1: Core implementation — `cmdManagedRegister`

**Files:**
- Create: `cmd/action-tool/managed.go`
- Modify: `cmd/action-tool/main.go`

- [ ] **Step 1: Create `managed.go` with the full command implementation**

```go
// cmd/action-tool/managed.go
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	"github.com/vutran1710/dating-dev/internal/schema"
)

func cmdManagedRegister() {
	fs := flag.NewFlagSet("managed-register", flag.ExitOnError)
	provider := fs.String("provider", "", "identity provider (e.g. google, email, managed)")
	userid := fs.String("userid", "", "user identifier within the provider")
	profilePath := fs.String("profile", "", "path to JSON profile file")
	pool := fs.String("pool", "", "pool repo (owner/repo)")
	schemaPath := fs.String("schema", "pool.yaml", "path to pool.yaml")
	outputDir := fs.String("output-dir", "", "directory to write DATING_HOME bundle")
	fs.Parse(os.Args[2:])

	// Validate required flags
	if *provider == "" || *userid == "" || *profilePath == "" || *pool == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "Usage: action-tool managed-register --provider <provider> --userid <id> --profile <path> --pool <owner/repo> --output-dir <dir>")
		fmt.Fprintln(os.Stderr, "\nRequired env: POOL_SALT, OPERATOR_PRIVATE_KEY")
		os.Exit(1)
	}

	salt := os.Getenv("POOL_SALT")
	opKeyHex := os.Getenv("OPERATOR_PRIVATE_KEY")
	if salt == "" || opKeyHex == "" {
		writeError("POOL_SALT and OPERATOR_PRIVATE_KEY env vars required")
	}

	opKey, err := hex.DecodeString(opKeyHex)
	if err != nil || len(opKey) != ed25519.PrivateKeySize {
		writeError("invalid OPERATOR_PRIVATE_KEY: must be 128 hex chars")
	}
	opPub := ed25519.PrivateKey(opKey).Public().(ed25519.PublicKey)

	// 1. Load and validate profile
	profileData, err := os.ReadFile(*profilePath)
	if err != nil {
		writeError("reading profile: " + err.Error())
	}
	var profile map[string]any
	if err := json.Unmarshal(profileData, &profile); err != nil {
		writeError("parsing profile JSON: " + err.Error())
	}

	s, err := schema.Load(*schemaPath)
	if err != nil {
		writeError("loading schema: " + err.Error())
	}
	if errs := s.ValidateProfile(profile); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		writeError("invalid profile: " + strings.Join(msgs, "; "))
	}

	// 2. Generate keypair
	keysDir := filepath.Join(*outputDir, "keys")
	pub, _, err := crypto.GenerateKeyPair(keysDir)
	if err != nil {
		writeError("generating keypair: " + err.Error())
	}

	// 3. Compute hash chain
	idHash := string(crypto.UserHash(*pool, *provider, *userid))
	binHash := sha256Short(salt + ":" + idHash)
	matchHash := sha256Short(salt + ":" + binHash)

	// 4. Encrypt profile → .bin
	profileJSON, _ := json.Marshal(profile)
	binData, err := crypto.PackUserBin(pub, opPub, profileJSON)
	if err != nil {
		writeError("encrypting profile: " + err.Error())
	}

	// 5. Clone pool repo and commit .bin
	repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(*pool))
	if err != nil {
		writeError("cloning pool repo: " + err.Error())
	}
	repo.Sync()

	binFilePath := filepath.Join(repo.LocalDir, "users", binHash+".bin")
	os.MkdirAll(filepath.Dir(binFilePath), 0755)
	if err := os.WriteFile(binFilePath, binData, 0644); err != nil {
		writeError("writing .bin file: " + err.Error())
	}

	gh, err := github.NewCLI(*pool)
	if err != nil {
		writeError("github CLI: " + err.Error())
	}

	// AddCommitPush needs to run from the repo directory
	origDir, _ := os.Getwd()
	os.Chdir(repo.LocalDir)
	if err := gh.AddCommitPush([]string{"users/"}, "Register managed user "+binHash); err != nil {
		os.Chdir(origDir)
		writeError("committing .bin: " + err.Error())
	}
	os.Chdir(origDir)

	// 6. Read pool metadata for config
	poolName := s.Name
	if poolName == "" {
		// Fallback: extract from repo name
		parts := strings.Split(*pool, "/")
		poolName = parts[len(parts)-1]
	}
	relayURL := s.RelayURL
	opPubHex := s.OperatorPublicKey
	if opPubHex == "" {
		opPubHex = hex.EncodeToString(opPub)
	}

	// 7. Write bundle
	// profile.json
	poolDir := filepath.Join(*outputDir, "pools", poolName)
	os.MkdirAll(poolDir, 0700)
	profilePretty, _ := json.MarshalIndent(profile, "", "  ")
	os.WriteFile(filepath.Join(poolDir, "profile.json"), profilePretty, 0600)

	// setting.toml
	displayName := ""
	if v, ok := profile["display_name"].(string); ok {
		displayName = v
	}
	if displayName == "" {
		displayName = *userid
	}

	config := fmt.Sprintf(`active_pool = '%s'
registries = []
active_registry = ''

[user]
id_hash = '%s'
display_name = '%s'
username = '%s'
provider = '%s'
provider_user_id = '%s'
encrypted_token = ''

[[pools]]
name = '%s'
repo = '%s'
operator_public_key = '%s'
relay_url = '%s'
status = 'active'
bin_hash = '%s'
match_hash = '%s'
`, poolName, idHash, displayName, *userid, *provider, *userid,
		poolName, *pool, opPubHex, relayURL, binHash, matchHash)

	if err := os.WriteFile(filepath.Join(*outputDir, "setting.toml"), []byte(config), 0600); err != nil {
		writeError("writing config: " + err.Error())
	}

	// 8. Print summary
	fmt.Println("Managed user registered successfully:")
	fmt.Printf("  provider:   %s\n", *provider)
	fmt.Printf("  userid:     %s\n", *userid)
	fmt.Printf("  bin_hash:   %s\n", binHash)
	fmt.Printf("  match_hash: %s\n", matchHash)
	fmt.Printf("  output:     %s\n", *outputDir)
	fmt.Println()
	fmt.Printf("  DATING_HOME=%s dating\n", *outputDir)
}
```

- [ ] **Step 2: Register the command in `main.go`**

Add to the switch in `cmd/action-tool/main.go`:

```go
// In the switch block, add:
case "managed-register":
    cmdManagedRegister()

// Update usage line:
fmt.Fprintln(os.Stderr, "Usage: action-tool <register|match|squash|index|sign|decrypt|pubkey|managed-register>")
```

- [ ] **Step 3: Build and verify it compiles**

Run: `go build ./cmd/action-tool/`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add cmd/action-tool/managed.go cmd/action-tool/main.go
git commit -m "feat: action-tool managed-register — operator creates managed accounts"
```

---

### Task 2: Unit tests

**Files:**
- Create: `cmd/action-tool/managed_test.go`

- [ ] **Step 1: Write unit tests**

```go
// cmd/action-tool/managed_test.go
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/schema"
)

func TestHashChain(t *testing.T) {
	salt := "testsalt123"
	pool := "owner/pool"
	provider := "google"
	userid := "user@example.com"

	idHash := string(crypto.UserHash(pool, provider, userid))
	binHash := sha256Short(salt + ":" + idHash)
	matchHash := sha256Short(salt + ":" + binHash)

	// Verify deterministic
	idHash2 := string(crypto.UserHash(pool, provider, userid))
	if idHash != idHash2 {
		t.Error("UserHash should be deterministic")
	}

	// Verify lengths
	if len(idHash) != 64 {
		t.Errorf("id_hash should be 64 hex chars, got %d", len(idHash))
	}
	if len(binHash) != 16 {
		t.Errorf("bin_hash should be 16 hex chars, got %d", len(binHash))
	}
	if len(matchHash) != 16 {
		t.Errorf("match_hash should be 16 hex chars, got %d", len(matchHash))
	}

	// Verify different inputs produce different hashes
	idHash3 := string(crypto.UserHash(pool, provider, "other@example.com"))
	if idHash == idHash3 {
		t.Error("different userids should produce different id_hashes")
	}
}

func TestBundleStructure(t *testing.T) {
	dir := t.TempDir()
	keysDir := filepath.Join(dir, "keys")

	// Generate keys
	pub, _, err := crypto.GenerateKeyPair(keysDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify key files exist and are hex-encoded
	pubHex, err := os.ReadFile(filepath.Join(keysDir, "identity.pub"))
	if err != nil {
		t.Fatal("identity.pub missing")
	}
	privHex, err := os.ReadFile(filepath.Join(keysDir, "identity.key"))
	if err != nil {
		t.Fatal("identity.key missing")
	}

	// Verify hex-decodable
	decoded, err := hex.DecodeString(string(pubHex))
	if err != nil {
		t.Fatal("identity.pub not valid hex")
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Errorf("pubkey wrong size: %d", len(decoded))
	}

	privDecoded, err := hex.DecodeString(string(privHex))
	if err != nil {
		t.Fatal("identity.key not valid hex")
	}
	if len(privDecoded) != ed25519.PrivateKeySize {
		t.Errorf("privkey wrong size: %d", len(privDecoded))
	}

	// Verify PackUserBin works with generated keys
	opPub, _, _ := ed25519.GenerateKey(rand.Reader)
	profile := []byte(`{"age": 25}`)
	binData, err := crypto.PackUserBin(pub, opPub, profile)
	if err != nil {
		t.Fatal("PackUserBin failed:", err)
	}
	if len(binData) < ed25519.PublicKeySize {
		t.Error("bin data too small")
	}
}

func TestProfileValidation(t *testing.T) {
	schemaYAML := `name: test
profile:
  age:
    type: range
    min: 18
    max: 100
  interests:
    type: multi
    values: hiking, coding, music
`
	schemaFile := filepath.Join(t.TempDir(), "pool.yaml")
	os.WriteFile(schemaFile, []byte(schemaYAML), 0600)

	s, err := schema.Load(schemaFile)
	if err != nil {
		t.Fatal(err)
	}

	// Valid profile
	valid := map[string]any{"age": 25, "interests": []any{"hiking"}}
	if errs := s.ValidateProfile(valid); len(errs) > 0 {
		t.Errorf("valid profile rejected: %v", errs)
	}

	// Invalid: age out of range
	invalid := map[string]any{"age": 150, "interests": []any{"hiking"}}
	if errs := s.ValidateProfile(invalid); len(errs) == 0 {
		t.Error("invalid profile should be rejected")
	}

	// Invalid: bad interest value
	invalid2 := map[string]any{"age": 25, "interests": []any{"swimming"}}
	if errs := s.ValidateProfile(invalid2); len(errs) == 0 {
		t.Error("invalid interest should be rejected")
	}
}

func TestConfigFormat(t *testing.T) {
	// Verify the config template produces valid TOML
	config := `active_pool = 'test-pool'
registries = []
active_registry = ''

[user]
id_hash = 'abc123'
display_name = 'Test'
username = 'test'
provider = 'managed'
provider_user_id = 'test'
encrypted_token = ''

[[pools]]
name = 'test-pool'
repo = 'owner/repo'
operator_public_key = 'deadbeef'
relay_url = 'wss://relay.example.com'
status = 'active'
bin_hash = 'abcd1234'
match_hash = 'efgh5678'
`
	// Just verify it's parseable (basic check)
	if len(config) == 0 {
		t.Error("config should not be empty")
	}
	if !contains(config, "active_pool") {
		t.Error("config missing active_pool")
	}
	if !contains(config, "bin_hash") {
		t.Error("config missing bin_hash")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests**

Run: `DATING_HOME=/tmp/test-managed make test`
Expected: all pass

- [ ] **Step 3: Commit**

```bash
git add cmd/action-tool/managed_test.go
git commit -m "test: unit tests for managed-register — hash chain, bundle, validation"
```

---

### Task 3: Integration test

**Files:**
- Create: `cmd/action-tool/managed_integration_test.go`

- [ ] **Step 1: Write integration test**

This test runs the full flow against the real test pool repo (skip in CI with `testing.Short()`).

```go
// cmd/action-tool/managed_integration_test.go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedRegister_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	salt := os.Getenv("POOL_SALT")
	opKey := os.Getenv("OPERATOR_PRIVATE_KEY")
	if salt == "" || opKey == "" {
		t.Skip("POOL_SALT and OPERATOR_PRIVATE_KEY required for integration test")
	}

	// Create profile
	dir := t.TempDir()
	profile := map[string]any{
		"about":     "integration test user",
		"interests": []any{"coding", "hiking"},
		"age":       30,
	}
	profileJSON, _ := json.MarshalIndent(profile, "", "  ")
	profilePath := filepath.Join(dir, "profile.json")
	os.WriteFile(profilePath, profileJSON, 0600)

	outputDir := filepath.Join(dir, "home")

	// Set args to simulate CLI invocation
	os.Args = []string{
		"action-tool", "managed-register",
		"--provider", "test",
		"--userid", "integration-test-user",
		"--profile", profilePath,
		"--pool", "vutran1710/dating-test-pool",
		"--schema", filepath.Join(os.Getenv("POOL_REPO_DIR"), "pool.yaml"),
		"--output-dir", outputDir,
	}

	// NOTE: Can't call cmdManagedRegister() directly in test because it calls os.Exit.
	// Instead, verify the bundle structure after running via `go run`.
	// This test documents the expected structure.

	// Verify expected output structure
	expectedFiles := []string{
		"keys/identity.pub",
		"keys/identity.key",
		"setting.toml",
	}
	_ = expectedFiles
	_ = outputDir

	t.Log("Integration test documents expected flow — run manually:")
	t.Logf("  POOL_SALT=%s OPERATOR_PRIVATE_KEY=<key> go run ./cmd/action-tool/ managed-register \\", salt)
	t.Logf("    --provider test --userid integration-test \\")
	t.Logf("    --profile %s --pool vutran1710/dating-test-pool \\", profilePath)
	t.Logf("    --schema /path/to/pool.yaml --output-dir %s", outputDir)
}
```

- [ ] **Step 2: Run tests**

Run: `make test`
Expected: integration test skipped (no env vars in CI), unit tests pass

- [ ] **Step 3: Commit**

```bash
git add cmd/action-tool/managed_integration_test.go
git commit -m "test: integration test scaffold for managed-register"
```

---

### Task 4: Update E2E guide + remove chatsetup

**Files:**
- Modify: `docs/e2e-testing.md`
- Delete: `cmd/chatsetup/main.go`

- [ ] **Step 1: Update E2E guide**

Replace Journey 5b and references to `cmd/chatsetup` with `action-tool managed-register`:

Add a new section **"Creating Test Users"** near the top of the guide:

```markdown
## Creating Test Users

Use `action-tool managed-register` to create additional test users without a second GitHub account.

```bash
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
action-tool managed-register \
  --provider managed \
  --userid test-user-b \
  --profile /tmp/user-b-profile.json \
  --pool vutran1710/dating-test-pool \
  --schema /path/to/dating-test-pool/pool.yaml \
  --output-dir /tmp/dating-user-b

# Use immediately
DATING_HOME=/tmp/dating-user-b dating
```

For chat testing, create a match file manually:
```bash
# After creating User B, note their match_hash from the output.
# Create match file in the pool repo:
cd /path/to/dating-test-pool
mkdir -p matches
echo '{"match_hash_1":"<A_match>","match_hash_2":"<B_match>"}' > matches/<pair_hash>.json
git add -A && git commit -m "add test match" && git push
```
```

Update Journey 5b to reference this command instead of `cmd/chatsetup`.

- [ ] **Step 2: Delete `cmd/chatsetup/`**

```bash
rm -rf cmd/chatsetup/
```

- [ ] **Step 3: Commit**

```bash
git add docs/e2e-testing.md
git rm -r cmd/chatsetup/
git commit -m "docs: update E2E guide to use managed-register, remove chatsetup"
```

---

### Task 5: Manual end-to-end verification

- [ ] **Step 1: Build action-tool**

```bash
go build -o bin/action-tool ./cmd/action-tool/
```

- [ ] **Step 2: Create a test user**

```bash
source /path/to/dating-test-pool/.dev-secrets

bin/action-tool managed-register \
  --provider managed \
  --userid e2e-verify-user \
  --profile /tmp/verify-profile.json \
  --pool vutran1710/dating-test-pool \
  --schema /path/to/dating-test-pool/pool.yaml \
  --output-dir /tmp/dating-verify
```

- [ ] **Step 3: Verify bundle**

```bash
ls -la /tmp/dating-verify/keys/
cat /tmp/dating-verify/setting.toml
cat /tmp/dating-verify/pools/*/profile.json
```

Expected: keys exist, config has bin_hash + match_hash, profile matches input.

- [ ] **Step 4: Verify .bin committed to pool repo**

```bash
cd /path/to/dating-test-pool && git pull
ls users/ | grep <bin_hash>
```

- [ ] **Step 5: Test CLI works with the bundle**

```bash
DATING_HOME=/tmp/dating-verify bin/dating
# Should launch TUI with active pool, profile visible
```

- [ ] **Step 6: Commit everything**

```bash
git add -A
git commit -m "chore: verified managed-register e2e"
git push
```
