package gitrepo

import (
	"fmt"
	"strings"
)

// MockGitOps is an in-memory git operations mock for testing.
type MockGitOps struct {
	Repos map[string]map[string][]byte // repoURL → path → content
}

func NewMockGitOps() *MockGitOps {
	return &MockGitOps{
		Repos: make(map[string]map[string][]byte),
	}
}

func (m *MockGitOps) AddFile(repoURL, path string, content []byte) {
	if m.Repos[repoURL] == nil {
		m.Repos[repoURL] = make(map[string][]byte)
	}
	m.Repos[repoURL][path] = content
}

func (m *MockGitOps) Clone(repoURL string) (*Repo, error) {
	if _, ok := m.Repos[repoURL]; !ok {
		return nil, fmt.Errorf("repo not found: %s", repoURL)
	}
	return &Repo{URL: repoURL, LocalDir: "/mock/" + repoURL}, nil
}

func (m *MockGitOps) CloneRegistry(repoURL string) (*Repo, error) {
	files, ok := m.Repos[repoURL]
	if !ok {
		return nil, fmt.Errorf("repo not found: %s", repoURL)
	}
	if _, hasRegistry := files["registry.json"]; !hasRegistry {
		return nil, fmt.Errorf("not a valid registry: missing registry.json")
	}
	return &Repo{URL: repoURL, LocalDir: "/mock/" + repoURL}, nil
}

func (m *MockGitOps) ReadFile(repo *Repo, path string) ([]byte, error) {
	files, ok := m.Repos[repo.URL]
	if !ok {
		return nil, fmt.Errorf("repo not cloned: %s", repo.URL)
	}
	data, ok := files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return data, nil
}

func (m *MockGitOps) ListDir(repo *Repo, path string) ([]string, error) {
	files, ok := m.Repos[repo.URL]
	if !ok {
		return nil, fmt.Errorf("repo not cloned: %s", repo.URL)
	}
	prefix := path + "/"
	seen := make(map[string]bool)
	var names []string
	for p := range files {
		if strings.HasPrefix(p, prefix) {
			rest := strings.TrimPrefix(p, prefix)
			name := strings.SplitN(rest, "/", 2)[0]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (m *MockGitOps) FileExists(repo *Repo, path string) bool {
	files, ok := m.Repos[repo.URL]
	if !ok {
		return false
	}
	_, ok = files[path]
	return ok
}
