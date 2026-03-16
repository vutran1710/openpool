// Package svc defines service interfaces shared between CLI and TUI packages.
// Each service has a single responsibility and does not reach into another's domain.
package svc

import (
	"context"
	"crypto/ed25519"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

// Services bundles all service dependencies.
type Services struct {
	Config      ConfigService
	Crypto      CryptoService
	Git         GitService
	GitHub      GitHubService
	Profile     ProfileService
	Persistence PersistenceService
	Polling     PollingService
}

// ── Config: read/write setting.toml ──

type ConfigService interface {
	Load() (*config.Config, error)
	Save(cfg *config.Config) error
	Dir() string
	KeysDir() string
}

// ── Crypto: encrypt/decrypt/sign ──

type CryptoService interface {
	GenerateKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	LoadKeyPair(dir string) (ed25519.PublicKey, ed25519.PrivateKey, error)
	Encrypt(pub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Decrypt(priv ed25519.PrivateKey, ciphertext []byte) ([]byte, error)
	PackUserBin(userPub, operatorPub ed25519.PublicKey, plaintext []byte) ([]byte, error)
	Sign(priv ed25519.PrivateKey, message []byte) string
}

// ── Git: clone/fetch repos ──

type GitService interface {
	Clone(repoURL string) (*gitrepo.Repo, error)
	CloneRegistry(repoURL string) (*gitrepo.Repo, error)
	EnsureGitURL(input string) string
	FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error)
	FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool
}

// ── GitHub: API calls ──

type GitHubUser struct {
	UserID      string
	Username    string
	DisplayName string
	Token       string
}

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

// ── Profile: read/write profile files ──

type ProfileService interface {
	// Global profile (all sources merged)
	LoadGlobal() (*gh.DatingProfile, error)
	SaveGlobal(p *gh.DatingProfile) error

	// Per-pool profile (filtered fields submitted to a specific pool)
	LoadPool(poolName string) (*gh.DatingProfile, error)
	SavePool(poolName string, p *gh.DatingProfile) error

	// Paths
	GlobalPath() string
	PoolPath(poolName string) string
}

// ── Persistence: orchestrates writes across config + profiles ──

type PersistenceService interface {
	// Pool registration lifecycle
	SavePendingPool(pool config.PoolConfig) error
	MarkPoolActive(poolName, userHash string) error
	MarkPoolRejected(poolName string) error

	// Token
	SaveEncryptedToken(encryptedHex string) error
	DecryptToken() (string, error)

	// User identity
	SaveUserIdentity(displayName, username, provider, providerUserID string) error

	// Registry
	AddRegistry(repoURL string) error
}

// ── Polling: background status checks ──

// StatusUpdate is emitted when a pending pool's status changes.
type StatusUpdate struct {
	PoolName string
	Status   string // "active" or "rejected"
}

type PollingService interface {
	// Start begins background polling. Updates are sent to the channel.
	Start(ctx context.Context, updates chan<- StatusUpdate)

	// Stop halts background polling.
	Stop()

	// PollOnce runs a single poll cycle (for testing or manual trigger).
	PollOnce(ctx context.Context) *StatusUpdate
}
