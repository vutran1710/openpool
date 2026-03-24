package github

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DownloadReleaseAsset downloads a release asset from a public GitHub repo.
// Skips download if the local file is up to date (If-Modified-Since).
func DownloadReleaseAsset(repoURL, tag, assetName, destPath string) error {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, tag, assetName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// Conditional download: skip if local file hasn't changed
	if info, err := os.Stat(destPath); err == nil {
		req.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", assetName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 304 {
		return nil // not modified, local file is current
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("release asset not found: %s/%s/%s", repoURL, tag, assetName)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}

	// Set mtime from Last-Modified header for future conditional checks
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := time.Parse(http.TimeFormat, lm); err == nil {
			os.Chtimes(destPath, t, t)
		}
	}

	return nil
}
