package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProfilePath returns the path to a pool-specific profile.
func ProfilePath(datingHome, poolName string) string {
	return filepath.Join(datingHome, "pools", poolName, "profile.json")
}

// LoadProfile loads a pool-specific profile from disk.
// Returns nil, nil if not found.
func LoadProfile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return profile, nil
}

// SaveProfile saves a profile to disk as JSON.
func SaveProfile(path string, profile map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadAndValidate loads a profile and validates against schema.
// Returns (profile, nil) if valid.
// Returns (nil, nil) if profile not found (new user).
// Returns (nil, error) if profile exists but invalid (corrupted/outdated).
func LoadAndValidate(profilePath string, schema *PoolSchema) (map[string]any, error) {
	profile, err := LoadProfile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("corrupted profile: %w", err)
	}
	if profile == nil {
		return nil, nil // not found, new user
	}
	errs := schema.ValidateProfile(profile)
	if len(errs) > 0 {
		return nil, fmt.Errorf("profile outdated: %v", errs[0])
	}
	return profile, nil
}
