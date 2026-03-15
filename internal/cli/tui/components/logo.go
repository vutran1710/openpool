package components

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/qeesung/image2ascii/convert"
	"github.com/vutran1710/dating-dev/internal/cli/tui/theme"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

var logoExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp"}

// PoolLogoFromRepo finds a logo image in the registry clone, converts it to
// ASCII, caches the result as logo.txt, and returns the ASCII string.
// Falls back to the default heart logo if no image is found.
func PoolLogoFromRepo(registryRepo *gitrepo.Repo, poolName string) string {
	if registryRepo == nil {
		return PoolLogo()
	}

	poolDir := filepath.Join(registryRepo.LocalDir, "pools", poolName)

	// Check for cached ASCII logo first
	cachedPath := filepath.Join(poolDir, "logo.txt")
	if data, err := os.ReadFile(cachedPath); err == nil {
		return theme.BrandStyle.Render(string(data))
	}

	// Find logo image
	imagePath := ""
	for _, ext := range logoExtensions {
		candidate := filepath.Join(poolDir, "logo"+ext)
		if _, err := os.Stat(candidate); err == nil {
			imagePath = candidate
			break
		}
	}

	if imagePath == "" {
		return PoolLogo()
	}

	// Convert image to ASCII
	ascii := convertImageToASCII(imagePath)
	if ascii == "" {
		return PoolLogo()
	}

	// Cache the result
	os.WriteFile(cachedPath, []byte(ascii), 0644)

	return theme.BrandStyle.Render(ascii)
}

func convertImageToASCII(imagePath string) string {
	converter := convert.NewImageConverter()
	opts := convert.DefaultOptions
	opts.FixedWidth = 15
	opts.FixedHeight = 9
	opts.Colored = false

	result := converter.ImageFile2ASCIIString(imagePath, &opts)
	return strings.TrimRight(result, "\n ")
}
