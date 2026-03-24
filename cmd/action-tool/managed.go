package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/gitclient"
	"github.com/vutran1710/openpool/internal/schema"
)

func cmdManagedRegister() {
	fs := flag.NewFlagSet("managed-register", flag.ExitOnError)
	provider := fs.String("provider", "", "identity provider (e.g. google, email, managed)")
	userid := fs.String("userid", "", "user identifier within the provider")
	profilePath := fs.String("profile", "", "path to JSON profile file")
	pool := fs.String("pool", "", "pool repo (owner/repo)")
	schemaPath := fs.String("schema", "pool.yaml", "path to pool.yaml")
	outputDir := fs.String("output-dir", "", "directory to write OPENPOOL_HOME bundle")
	fs.Parse(os.Args[2:])

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
	repo, err := gitclient.Clone(gitclient.EnsureGitURL(*pool))
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
		parts := strings.Split(*pool, "/")
		poolName = parts[len(parts)-1]
	}
	relayURL := s.RelayURL
	opPubHex := s.OperatorPublicKey
	if opPubHex == "" {
		opPubHex = hex.EncodeToString(opPub)
	}

	// 7. Write bundle
	poolDir := filepath.Join(*outputDir, "pools", poolName)
	os.MkdirAll(poolDir, 0700)
	profilePretty, _ := json.MarshalIndent(profile, "", "  ")
	os.WriteFile(filepath.Join(poolDir, "profile.json"), profilePretty, 0600)

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
	fmt.Printf("  OPENPOOL_HOME=%s dating\n", *outputDir)
}
