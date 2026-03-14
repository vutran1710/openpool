package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type PRFile struct {
	Path    string
	Content []byte
}

type PRRequest struct {
	Title  string
	Body   string
	Branch string
	Labels []string
	Files  []PRFile
}

func (c *Client) GetDefaultBranch() (string, error) {
	resp, err := c.do("GET", fmt.Sprintf("https://api.github.com/repos/%s", c.repo), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return "", err
	}
	return repo.DefaultBranch, nil
}

func (c *Client) getRef(branch string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/git/ref/heads/%s", c.repo, branch)
	resp, err := c.do("GET", url, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

func (c *Client) createRef(branch, sha string) error {
	payload, _ := json.Marshal(map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": sha,
	})

	resp, err := c.do("POST",
		fmt.Sprintf("https://api.github.com/repos/%s/git/refs", c.repo),
		strings.NewReader(string(payload)),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create ref failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

func (c *Client) createOrUpdateFile(path, branch string, content []byte, message string) error {
	encoded := base64.StdEncoding.EncodeToString(content)

	payload := map[string]string{
		"message": message,
		"content": encoded,
		"branch":  branch,
	}

	body, _ := json.Marshal(payload)
	url := c.apiURL("/contents/" + path)

	resp, err := c.do("PUT", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create file failed (%d): %s", resp.StatusCode, respBody)
	}
	return nil
}

func (c *Client) CreatePullRequest(pr PRRequest) (int, error) {
	defaultBranch, err := c.GetDefaultBranch()
	if err != nil {
		return 0, fmt.Errorf("getting default branch: %w", err)
	}

	baseSHA, err := c.getRef(defaultBranch)
	if err != nil {
		return 0, fmt.Errorf("getting base ref: %w", err)
	}

	if err := c.createRef(pr.Branch, baseSHA); err != nil {
		return 0, fmt.Errorf("creating branch: %w", err)
	}

	for _, f := range pr.Files {
		msg := fmt.Sprintf("add %s", f.Path)
		if err := c.createOrUpdateFile(f.Path, pr.Branch, f.Content, msg); err != nil {
			return 0, fmt.Errorf("creating file %s: %w", f.Path, err)
		}
	}

	payload := map[string]any{
		"title": pr.Title,
		"body":  pr.Body,
		"head":  pr.Branch,
		"base":  defaultBranch,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.do("POST", c.apiURL("/pulls"), strings.NewReader(string(body)))
	if err != nil {
		return 0, fmt.Errorf("creating pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("create PR failed (%d): %s", resp.StatusCode, respBody)
	}

	var result PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(pr.Labels) > 0 {
		c.addLabels(result.Number, pr.Labels)
	}

	return result.Number, nil
}

func (c *Client) addLabels(prNumber int, labels []string) {
	payload, _ := json.Marshal(map[string]any{"labels": labels})
	url := c.apiURL(fmt.Sprintf("/issues/%d/labels", prNumber))
	resp, err := c.do("POST", url, strings.NewReader(string(payload)))
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (c *Client) MergePullRequest(prNumber int) error {
	url := c.apiURL(fmt.Sprintf("/pulls/%d/merge", prNumber))
	payload, _ := json.Marshal(map[string]string{"merge_method": "squash"})

	resp, err := c.do("PUT", url, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("merging PR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("merge failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

func (c *Client) FileExists(path string) bool {
	resp, err := c.do("GET", c.apiURL("/contents/"+path), nil)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
