package svc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

type realProfile struct {
	dir func() string
}

func NewProfileService(dirFn func() string) ProfileService {
	return &realProfile{dir: dirFn}
}

func (s *realProfile) GlobalPath() string {
	return filepath.Join(s.dir(), "profile.json")
}

func (s *realProfile) PoolPath(poolName string) string {
	return filepath.Join(s.dir(), "pools", poolName, "profile.json")
}

func (s *realProfile) LoadGlobal() (*gh.DatingProfile, error) {
	return s.loadProfile(s.GlobalPath())
}

func (s *realProfile) SaveGlobal(p *gh.DatingProfile) error {
	return s.saveProfile(s.GlobalPath(), p)
}

func (s *realProfile) LoadPool(poolName string) (*gh.DatingProfile, error) {
	return s.loadProfile(s.PoolPath(poolName))
}

func (s *realProfile) SavePool(poolName string, p *gh.DatingProfile) error {
	return s.saveProfile(s.PoolPath(poolName), p)
}

func (s *realProfile) loadProfile(path string) (*gh.DatingProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile: %w", err)
	}
	var p gh.DatingProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile: %w", err)
	}
	return &p, nil
}

func (s *realProfile) saveProfile(path string, p *gh.DatingProfile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}
