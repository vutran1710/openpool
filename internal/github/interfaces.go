package github

import "context"

// UserInfo holds basic GitHub user information.
type UserInfo struct {
	ID              int    `json:"id"`
	Login           string `json:"login"`
	Name            string `json:"name"`
	Bio             string `json:"bio"`
	Location        string `json:"location"`
	AvatarURL       string `json:"avatar_url"`
	Blog            string `json:"blog"`
	TwitterUsername string `json:"twitter_username"`
}

// Issue represents a GitHub issue with state info for polling.
type Issue struct {
	Number      int    `json:"number"`
	State       string `json:"state"`        // "open" | "closed"
	StateReason string `json:"state_reason"` // "completed" | "not_planned" | null
}

// GitHubAPI defines the interface for GitHub API operations.
// Implementations: Client (real), MockGitHub (test).
type GitHubAPI interface {
	GetUser(ctx context.Context, token string) (*UserInfo, error)
	CreateIssue(ctx context.Context, title, body string, labels []string) (int, error)
	GetIssue(ctx context.Context, number int) (*Issue, error)
	StarRepo(ctx context.Context, token string) error
	CreateRepo(ctx context.Context, token, name string, private bool) error
	CommitFile(ctx context.Context, token, repo, path, message string, content []byte) error
	GetFileContent(ctx context.Context, path string) ([]byte, error)
}
