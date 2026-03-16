package cli

import (
	"context"
	"crypto/ed25519"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// Services bundles all external dependencies for the CLI.
// Pass this to screens and commands instead of importing packages directly.
type Services struct {
	Config  ConfigService
	Crypto  CryptoService
	Git     GitService
	GitHub  GitHubService
}

// DefaultServices creates the real service implementations.
func DefaultServices() *Services {
	return &Services{
		Config:  &realConfig{},
		Crypto:  &realCrypto{},
		Git:     &realGit{},
		GitHub:  &realGitHub{},
	}
}

// --- Config Service ---

type ConfigService interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
	Dir() string
	KeysDir() string
	ProfilePath() string
}

type realConfig struct{}

func (r *realConfig) Load() (*config.Config, error) { return config.Load() }
func (r *realConfig) Save(cfg *config.Config) error  { return cfg.Save() }
func (r *realConfig) Dir() string                    { return config.Dir() }
func (r *realConfig) KeysDir() string                { return config.KeysDir() }
func (r *realConfig) ProfilePath() string            { return config.ProfilePath() }

// --- Crypto Service ---

type CryptoService interface {
	GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Decrypt(priv ed25519.PrivateKey, ciphertext []byte) ([]byte, error)
	PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Sign(priv ed25519.PrivateKey, message []byte) string
}

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

// --- Git Service ---

type GitService interface {
	Clone(repoURL string) (*gitrepo.Repo, error)
	CloneRegistry(repoURL string) (*gitrepo.Repo, error)
	EnsureGitURL(input string) string
	FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error)
	FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool
}

type realGit struct{}

func (r *realGit) Clone(repoURL string) (*gitrepo.Repo, error)         { return gitrepo.Clone(repoURL) }
func (r *realGit) CloneRegistry(repoURL string) (*gitrepo.Repo, error) { return gitrepo.CloneRegistry(repoURL) }
func (r *realGit) EnsureGitURL(input string) string                    { return gitrepo.EnsureGitURL(input) }
func (r *realGit) FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error) {
	return gitrepo.FetchRaw(ctx, repoRef, branch, path)
}
func (r *realGit) FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool {
	return gitrepo.FileExistsRaw(ctx, repoRef, branch, path)
}

// --- GitHub Service ---

type GitHubService interface {
	GetUser(ctx context.Context, token string) (*GitHubIdentity, error)
	ResolveToken(promptFn func(string) string) (string, error)
}

type realGitHub struct{}

func (r *realGitHub) GetUser(ctx context.Context, token string) (*GitHubIdentity, error) {
	return fetchGitHubIdentity(ctx, token)
}

func (r *realGitHub) ResolveToken(promptFn func(string) string) (string, error) {
	return resolveGitHubToken(promptFn)
}
