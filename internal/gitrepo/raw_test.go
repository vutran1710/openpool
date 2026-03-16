package gitrepo

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

func TestMockGitOps(t *testing.T) {
	m := NewMockGitOps()
	m.AddFile("https://example.com/repo.git", "README.md", []byte("hello"))
	m.AddFile("https://example.com/repo.git", "pools/a/pool.json", []byte("{}"))

	// Clone
	repo, err := m.Clone("https://example.com/repo.git")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	// ReadFile
	data, err := m.ReadFile(repo, "README.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected hello, got %s", data)
	}

	// FileExists
	if !m.FileExists(repo, "README.md") {
		t.Error("expected file to exist")
	}
	if m.FileExists(repo, "nonexistent") {
		t.Error("expected file to not exist")
	}

	// ListDir
	dirs, _ := m.ListDir(repo, "pools")
	if len(dirs) != 1 || dirs[0] != "a" {
		t.Errorf("expected [a], got %v", dirs)
	}

	// Clone nonexistent
	_, err = m.Clone("https://missing.com/repo.git")
	if err == nil {
		t.Error("expected error for missing repo")
	}

	// CloneRegistry without registry.json
	_, err = m.CloneRegistry("https://example.com/repo.git")
	if err == nil {
		t.Error("expected error without registry.json")
	}

	// CloneRegistry with registry.json
	m.AddFile("https://example.com/reg.git", "registry.json", []byte(`{"name":"test","version":1}`))
	_, err = m.CloneRegistry("https://example.com/reg.git")
	if err != nil {
		t.Fatalf("clone registry: %v", err)
	}
}
