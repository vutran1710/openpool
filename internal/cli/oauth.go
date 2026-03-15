package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type GitHubIdentity struct {
	UserID      string
	Username    string
	DisplayName string
	Token       string
}

// ghHTTPClient is a shared HTTP client with a timeout for GitHub API calls.
var ghHTTPClient = &http.Client{Timeout: 30 * time.Second}

// resolveGitHubToken tries `gh auth token` first, then asks the user for a PAT.
func resolveGitHubToken(promptFn func(label string) string) (string, error) {
	// Try gh CLI first
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token, nil
		}
	}

	// Fall back to manual input
	token := promptFn("  GitHub token (or run `gh auth login` first): ")
	if token == "" {
		return "", fmt.Errorf("no GitHub token provided")
	}
	return token, nil
}

// fetchGitHubIdentity fetches the authenticated user's identity from GitHub.
func fetchGitHubIdentity(ctx context.Context, token string) (*GitHubIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ghHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("parsing GitHub response: %w", err)
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return &GitHubIdentity{
		UserID:      fmt.Sprintf("%d", data.ID),
		Username:    data.Login,
		DisplayName: name,
		Token:       token,
	}, nil
}
