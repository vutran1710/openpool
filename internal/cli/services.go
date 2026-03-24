package cli

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/cli/svc"
	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/gitclient"
)

// normalizeRepo extracts "owner/repo" from git URL formats.
func normalizeRepo(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	if strings.HasPrefix(s, "git@") {
		if idx := strings.Index(s, ":"); idx != -1 {
			return s[idx+1:]
		}
	}
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}
	return s
}

// DefaultServices creates real service implementations.
func DefaultServices() *svc.Services {
	return &svc.Services{
		Config: &realConfig{},
		Crypto: &realCrypto{},
		Git:    &realGit{},
		GitHub: &realGitHub{client: &http.Client{Timeout: 30 * time.Second}},
	}
}

// --- Config ---

type realConfig struct{}

func (r *realConfig) Load() (*config.Config, error) { return config.Load() }
func (r *realConfig) Save(cfg *config.Config) error  { return cfg.Save() }
func (r *realConfig) Dir() string                    { return config.Dir() }
func (r *realConfig) KeysDir() string                { return config.KeysDir() }
func (r *realConfig) ProfilePath() string            { return config.ProfilePath() }

// --- Crypto ---

type realCrypto struct{}

func (r *realCrypto) GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return crypto.GenerateKeyPair(dir)
}
func (r *realCrypto) LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return crypto.LoadKeyPair(dir)
}
func (r *realCrypto) Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(pub, plaintext)
}
func (r *realCrypto) Decrypt(priv ed25519.PrivateKey, ciphertext []byte) ([]byte, error) {
	return crypto.Decrypt(priv, ciphertext)
}
func (r *realCrypto) PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return crypto.PackUserBin(userPub, operatorPub, plaintext)
}
func (r *realCrypto) Sign(priv ed25519.PrivateKey, message []byte) string {
	return crypto.Sign(priv, message)
}

// --- Git ---

type realGit struct{}

func (r *realGit) Clone(repoURL string) (*gitclient.Repo, error)         { return gitclient.Clone(repoURL) }
func (r *realGit) CloneRegistry(repoURL string) (*gitclient.Repo, error) { return gitclient.CloneRegistry(repoURL) }
func (r *realGit) EnsureGitURL(input string) string                    { return gitclient.EnsureGitURL(input) }
func (r *realGit) FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error) {
	return gitclient.FetchRaw(ctx, repoRef, branch, path)
}
func (r *realGit) FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool {
	return gitclient.FileExistsRaw(ctx, repoRef, branch, path)
}

// --- GitHub ---

type realGitHub struct {
	client *http.Client
}

func (r *realGitHub) GetUser(ctx context.Context, token string) (*svc.GitHubUser, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	json.NewDecoder(resp.Body).Decode(&data)
	name := data.Name
	if name == "" {
		name = data.Login
	}
	return &svc.GitHubUser{
		UserID:      fmt.Sprintf("%d", data.ID),
		Username:    data.Login,
		DisplayName: name,
		Token:       token,
	}, nil
}

func (r *realGitHub) ResolveToken(promptFn func(string) string) (string, error) {
	return resolveGitHubToken(promptFn)
}

func (r *realGitHub) CreateIssue(ctx context.Context, repo, token, title, body string, labels []string) (int, error) {
	payload := map[string]any{"title": title, "body": body}
	if len(labels) > 0 {
		payload["labels"] = labels
	}
	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues", normalizeRepo(repo))
	req, _ := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("create issue failed (%d): %s", resp.StatusCode, body)
	}
	var result struct{ Number int `json:"number"` }
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Number, nil
}

func (r *realGitHub) GetIssue(ctx context.Context, repo, token string, number int) (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d", normalizeRepo(repo), number)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("get issue failed (%d)", resp.StatusCode)
	}
	var issue struct {
		State       string `json:"state"`
		StateReason string `json:"state_reason"`
	}
	json.NewDecoder(resp.Body).Decode(&issue)
	return issue.State, issue.StateReason, nil
}

func (r *realGitHub) StarRepo(ctx context.Context, repo, token string) error {
	url := fmt.Sprintf("https://api.github.com/user/starred/%s", normalizeRepo(repo))
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (r *realGitHub) CreateRepo(ctx context.Context, token, name string, private bool) error {
	body := fmt.Sprintf(`{"name":"%s","private":%t}`, name, private)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/user/repos", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("create repo failed (%d)", resp.StatusCode)
	}
	return nil
}

func (r *realGitHub) CommitFile(ctx context.Context, token, repo, path, message string, content []byte) error {
	encoded := base64.StdEncoding.EncodeToString(content)
	body := fmt.Sprintf(`{"message":"%s","content":"%s"}`, message, encoded)
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", normalizeRepo(repo), path)
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return fmt.Errorf("commit file failed (%d)", resp.StatusCode)
	}
	return nil
}

func (r *realGitHub) RepoExists(ctx context.Context, token, repo string) bool {
	url := fmt.Sprintf("https://api.github.com/repos/%s", normalizeRepo(repo))
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
