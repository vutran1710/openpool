package components

import (
	"os"
	"path/filepath"

	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/gitrepo"
)

// PoolLogoFromRepo reads logo.txt from the pool repo clone.
// Returns empty string if no logo found.
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
