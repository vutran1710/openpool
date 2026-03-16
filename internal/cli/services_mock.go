package cli

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// MockServices creates a fully mocked Services for testing.
func MockServices() *Services {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return &Services{
		Config: &MockConfigService{Cfg: &config.Config{}},
		Crypto: &MockCryptoService{Pub: pub, Priv: priv},
		Git:    &MockGitService{Repos: make(map[string]map[string][]byte)},
		GitHub: &MockGitHubService{},
	}
}

// --- Mock Config ---

type MockConfigService struct {
	Cfg     *config.Config
	SaveErr error
}

func (m *MockConfigService) Load() (*config.Config, error)       { return m.Cfg, nil }
func (m *MockConfigService) Save(cfg *config.Config) error       { m.Cfg = cfg; return m.SaveErr }
func (m *MockConfigService) Dir() string                         { return "/mock/.dating" }
func (m *MockConfigService) KeysDir() string                     { return "/mock/.dating/keys" }
func (m *MockConfigService) ProfilePath() string                 { return "/mock/.dating/profile.json" }

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
	Repos map[string]map[string][]byte // repoURL → path → content
}

func (m *MockGitService) AddFile(repoURL, path string, content []byte) {
	if m.Repos[repoURL] == nil {
		m.Repos[repoURL] = make(map[string][]byte)
	}
	m.Repos[repoURL][path] = content
}

func (m *MockGitService) Clone(repoURL string) (*gitrepo.Repo, error) {
	if _, ok := m.Repos[repoURL]; !ok {
		return nil, fmt.Errorf("repo not found: %s", repoURL)
	}
	return &gitrepo.Repo{URL: repoURL, LocalDir: "/mock/" + repoURL}, nil
}

func (m *MockGitService) CloneRegistry(repoURL string) (*gitrepo.Repo, error) {
	return m.Clone(repoURL)
}

func (m *MockGitService) EnsureGitURL(input string) string {
	return gitrepo.EnsureGitURL(input)
}

func (m *MockGitService) FetchRaw(_ context.Context, repoRef, _, path string) ([]byte, error) {
	files, ok := m.Repos[repoRef]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	data, ok := files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return data, nil
}

func (m *MockGitService) FileExistsRaw(_ context.Context, repoRef, _, path string) bool {
	files, ok := m.Repos[repoRef]
	if !ok {
		return false
	}
	_, ok = files[path]
	return ok
}

// --- Mock GitHub ---

type MockGitHubService struct {
	User     *GitHubIdentity
	Token    string
	TokenErr error
}

func (m *MockGitHubService) GetUser(_ context.Context, _ string) (*GitHubIdentity, error) {
	if m.User == nil {
		return nil, fmt.Errorf("no user")
	}
	return m.User, nil
}

func (m *MockGitHubService) ResolveToken(_ func(string) string) (string, error) {
	if m.TokenErr != nil {
		return "", m.TokenErr
	}
	return m.Token, nil
}
