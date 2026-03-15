package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	token      string
	repo       string
	httpClient *http.Client
}

func NewClient(repo, token string) *Client {
	return &Client{
		token:      token,
		repo:       repo,
		httpClient: &http.Client{},
	}
}

func (c *Client) apiURL(path string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s%s", c.repo, path)
}

func (c *Client) do(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) GetFile(path string) ([]byte, error) {
	resp, err := c.do("GET", c.apiURL("/contents/"+path), nil)
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

func (c *Client) ListDir(path string) ([]string, error) {
	resp, err := c.do("GET", c.apiURL("/contents/"+path), nil)
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

func (c *Client) TriggerWorkflow(workflowFile string, inputs map[string]string) error {
	payload := map[string]any{
		"ref":    "main",
		"inputs": inputs,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.do("POST",
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

func (c *Client) CreateIssue(title, body string, labels []string) (int, error) {
	payload := map[string]any{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}
	data, _ := json.Marshal(payload)

	resp, err := c.do("POST", c.apiURL("/issues"), strings.NewReader(string(data)))
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

func (c *Client) ListPullRequests(state string) ([]PullRequest, error) {
	url := c.apiURL("/pulls?state=" + state + "&per_page=100")
	resp, err := c.do("GET", url, nil)
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
