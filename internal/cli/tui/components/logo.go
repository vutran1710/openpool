package components

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/gitrepo"
)

// PoolLogoFromRepo reads logo.txt from a local pool repo clone.
func PoolLogoFromRepo(poolRepo *gitrepo.Repo) string {
	if poolRepo == nil {
		return ""
	}

	logoPath := filepath.Join(poolRepo.LocalDir, "logo.txt")
	data, err := os.ReadFile(logoPath)
	if err != nil {
		return ""
	}

	return theme.BrandStyle.Render(string(data))
}

// PoolLogoFromRaw fetches logo.txt via raw.githubusercontent.com (no clone needed).
func PoolLogoFromRaw(repoRef string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := gitrepo.FetchRaw(ctx, repoRef, "main", "logo.txt")
	if err != nil {
		return ""
	}

	return theme.BrandStyle.Render(string(data))
}
