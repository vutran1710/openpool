package gitrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var rawClient = &http.Client{Timeout: 15 * time.Second}

// RawURL builds a raw.githubusercontent.com URL for a file.
// Accepts "owner/repo" or full git URLs — extracts owner/repo automatically.
func RawURL(repoRef, branch, path string) string {
	ownerRepo := extractOwnerRepo(repoRef)
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", ownerRepo, branch, path)
}

// FetchRaw fetches a file's raw content from GitHub without counting against API rate limits.
// Returns the content bytes, or error if not found / not accessible.
func FetchRaw(ctx context.Context, repoRef, branch, path string) ([]byte, error) {
	url := RawURL(repoRef, branch, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := rawClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found: %s", path)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, path)
	}

	return io.ReadAll(resp.Body)
}

// FileExistsRaw checks if a file exists in a GitHub repo using raw content (no API rate limit).
func FileExistsRaw(ctx context.Context, repoRef, branch, path string) bool {
	url := RawURL(repoRef, branch, path)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false
	}

	resp, err := rawClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == 200
}

// extractOwnerRepo gets "owner/repo" from various formats.
func extractOwnerRepo(ref string) string {
	s := strings.TrimSpace(ref)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	if strings.HasPrefix(s, "git@") {
		if idx := strings.Index(s, ":"); idx != -1 {
			return s[idx+1:]
		}
	}

	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(s, prefix) {
			return strings.TrimPrefix(s, prefix)
		}
	}

	return s
}
