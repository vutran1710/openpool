package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

type GitHubIdentity struct {
	UserID      string
	Username    string
	DisplayName string
	Token       string
}

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
func fetchGitHubIdentity(token string) (*GitHubIdentity, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
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
