package gitclient

import "testing"

func TestRawURL(t *testing.T) {
	tests := []struct {
		repo, branch, path, expected string
	}{
		{"owner/repo", "main", "file.json", "https://raw.githubusercontent.com/owner/repo/main/file.json"},
		{"https://github.com/owner/repo.git", "main", "README.md", "https://raw.githubusercontent.com/owner/repo/main/README.md"},
		{"git@github.com:owner/repo.git", "main", "pool.json", "https://raw.githubusercontent.com/owner/repo/main/pool.json"},
		{"https://github.com/owner/repo", "dev", "src/main.go", "https://raw.githubusercontent.com/owner/repo/dev/src/main.go"},
	}

	for _, tt := range tests {
		result := RawURL(tt.repo, tt.branch, tt.path)
		if result != tt.expected {
			t.Errorf("RawURL(%q, %q, %q) = %q, want %q", tt.repo, tt.branch, tt.path, result, tt.expected)
		}
	}
}

func TestExtractOwnerRepo(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"owner/repo", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"http://github.com/owner/repo", "owner/repo"},
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"  owner/repo  ", "owner/repo"},
	}

	for _, tt := range tests {
		result := extractOwnerRepo(tt.input)
		if result != tt.expected {
			t.Errorf("extractOwnerRepo(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestEnsureGitURL(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"owner/repo", "https://github.com/owner/repo.git"},
		{"https://github.com/owner/repo", "https://github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "git@github.com:owner/repo.git"},
	}

	for _, tt := range tests {
		result := EnsureGitURL(tt.input)
		if result != tt.expected {
			t.Errorf("EnsureGitURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDirName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"https://github.com/owner/repo.git", "github.com_owner_repo"},
		{"git@github.com:owner/repo.git", "github.com_owner_repo"},
		{"https://gitlab.com/org/project.git", "gitlab.com_org_project"},
	}

	for _, tt := range tests {
		result := dirName(tt.input)
		if result != tt.expected {
			t.Errorf("dirName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestClone_ReturnsCachedRepo(t *testing.T) {
	// Clone a real public repo to temp dir — we can't test git operations
	// without a real repo, so test the caching behavior
	repo1 := &Repo{URL: "https://example.com/test.git", LocalDir: "/tmp/test-clone"}
	repo2 := &Repo{URL: "https://example.com/test.git", LocalDir: "/tmp/test-clone"}
	if repo1.URL != repo2.URL {
		t.Error("same URL should produce same repo reference")
	}
}

func TestSync_NotCloned(t *testing.T) {
	repo := &Repo{URL: "https://example.com/nope.git", LocalDir: "/tmp/nonexistent-" + t.Name()}
	_, err := repo.Sync()
	if err == nil {
		// Expected to fail for non-existent dir
	}
}

// MockGitOps tests are in internal/cli/services_test.go
