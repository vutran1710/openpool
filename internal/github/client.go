package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

type Client struct {
	token      string
	repo       string
	httpClient *http.Client
}

func NewClient(repo, token string) *Client {
	return &Client{
		token:      token,
		repo:       NormalizeRepo(repo),
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// NormalizeRepo extracts "owner/repo" from various git URL formats.
// Accepts: "owner/repo", "https://github.com/owner/repo.git", "git@github.com:owner/repo.git"
func NormalizeRepo(input string) string {
	s := strings.TrimSpace(input)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	// SSH: git@github.com:owner/repo
	if strings.HasPrefix(s, "git@") {
		if idx := strings.Index(s, ":"); idx != -1 {
			return s[idx+1:]
		}
	}

	// HTTPS: https://github.com/owner/repo
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}

	return s
}

func (c *Client) apiURL(path string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s%s", c.repo, path)
}

func (c *Client) do(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) GetFile(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.do(ctx, "GET", c.apiURL("/contents/"+path), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found: %s", path)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}

	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, err
	}

	if file.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", file.Encoding)
	}

	return decodeBase64(file.Content)
}

func (c *Client) ListDir(ctx context.Context, path string) ([]string, error) {
	resp, err := c.do(ctx, "GET", c.apiURL("/contents/"+path), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names, nil
}

func (c *Client) TriggerWorkflow(ctx context.Context, workflowFile string, inputs map[string]string) error {
	payload := map[string]any{
		"ref":    "main",
		"inputs": inputs,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	resp, err := c.do(ctx, "POST",
		c.apiURL("/actions/workflows/"+workflowFile+"/dispatches"),
		strings.NewReader(string(body)),
	)
	if err != nil {
		return fmt.Errorf("triggering workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("workflow trigger failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
	payload := map[string]any{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshaling payload: %w", err)
	}

	resp, err := c.do(ctx, "POST", c.apiURL("/issues"), strings.NewReader(string(data)))
	if err != nil {
		return 0, fmt.Errorf("creating issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("create issue failed (%d): %s", resp.StatusCode, respBody)
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Number, nil
}

func (c *Client) ListPullRequests(ctx context.Context, state string) ([]PullRequest, error) {
	url := c.apiURL("/pulls?state=" + state + "&per_page=100")
	resp, err := c.do(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var prs []PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}
	return prs, nil
}

func (c *Client) GetIssue(ctx context.Context, number int) (*Issue, error) {
	url := c.apiURL(fmt.Sprintf("/issues/%d", number))
	resp, err := c.do(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("getting issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get issue failed (%d)", resp.StatusCode)
	}

	var issue Issue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// IssueComment represents a GitHub issue comment.
type IssueComment struct {
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ListIssueComments returns all comments on an issue.
func (c *Client) ListIssueComments(ctx context.Context, number int) ([]IssueComment, error) {
	url := c.apiURL(fmt.Sprintf("/issues/%d/comments", number))
	resp, err := c.do(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list comments failed (%d)", resp.StatusCode)
	}

	var comments []IssueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (c *Client) StarRepo(ctx context.Context) error {
	url := fmt.Sprintf("https://api.github.com/user/starred/%s", c.repo)
	resp, err := c.do(ctx, "PUT", url, nil)
	if err != nil {
		return fmt.Errorf("starring repo: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
