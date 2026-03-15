package github

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

const PoolStatusPending = "pending"
const PoolStatusActive = "active"

type Registry struct {
	client *Client       // for writes (PRs)
	repo   *gitrepo.Repo // for reads (local clone)
}

type PoolEntry struct {
	Name           string   `json:"name"`
	Repo           string   `json:"repo"`
	Description    string   `json:"description"`
	About          string   `json:"about,omitempty"`
	OperatorPubKey string   `json:"operator_public_key"`
	RelayURL       string   `json:"relay_url,omitempty"`
	Website        string   `json:"website,omitempty"`
	CreatedAt      string   `json:"created_at"`
	Tags           []string `json:"tags,omitempty"`
	Operator       string   `json:"operator,omitempty"`
}

type PoolTokens struct {
	GHToken string `json:"gh_token"`
}

// NewRegistry creates a registry with write access (for creating PRs).
func NewRegistry(repoURL, token string) *Registry {
	return &Registry{
		client: NewClient(repoURL, token),
	}
}

// NewLocalRegistry creates a registry from a local git clone (read-only).
func NewLocalRegistry(repo *gitrepo.Repo) *Registry {
	return &Registry{repo: repo}
}

// CloneRegistry clones the registry repo (with validation) and returns a read-only Registry.
func CloneRegistry(repoURL string) (*Registry, error) {
	repo, err := gitrepo.CloneRegistry(gitrepo.EnsureGitURL(repoURL))
	if err != nil {
		return nil, fmt.Errorf("cloning registry: %w", err)
	}
	return &Registry{repo: repo}, nil
}

func (r *Registry) Client() *Client {
	return r.client
}

func (r *Registry) ListPools() ([]PoolEntry, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("no local clone available")
	}

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

	body := fmt.Sprintf("Register new pool **%s**\n\nRepo: %s\nDescription: %s", entry.Name, entry.Repo, entry.Description)
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
	if r.repo != nil {
		return r.repo.FileExists(fmt.Sprintf("pools/%s/pool.json", name))
	}
	if r.client != nil {
		return r.client.FileExists(context.Background(), fmt.Sprintf("pools/%s/pool.json", name))
	}
	return false
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
