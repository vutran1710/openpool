package components

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
	"github.com/qeesung/image2ascii/convert"
	"github.com/vutran1710/openpool/internal/cli/tui/theme"
	"github.com/vutran1710/openpool/internal/gitrepo"
)

const (
	logoMaxBytes = 1 << 20 // 1MB
	logoSize     = 100     // resize to 100x100 before conversion
)

var logoExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".bmp"}

// PoolLogoFromRepo finds a logo image in the registry clone, resizes it to
// 100x100, converts to ASCII, caches the result as logo.txt, and returns
// the ASCII string. Falls back to the default heart logo if no image found
// or image exceeds 1MB.
func PoolLogoFromRepo(registryRepo *gitrepo.Repo, poolName string) string {
	if registryRepo == nil {
		return ""
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
		return ""
	}

	// Check file size (max 1MB)
	info, err := os.Stat(imagePath)
	if err != nil || info.Size() > logoMaxBytes {
		return ""
	}

	// Resize to 100x100 and convert to ASCII
	ascii, err := processLogo(imagePath)
	if err != nil {
		return ""
	}

	// Cache the result
	os.WriteFile(cachedPath, []byte(ascii), 0644)

	return theme.BrandStyle.Render(ascii)
}

// processLogo loads an image, resizes to 100x100, saves the resized version,
// and converts to ASCII art.
func processLogo(imagePath string) (string, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decoding image: %w", err)
	}

	// Resize to 100x100
	resized := resize.Resize(logoSize, logoSize, img, resize.Lanczos3)

	// Overwrite the original with the resized version
	resizedPath := imagePath
	out, err := os.Create(resizedPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	switch format {
	case "png":
		png.Encode(out, resized)
	case "jpeg":
		jpeg.Encode(out, resized, &jpeg.Options{Quality: 90})
	case "gif":
		gif.Encode(out, resized, nil)
	default:
		png.Encode(out, resized)
	}

	// Convert resized image to ASCII
	converter := convert.NewImageConverter()
	opts := convert.DefaultOptions
	opts.FixedWidth = 10
	opts.FixedHeight = 5
	opts.Colored = false

	result := converter.Image2ASCIIString(resized, &opts)
	return strings.TrimRight(result, "\n "), nil
}
