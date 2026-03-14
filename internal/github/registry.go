package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const PoolStatusPending = "pending"
const PoolStatusActive = "active"

type Registry struct {
	client *Client
}

type PoolEntry struct {
	Name           string `json:"name"`
	Repo           string `json:"repo"`
	Description    string `json:"description"`
	OperatorPubKey string `json:"operator_public_key"`
	GitHubClientID string `json:"github_client_id,omitempty"`
	GoogleClientID string `json:"google_client_id,omitempty"`
	RelayURL       string `json:"relay_url,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type PoolTokens struct {
	GHToken string `json:"gh_token"`
}

func NewRegistry(repo, token string) *Registry {
	return &Registry{client: NewClient(repo, token)}
}

func NewPublicRegistry(repo string) *Registry {
	return &Registry{client: NewClient(repo, "")}
}

func (r *Registry) Client() *Client {
	return r.client
}

func (r *Registry) ListPools() ([]PoolEntry, error) {
	names, err := r.client.ListDir("pools")
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
	data, err := r.client.GetFile("pools/" + name + "/pool.json")
	if err != nil {
		return nil, fmt.Errorf("reading pool entry: %w", err)
	}

	var entry PoolEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parsing pool entry: %w", err)
	}
	return &entry, nil
}

func (r *Registry) GetPoolTokens(name string) (*PoolTokens, error) {
	data, err := r.client.GetFile("pools/" + name + "/tokens.bin")
	if err != nil {
		return nil, fmt.Errorf("reading pool tokens: %w", err)
	}

	tokens, err := DeserializeTokens(data)
	if err != nil {
		return nil, fmt.Errorf("deserializing tokens: %w", err)
	}
	return tokens, nil
}

func (r *Registry) RegisterPool(entry PoolEntry, tokens PoolTokens, templateBody string) (int, error) {
	entryJSON, _ := json.MarshalIndent(entry, "", "  ")
	tokensBin := SerializeTokens(tokens)

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

	return r.client.CreatePullRequest(pr)
}

func (r *Registry) IsPoolRegistered(name string) bool {
	return r.client.FileExists(fmt.Sprintf("pools/%s/pool.json", name))
}

func SerializeTokens(tokens PoolTokens) []byte {
	data, _ := json.Marshal(tokens)
	encoded := base64.StdEncoding.EncodeToString(data)
	return []byte(encoded)
}

func DeserializeTokens(data []byte) (*PoolTokens, error) {
	cleaned := strings.TrimSpace(string(data))
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	var tokens PoolTokens
	if err := json.Unmarshal(decoded, &tokens); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &tokens, nil
}
