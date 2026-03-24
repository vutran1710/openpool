package cli

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/cli/svc"
	"github.com/vutran1710/openpool/internal/crypto"
	"github.com/vutran1710/openpool/internal/gitclient"
)

// MockServices creates a fully mocked Services for testing.
func MockServices() *svc.Services {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return &svc.Services{
		Config: &MockConfigService{Cfg: &config.Config{}},
		Crypto: &MockCryptoService{Pub: pub, Priv: priv},
		Git:    &MockGitService{Repos: make(map[string]map[string][]byte)},
		GitHub: &MockGitHubService{Issues: make(map[int]*mockIssue)},
	}
}

// --- Mock Config ---

type MockConfigService struct {
	Cfg     *config.Config
	SaveErr error
}

func (m *MockConfigService) Load() (*config.Config, error)       { return m.Cfg, nil }
func (m *MockConfigService) Save(cfg *config.Config) error       { m.Cfg = cfg; return m.SaveErr }
func (m *MockConfigService) Dir() string                         { return "/mock/.openpool" }
func (m *MockConfigService) KeysDir() string                     { return "/mock/.openpool/keys" }
func (m *MockConfigService) ProfilePath() string                 { return "/mock/.openpool/profile.json" }

// --- Mock Crypto ---

type MockCryptoService struct {
	Pub  ed25519.PublicKey
	Priv ed25519.PrivateKey
}

func (m *MockCryptoService) GenerateKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.Pub, m.Priv, nil
}
func (m *MockCryptoService) LoadKeyPair(_ string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return m.Pub, m.Priv, nil
}
func (m *MockCryptoService) Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return crypto.Encrypt(pub, plaintext)
}
func (m *MockCryptoService) Decrypt(priv ed25519.PrivateKey, ciphertext []byte) ([]byte, error) {
	return crypto.Decrypt(priv, ciphertext)
}
func (m *MockCryptoService) PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error) {
	return crypto.PackUserBin(userPub, operatorPub, plaintext)
}
func (m *MockCryptoService) Sign(priv ed25519.PrivateKey, message []byte) string {
	return crypto.Sign(priv, message)
}

// --- Mock Git ---

type MockGitService struct {
	Repos map[string]map[string][]byte
}

func (m *MockGitService) AddFile(repoURL, path string, content []byte) {
	if m.Repos[repoURL] == nil {
		m.Repos[repoURL] = make(map[string][]byte)
	}
	m.Repos[repoURL][path] = content
}

func (m *MockGitService) Clone(repoURL string) (*gitclient.Repo, error) {
	if _, ok := m.Repos[repoURL]; !ok {
		return nil, fmt.Errorf("repo not found: %s", repoURL)
	}
	return &gitclient.Repo{URL: repoURL, LocalDir: "/mock/" + repoURL}, nil
}
func (m *MockGitService) CloneRegistry(repoURL string) (*gitclient.Repo, error) { return m.Clone(repoURL) }
func (m *MockGitService) EnsureGitURL(input string) string                    { return gitclient.EnsureGitURL(input) }
func (m *MockGitService) FetchRaw(_ context.Context, repoRef, _, path string) ([]byte, error) {
	files, ok := m.Repos[repoRef]
	if !ok { return nil, fmt.Errorf("not found") }
	data, ok := files[path]
	if !ok { return nil, fmt.Errorf("not found: %s", path) }
	return data, nil
}
func (m *MockGitService) FileExistsRaw(_ context.Context, repoRef, _, path string) bool {
	files, ok := m.Repos[repoRef]
	if !ok { return false }
	_, ok = files[path]
	return ok
}

// --- Mock GitHub ---

type mockIssue struct {
	State  string
	Reason string
}

type MockGitHubService struct {
	mu       sync.Mutex
	User     *svc.GitHubUser
	Token    string
	TokenErr error
	Issues   map[int]*mockIssue
	Files    map[string][]byte
	Repos    map[string]bool
	nextID   int
}

func (m *MockGitHubService) GetUser(_ context.Context, _ string) (*svc.GitHubUser, error) {
	if m.User == nil { return nil, fmt.Errorf("no user") }
	return m.User, nil
}
func (m *MockGitHubService) ResolveToken(_ func(string) string) (string, error) {
	if m.TokenErr != nil { return "", m.TokenErr }
	return m.Token, nil
}
func (m *MockGitHubService) CreateIssue(_ context.Context, _, _, _, _ string, _ []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.Issues[m.nextID] = &mockIssue{State: "open"}
	return m.nextID, nil
}
func (m *MockGitHubService) GetIssue(_ context.Context, _, _ string, number int) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	issue, ok := m.Issues[number]
	if !ok { return "", "", fmt.Errorf("issue %d not found", number) }
	return issue.State, issue.Reason, nil
}
func (m *MockGitHubService) StarRepo(_ context.Context, _, _ string) error { return nil }
func (m *MockGitHubService) CreateRepo(_ context.Context, _, _ string, _ bool) error { return nil }
func (m *MockGitHubService) CommitFile(_ context.Context, _, _, _, _ string, _ []byte) error { return nil }
func (m *MockGitHubService) RepoExists(_ context.Context, _, _ string) bool { return false }

// CloseIssue is a test helper.
func (m *MockGitHubService) CloseIssue(number int, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issue, ok := m.Issues[number]; ok {
		issue.State = "closed"
		issue.Reason = reason
	}
}
