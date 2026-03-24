package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/vutran1710/openpool/internal/gitrepo"
	"strconv"
	"strings"
)

// CLIClient implements GitHubClient by shelling out to the gh CLI.
type CLIClient struct {
	repo string // "owner/repo"
}

// NewCLI creates a CLIClient after verifying that gh is installed.
func NewCLI(repo string) (*CLIClient, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not found: %w", err)
	}
	return &CLIClient{repo: NormalizeRepo(repo)}, nil
}

// gh runs a gh command and returns stdout bytes.
func (c *CLIClient) gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// api calls gh api with the given path and returns the response body.
func (c *CLIClient) api(method, path string, body ...string) ([]byte, error) {
	args := []string{"api", fmt.Sprintf("repos/%s%s", c.repo, path), "-X", method}
	if len(body) > 0 && body[0] != "" {
		args = append(args, "--input", "-")
		cmd := exec.Command("gh", args...)
		cmd.Stdin = strings.NewReader(body[0])
		out, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("gh api %s: %s", path, string(exitErr.Stderr))
			}
			return nil, fmt.Errorf("gh api %s: %w", path, err)
		}
		return out, nil
	}
	return c.gh(args...)
}

func (c *CLIClient) GetFile(_ context.Context, path string) ([]byte, error) {
	out, err := c.api("GET", "/contents/"+path)
	if err != nil {
		return nil, fmt.Errorf("getting file %s: %w", path, err)
	}

	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(out, &file); err != nil {
		return nil, err
	}
	if file.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", file.Encoding)
	}
	return decodeBase64(file.Content)
}

func (c *CLIClient) ListDir(_ context.Context, path string) ([]string, error) {
	out, err := c.api("GET", "/contents/"+path)
	if err != nil {
		// Treat 404 as empty
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return nil, nil
		}
		return nil, err
	}

	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names, nil
}

func (c *CLIClient) FileExists(_ context.Context, path string) bool {
	_, err := c.api("GET", "/contents/"+path)
	return err == nil
}

func (c *CLIClient) CreateIssue(_ context.Context, title, body string, labels []string) (int, error) {
	args := []string{"issue", "create", "-R", c.repo, "--title", title, "--body", body}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	out, err := c.gh(args...)
	if err != nil {
		return 0, fmt.Errorf("creating issue: %w", err)
	}
	return parseNumberFromURL(strings.TrimSpace(string(out)))
}

func (c *CLIClient) GetIssue(_ context.Context, number int) (*Issue, error) {
	out, err := c.api("GET", fmt.Sprintf("/issues/%d", number))
	if err != nil {
		return nil, fmt.Errorf("getting issue: %w", err)
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *CLIClient) CloseIssue(_ context.Context, number int, reason string) error {
	args := []string{"issue", "close", strconv.Itoa(number), "-R", c.repo}
	if reason == "not_planned" {
		args = append(args, "--reason", "not planned")
	}
	_, err := c.gh(args...)
	return err
}

func (c *CLIClient) LockIssue(_ context.Context, number int, reason string) error {
	args := []string{"issue", "lock", strconv.Itoa(number), "-R", c.repo}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	_, err := c.gh(args...)
	return err
}

func (c *CLIClient) ListIssues(_ context.Context, state string, labels ...string) ([]Issue, error) {
	args := []string{"issue", "list", "--repo", c.repo, "--state", state,
		"--json", "number,title,body,state,stateReason,labels,createdAt", "--limit", "100"}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	out, err := c.gh(args...)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func (c *CLIClient) ListIssueComments(_ context.Context, number int) ([]IssueComment, error) {
	out, err := c.api("GET", fmt.Sprintf("/issues/%d/comments", number))
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	var comments []IssueComment
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (c *CLIClient) CommentIssue(_ context.Context, number int, body string) error {
	_, err := c.gh("issue", "comment", strconv.Itoa(number), "-R", c.repo, "--body", body)
	return err
}

func (c *CLIClient) ListPullRequests(_ context.Context, state string) ([]PullRequest, error) {
	out, err := c.gh("pr", "list", "-R", c.repo, "--state", state, "--json", "number,title,body,state,createdAt,labels,headRefName", "--limit", "100")
	if err != nil {
		return nil, fmt.Errorf("listing PRs: %w", err)
	}

	var ghPRs []struct {
		Number     int       `json:"number"`
		Title      string    `json:"title"`
		Body       string    `json:"body"`
		State      string    `json:"state"`
		CreatedAt  string    `json:"createdAt"`
		Labels     []Label   `json:"labels"`
		HeadRefName string   `json:"headRefName"`
	}
	if err := json.Unmarshal(out, &ghPRs); err != nil {
		return nil, err
	}

	var prs []PullRequest
	for _, p := range ghPRs {
		prs = append(prs, PullRequest{
			Number: p.Number,
			Title:  p.Title,
			Body:   p.Body,
			State:  p.State,
			Labels: p.Labels,
			Head:   Branch{Ref: p.HeadRefName},
		})
	}
	return prs, nil
}

func (c *CLIClient) CreatePullRequest(_ context.Context, pr PRRequest) (int, error) {
	// Get default branch
	repoOut, err := c.gh("api", fmt.Sprintf("repos/%s", c.repo), "--jq", ".default_branch")
	if err != nil {
		return 0, fmt.Errorf("getting default branch: %w", err)
	}
	defaultBranch := strings.TrimSpace(string(repoOut))

	// Get base SHA
	refOut, err := c.api("GET", fmt.Sprintf("/git/ref/heads/%s", defaultBranch))
	if err != nil {
		return 0, fmt.Errorf("getting base ref: %w", err)
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(refOut, &ref); err != nil {
		return 0, err
	}

	// Create branch
	branchPayload, _ := json.Marshal(map[string]string{
		"ref": "refs/heads/" + pr.Branch,
		"sha": ref.Object.SHA,
	})
	_, err = c.api("POST", "/git/refs", string(branchPayload))
	if err != nil {
		return 0, fmt.Errorf("creating branch: %w", err)
	}

	// Create files
	for _, f := range pr.Files {
		filePayload, _ := json.Marshal(map[string]string{
			"message": fmt.Sprintf("add %s", f.Path),
			"content": encodeBase64Content(f.Content),
			"branch":  pr.Branch,
		})
		_, err = c.api("PUT", "/contents/"+f.Path, string(filePayload))
		if err != nil {
			return 0, fmt.Errorf("creating file %s: %w", f.Path, err)
		}
	}

	// Create PR
	args := []string{"pr", "create", "-R", c.repo, "--title", pr.Title, "--body", pr.Body, "--head", pr.Branch, "--base", defaultBranch}
	for _, l := range pr.Labels {
		args = append(args, "--label", l)
	}
	out, err := c.gh(args...)
	if err != nil {
		return 0, fmt.Errorf("creating PR: %w", err)
	}
	return parseNumberFromURL(strings.TrimSpace(string(out)))
}

func (c *CLIClient) MergePullRequest(_ context.Context, prNumber int) error {
	_, err := c.gh("pr", "merge", strconv.Itoa(prNumber), "-R", c.repo, "--squash", "--delete-branch")
	return err
}

func (c *CLIClient) ClosePullRequest(_ context.Context, prNumber int) error {
	_, err := c.gh("pr", "close", strconv.Itoa(prNumber), "-R", c.repo)
	return err
}

func (c *CLIClient) CommentPR(_ context.Context, prNumber int, body string) error {
	_, err := c.gh("pr", "comment", strconv.Itoa(prNumber), "-R", c.repo, "--body", body)
	return err
}

func (c *CLIClient) GetDefaultBranch(_ context.Context) (string, error) {
	out, err := c.gh("api", fmt.Sprintf("repos/%s", c.repo), "--jq", ".default_branch")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *CLIClient) TriggerWorkflow(_ context.Context, workflowFile string, inputs map[string]string) error {
	args := []string{"workflow", "run", workflowFile, "-R", c.repo}
	for k, v := range inputs {
		args = append(args, "-f", fmt.Sprintf("%s=%s", k, v))
	}
	_, err := c.gh(args...)
	return err
}

func (c *CLIClient) StarRepo(_ context.Context) error {
	_, err := c.gh("api", fmt.Sprintf("user/starred/%s", c.repo), "-X", "PUT")
	return err
}

// parseNumberFromURL extracts the issue/PR number from a GitHub URL.
func parseNumberFromURL(url string) (int, error) {
	parts := strings.Split(strings.TrimRight(url, "\n"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("cannot parse number from URL: %s", url)
	}
	return strconv.Atoi(parts[len(parts)-1])
}

// encodeBase64Content base64-encodes file content for the GitHub API.
func encodeBase64Content(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// AddCommitPush stages files, commits, pulls with rebase, and pushes.
// Delegates to gitrepo.AddCommitPush.
func (c *CLIClient) AddCommitPush(files []string, message string) error {
	return gitrepo.AddCommitPush(files, message)
}

func (c *CLIClient) UploadReleaseAsset(_ context.Context, tag, assetName, filePath string) error {
	// Create release if it doesn't exist (ignore error if already exists)
	c.gh("release", "create", tag, "-R", c.repo,
		"--title", tag, "--notes", "Auto-updated by indexer")
	// Upload with --clobber to overwrite existing asset
	_, err := c.gh("release", "upload", tag, filePath, "--clobber", "-R", c.repo)
	return err
}

// Compile-time check: CLIClient implements GitHubClient.
var _ GitHubClient = (*CLIClient)(nil)
