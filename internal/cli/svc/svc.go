// Package svc defines service interfaces shared between CLI and TUI packages.
// This breaks the circular dependency: screens can import svc without importing cli.
package svc

import (
	"context"
	"crypto/ed25519"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// Services bundles all external dependencies.
type Services struct {
	Config ConfigService
	Crypto CryptoService
	Git    GitService
	GitHub GitHubService
}

// ConfigService abstracts config file operations.
type ConfigService interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
	Dir() string
	KeysDir() string
	ProfilePath() string
}

// CryptoService abstracts cryptographic operations.
type CryptoService interface {
	GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Decrypt(priv ed25519.PrivateKey, ciphertext []byte) ([]byte, error)
	PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Sign(priv ed25519.PrivateKey, message []byte) string
}

// GitService abstracts git repository operations.
type GitService interface {
	Clone(repoURL string) (*gitrepo.Repo, error)
	CloneRegistry(repoURL string) (*gitrepo.Repo, error)
	EnsureGitURL(input string) string
	FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error)
	FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool
}

// GitHubUser holds basic GitHub user info.
type GitHubUser struct {
	UserID      string
	Username    string
	DisplayName string
	Token       string
}

// GitHubService abstracts GitHub API operations.
type GitHubService interface {
	GetUser(ctx context.Context, token string) (*GitHubUser, error)
	ResolveToken(promptFn func(string) string) (string, error)
	CreateIssue(ctx context.Context, repo, token, title, body string, labels []string) (int, error)
	GetIssue(ctx context.Context, repo, token string, number int) (state, reason string, err error)
	StarRepo(ctx context.Context, repo, token string) error
	CreateRepo(ctx context.Context, token, name string, private bool) error
	CommitFile(ctx context.Context, token, repo, path, message string, content []byte) error
	RepoExists(ctx context.Context, token, repo string) bool
}
