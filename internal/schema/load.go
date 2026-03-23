package schema

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load parses a pool.yaml from a local file path or URL.
func Load(pathOrURL string) (*PoolSchema, error) {
	var data []byte
	var err error

	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		var resp *http.Response
		resp, err = http.Get(pathOrURL)
		if err != nil {
			return nil, fmt.Errorf("fetching schema: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetching schema: HTTP %d", resp.StatusCode)
		}
		data, err = io.ReadAll(resp.Body)
	} else {
		data, err = os.ReadFile(pathOrURL)
	}
	if err != nil {
		return nil, fmt.Errorf("reading schema: %w", err)
	}

	var s PoolSchema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing pool.yaml: %w", err)
	}
	return &s, nil
}

func loadValuesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Split on commas and newlines
	raw := strings.ReplaceAll(string(data), ",", "\n")
	lines := strings.Split(raw, "\n")
	var vals []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			vals = append(vals, l)
		}
	}
	return vals, nil
}
