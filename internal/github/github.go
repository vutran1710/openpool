package github

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitHubClient defines the interface for GitHub API operations.
// Two implementations exist: HTTPClient (token-based) and CLIClient (gh CLI-based).
type GitHubClient interface {
	// Content operations
	GetFile(ctx context.Context, path string) ([]byte, error)
	ListDir(ctx context.Context, path string) ([]string, error)
	FileExists(ctx context.Context, path string) bool

	// Issue operations
	CreateIssue(ctx context.Context, title, body string, labels []string) (int, error)
	GetIssue(ctx context.Context, number int) (*Issue, error)
	CloseIssue(ctx context.Context, number int, reason string) error
	LockIssue(ctx context.Context, number int, reason string) error
	ListIssues(ctx context.Context, state string, labels ...string) ([]Issue, error)
	ListIssueComments(ctx context.Context, number int) ([]IssueComment, error)
	CommentIssue(ctx context.Context, number int, body string) error

	// Pull request operations
	ListPullRequests(ctx context.Context, state string) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, pr PRRequest) (int, error)
	MergePullRequest(ctx context.Context, prNumber int) error
	ClosePullRequest(ctx context.Context, prNumber int) error
	CommentPR(ctx context.Context, prNumber int, body string) error

	// Repository operations
	GetDefaultBranch(ctx context.Context) (string, error)
	TriggerWorkflow(ctx context.Context, workflowFile string, inputs map[string]string) error
	StarRepo(ctx context.Context) error
}

// IssueComment represents a GitHub issue comment.
type IssueComment struct {
	Body string `json:"body"`
	User User   `json:"user"`
}

// NewCLIOrHTTP returns a CLIClient if gh is installed, otherwise an HTTPClient.
// token is only used when falling back to HTTPClient.
func NewCLIOrHTTP(repo, token string) GitHubClient {
	cli, err := NewCLI(repo)
	if err == nil {
		return cli
	}
	return NewHTTP(repo, token)
}

// GetCLIToken returns the GitHub token from the gh CLI.
// Returns an error if gh is not installed or not authenticated.
func GetCLIToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed: %w (run: gh auth login)", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token from gh CLI")
	}
	return token, nil
}
