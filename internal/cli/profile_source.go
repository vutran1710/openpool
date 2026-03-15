package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	"gopkg.in/yaml.v3"
)

var profileHTTPClient = &http.Client{Timeout: 30 * time.Second}

// FetchGitHubProfile fetches profile fields from the GitHub API.
func FetchGitHubProfile(ctx context.Context, token string) (*gh.DatingProfile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := profileHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var user struct {
		Name            string `json:"name"`
		Login           string `json:"login"`
		Bio             string `json:"bio"`
		Location        string `json:"location"`
		AvatarURL       string `json:"avatar_url"`
		Blog            string `json:"blog"`
		TwitterUsername string `json:"twitter_username"`
	}
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	var social []string
	if user.TwitterUsername != "" {
		social = append(social, "https://twitter.com/"+user.TwitterUsername)
	}

	return &gh.DatingProfile{
		DisplayName: name,
		Bio:         user.Bio,
		Location:    user.Location,
		AvatarURL:   user.AvatarURL,
		Website:     user.Blog,
		Social:      social,
	}, nil
}

// FetchIdentityReadme fetches the user's {username}/{username}/README.md and returns it
// base64-encoded for the Showcase field.
func FetchIdentityReadme(username string) (string, error) {
	repoURL := gitrepo.EnsureGitURL(username + "/" + username)
	repo, err := gitrepo.Clone(repoURL)
	if err != nil {
		return "", fmt.Errorf("identity repo not found: %w", err)
	}

	data, err := repo.ReadFile("README.md")
	if err != nil {
		return "", fmt.Errorf("README.md not found: %w", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// datingReadmeData holds parsed fields from {username}/dating/README.md
type datingReadmeData struct {
	Interests  []string          `yaml:"interests"`
	LookingFor []gh.LookingFor   `yaml:"looking_for"`
	About      string            // body after frontmatter
}

// FetchDatingReadme fetches {username}/dating/README.md and parses its YAML frontmatter.
func FetchDatingReadme(username string) (*datingReadmeData, error) {
	repoURL := gitrepo.EnsureGitURL(username + "/dating")
	repo, err := gitrepo.Clone(repoURL)
	if err != nil {
		return nil, fmt.Errorf("dating repo not found: %w", err)
	}

	data, err := repo.ReadFile("README.md")
	if err != nil {
		return nil, fmt.Errorf("README.md not found: %w", err)
	}

	return parseDatingReadme(data)
}

// DatingRepoExists checks if {username}/dating exists.
func DatingRepoExists(username string) bool {
	repoURL := gitrepo.EnsureGitURL(username + "/dating")
	_, err := gitrepo.Clone(repoURL)
	return err == nil
}

// parseDatingReadme parses a dating README with YAML frontmatter.
func parseDatingReadme(data []byte) (*datingReadmeData, error) {
	content := string(data)

	// Split frontmatter
	if !strings.HasPrefix(content, "---") {
		return &datingReadmeData{About: strings.TrimSpace(content)}, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return &datingReadmeData{About: strings.TrimSpace(content)}, nil
	}

	var result datingReadmeData
	if err := yaml.Unmarshal([]byte(parts[1]), &result); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	// Body is everything after second ---
	body := strings.TrimSpace(parts[2])
	// Strip leading # About heading if present
	if strings.HasPrefix(body, "# About") {
		body = strings.TrimSpace(strings.TrimPrefix(body, "# About"))
	}
	result.About = body

	return &result, nil
}

// MergeProfiles merges fields from multiple sources into a single profile.
// Later sources override earlier ones for non-empty fields.
func MergeProfiles(profiles ...*gh.DatingProfile) *gh.DatingProfile {
	merged := &gh.DatingProfile{}
	for _, p := range profiles {
		if p == nil {
			continue
		}
		if p.DisplayName != "" {
			merged.DisplayName = p.DisplayName
		}
		if p.Bio != "" {
			merged.Bio = p.Bio
		}
		if p.Location != "" {
			merged.Location = p.Location
		}
		if p.AvatarURL != "" {
			merged.AvatarURL = p.AvatarURL
		}
		if p.Website != "" {
			merged.Website = p.Website
		}
		if len(p.Social) > 0 {
			merged.Social = p.Social
		}
		if p.Showcase != "" {
			merged.Showcase = p.Showcase
		}
		if len(p.Interests) > 0 {
			merged.Interests = p.Interests
		}
		if len(p.LookingFor) > 0 {
			merged.LookingFor = p.LookingFor
		}
		if p.About != "" {
			merged.About = p.About
		}
	}
	return merged
}

// GenerateDatingReadme creates a README.md with YAML frontmatter from user input.
func GenerateDatingReadme(interests []string, lookingFor []gh.LookingFor, about string) string {
	var b strings.Builder
	b.WriteString("---\n")
	if len(interests) > 0 {
		b.WriteString("interests: [")
		b.WriteString(strings.Join(interests, ", "))
		b.WriteString("]\n")
	}
	if len(lookingFor) > 0 {
		b.WriteString("looking_for: [")
		b.WriteString(strings.Join(lookingFor, ", "))
		b.WriteString("]\n")
	}
	b.WriteString("---\n\n")
	if about != "" {
		b.WriteString("# About\n\n")
		b.WriteString(about + "\n")
	}
	return b.String()
}
