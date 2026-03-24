package components

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/gitrepo"
)

const (
	logoMaxLines = 30
	logoMaxWidth = 30
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

	return theme.BrandStyle.Render(clampLogo(string(data)))
}

// PoolLogoFromRaw fetches logo.txt via raw.githubusercontent.com (no clone needed).
func PoolLogoFromRaw(repoRef string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := gitrepo.FetchRaw(ctx, repoRef, "main", "logo.txt")
	if err != nil {
		return ""
	}

	return theme.BrandStyle.Render(clampLogo(string(data)))
}

// clampLogo truncates logo to max 20 lines x 20 chars wide.
func clampLogo(raw string) string {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) > logoMaxLines {
		lines = lines[:logoMaxLines]
	}
	for i, line := range lines {
		if len(line) > logoMaxWidth {
			lines[i] = line[:logoMaxWidth]
		}
	}
	return strings.Join(lines, "\n")
}
