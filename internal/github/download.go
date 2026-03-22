package github

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// DownloadReleaseAsset downloads a release asset from a public GitHub repo.
func DownloadReleaseAsset(repoURL, tag, assetName, destPath string) error {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoURL, tag, assetName)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", assetName, err)
	}
	defer resp.Body.Close()
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
	_, err = io.Copy(f, resp.Body)
	return err
}
