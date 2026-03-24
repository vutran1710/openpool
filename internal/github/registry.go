package github

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

const PoolStatusPending = "pending"
const PoolStatusActive = "active"

// RegistryMeta holds branding metadata for a registry.
type RegistryMeta struct {
	Name    string      `yaml:"name"`
	Tagline string      `yaml:"tagline"`
	Accent  string      `yaml:"accent"` // pink, violet, green, blue, amber
	Version int         `yaml:"version"`
	Pools   []PoolEntry `yaml:"pools"`
}

// PoolEntry is a pool listed in the registry.
type PoolEntry struct {
	Name string `yaml:"name" json:"name"`
	Repo string `yaml:"repo" json:"repo"`

	// Populated from pool.yaml at runtime (not stored in registry)
	Description    string   `yaml:"-" json:"description,omitempty"`
	OperatorPubKey string   `yaml:"-" json:"operator_public_key,omitempty"`
	RelayURL       string   `yaml:"-" json:"relay_url,omitempty"`
	Website        string   `yaml:"-" json:"website,omitempty"`
	About          string   `yaml:"-" json:"about,omitempty"`
	Tags           []string `yaml:"-" json:"tags,omitempty"`
	Operator       string   `yaml:"-" json:"operator,omitempty"`
	CreatedAt      string   `yaml:"-" json:"created_at,omitempty"`
}

type PoolTokens struct {
	GHToken string `json:"gh_token"`
}

type Registry struct {
	client *HTTPClient
	repo   *gitrepo.Repo
	meta   *RegistryMeta
}

// NewRegistry creates a registry with write access (for creating PRs).
func NewRegistry(repoURL, token string) *Registry {
	return &Registry{
		client: NewHTTP(repoURL, token),
	}
}

// NewLocalRegistry creates a registry from a local git clone (read-only).
func NewLocalRegistry(repo *gitrepo.Repo) *Registry {
	r := &Registry{repo: repo}
	r.loadMeta()
	return r
}

// CloneRegistry clones the registry repo (with validation) and returns a read-only Registry.
func CloneRegistry(repoURL string) (*Registry, error) {
	repo, err := gitrepo.CloneRegistry(gitrepo.EnsureGitURL(repoURL))
	if err != nil {
		return nil, fmt.Errorf("cloning registry: %w", err)
	}
	r := &Registry{repo: repo}
	r.loadMeta()
	return r, nil
}

func (r *Registry) loadMeta() {
	if r.repo == nil {
		return
	}
	data, err := r.repo.ReadFile("registry.yaml")
	if err != nil {
		return
	}
	var meta RegistryMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return
	}
	r.meta = &meta
}

// Meta returns the registry metadata (name, tagline, accent).
func (r *Registry) Meta() *RegistryMeta {
	return r.meta
}

func (r *Registry) Client() *HTTPClient {
	return r.client
}

// ListPools returns pool entries from registry.yaml.
// Falls back to reading pools/<name>/pool.json for legacy registries.
func (r *Registry) ListPools() ([]PoolEntry, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("no local clone available")
	}

	// New format: read from registry.yaml
	if r.meta != nil && len(r.meta.Pools) > 0 {
		pools := make([]PoolEntry, len(r.meta.Pools))
		copy(pools, r.meta.Pools)
		// Populate operator from repo name if missing
		for i := range pools {
			if pools[i].Operator == "" {
				if parts := splitRepo(pools[i].Repo); len(parts) == 2 {
					pools[i].Operator = parts[0]
				}
			}
		}
		return pools, nil
	}

	// Legacy fallback: read pools/<name>/pool.json
	names, err := r.repo.ListDir("pools")
	if err != nil {
		return nil, fmt.Errorf("listing pools: %w", err)
	}

	var pools []PoolEntry
	for _, name := range names {
		entry, err := r.GetPoolEntry(name)
		if err != nil {
			continue
		}
		pools = append(pools, *entry)
	}
	return pools, nil
}

// GetPoolEntry reads a pool entry from the legacy pools/<name>/pool.json format.
func (r *Registry) GetPoolEntry(name string) (*PoolEntry, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("no local clone available")
	}

	data, err := r.repo.ReadFile("pools/" + name + "/pool.json")
	if err != nil {
		return nil, fmt.Errorf("reading pool entry: %w", err)
	}

	var entry PoolEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parsing pool entry: %w", err)
	}
	return &entry, nil
}

func (r *Registry) GetPoolTokens(name string, operatorPrivKey ed25519.PrivateKey) (*PoolTokens, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("no local clone available")
	}

	data, err := r.repo.ReadFile("pools/" + name + "/tokens.bin")
	if err != nil {
		return nil, fmt.Errorf("reading pool tokens: %w", err)
	}

	tokens, err := DeserializeTokens(data, operatorPrivKey)
	if err != nil {
		return nil, fmt.Errorf("deserializing tokens: %w", err)
	}
	return tokens, nil
}

func (r *Registry) RegisterPool(ctx context.Context, entry PoolEntry, tokens PoolTokens, templateBody string) (int, error) {
	if r.client == nil {
		return 0, fmt.Errorf("no write client available")
	}

	entryJSON, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshaling pool entry: %w", err)
	}
	tokensBin, err := SerializeTokens(tokens, entry.OperatorPubKey)
	if err != nil {
		return 0, fmt.Errorf("encrypting tokens: %w", err)
	}

	body := fmt.Sprintf("Register new pool **%s**\n\nRepo: %s", entry.Name, entry.Repo)
	if templateBody != "" {
		body = templateBody + "\n\n---\n\n" + body
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Register pool: %s", entry.Name),
		Body:   body,
		Branch: fmt.Sprintf("register-pool/%s", entry.Name),
		Files: []PRFile{
			{Path: fmt.Sprintf("pools/%s/pool.json", entry.Name), Content: entryJSON},
			{Path: fmt.Sprintf("pools/%s/tokens.bin", entry.Name), Content: tokensBin},
		},
	}

	return r.client.CreatePullRequest(ctx, pr)
}

func (r *Registry) IsPoolRegistered(name string) bool {
	// Check registry.yaml first
	if r.meta != nil {
		for _, p := range r.meta.Pools {
			if p.Name == name {
				return true
			}
		}
	}
	// Legacy fallback
	if r.repo != nil {
		return r.repo.FileExists(fmt.Sprintf("pools/%s/pool.json", name))
	}
	if r.client != nil {
		return r.client.FileExists(context.Background(), fmt.Sprintf("pools/%s/pool.json", name))
	}
	return false
}

func splitRepo(repo string) []string {
	for i := len(repo) - 1; i >= 0; i-- {
		if repo[i] == '/' {
			return []string{repo[:i], repo[i+1:]}
		}
	}
	return []string{repo}
}

// SerializeTokens encrypts the pool tokens to the operator's public key.
func SerializeTokens(tokens PoolTokens, operatorPubKeyHex string) ([]byte, error) {
	pubBytes, err := hex.DecodeString(operatorPubKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decoding operator pubkey: %w", err)
	}
	plaintext, err := json.Marshal(tokens)
	if err != nil {
		return nil, fmt.Errorf("marshaling tokens: %w", err)
	}
	return crypto.Encrypt(ed25519.PublicKey(pubBytes), plaintext)
}

// DeserializeTokens decrypts pool tokens using the operator's private key.
func DeserializeTokens(data []byte, operatorPrivKey ed25519.PrivateKey) (*PoolTokens, error) {
	plaintext, err := crypto.Decrypt(operatorPrivKey, data)
	if err != nil {
		return nil, fmt.Errorf("decrypting tokens: %w", err)
	}

	var tokens PoolTokens
	if err := json.Unmarshal(plaintext, &tokens); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &tokens, nil
}
