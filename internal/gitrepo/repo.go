// Package gitrepo provides local git clone and file access.
// All reads go through local clones — no API calls needed.
package gitrepo

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo represents a locally cloned git repository.
type Repo struct {
	URL     string // git-cloneable URL
	LocalDir string // path to the local clone
}

// CloneDir returns the base directory for all cloned repos.
func CloneDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dating", "repos")
}

// Clone clones a repo (shallow) into ~/.dating/repos/{hash}/.
// If already cloned, pulls latest changes instead.
func Clone(repoURL string) (*Repo, error) {
	localDir := filepath.Join(CloneDir(), dirName(repoURL))

	if isCloned(localDir) {
		// Pull latest
		cmd := exec.Command("git", "-C", localDir, "pull", "--ff-only", "-q")
		cmd.Run() // best-effort, don't fail if offline
		return &Repo{URL: repoURL, LocalDir: localDir}, nil
	}

	// Fresh shallow clone
	if err := os.MkdirAll(filepath.Dir(localDir), 0755); err != nil {
		return nil, fmt.Errorf("creating repos dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--depth=1", "-q", repoURL, localDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("cloning %s: %s", repoURL, strings.TrimSpace(string(out)))
	}

	return &Repo{URL: repoURL, LocalDir: localDir}, nil
}

// CloneRegistry validates a repo is a real registry before fully cloning it.
// Step 1: sparse clone with --filter=blob:none (fetches only tree, ~200KB max)
// Step 2: sparse-checkout registry.json only — if missing, abort + clean up
// Step 3: if valid, disable sparse checkout and pull all files
func CloneRegistry(repoURL string) (*Repo, error) {
	localDir := filepath.Join(CloneDir(), dirName(repoURL))

	if isCloned(localDir) {
		cmd := exec.Command("git", "-C", localDir, "pull", "--ff-only", "-q")
		cmd.Run()
		return &Repo{URL: repoURL, LocalDir: localDir}, nil
	}

	if err := os.MkdirAll(filepath.Dir(localDir), 0755); err != nil {
		return nil, fmt.Errorf("creating repos dir: %w", err)
	}

	// Step 1: sparse clone — only fetches tree metadata, no file blobs
	cmd := exec.Command("git", "clone", "--depth=1", "--filter=blob:none", "--sparse", "-q", repoURL, localDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(localDir)
		return nil, fmt.Errorf("cannot access %s: %s", repoURL, strings.TrimSpace(string(out)))
	}

	// Step 2: sparse-checkout registry.json and pools/ only (--no-cone for file+dir mix)
	cmd = exec.Command("git", "-C", localDir, "sparse-checkout", "set", "--no-cone", "/registry.json", "pools/")
	cmd.Run()

	// Step 3: validate registry.json exists and has required fields
	registryPath := filepath.Join(localDir, "registry.json")
	regData, err := os.ReadFile(registryPath)
	if err != nil {
		os.RemoveAll(localDir)
		return nil, fmt.Errorf("not a valid registry: missing registry.json")
	}

	var regMeta struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(regData, &regMeta); err != nil || regMeta.Name == "" || regMeta.Version == 0 {
		os.RemoveAll(localDir)
		return nil, fmt.Errorf("not a valid registry: invalid registry.json (requires name and version)")
	}

	// Step 4: validate pools/ directory exists
	poolsDir := filepath.Join(localDir, "pools")
	if info, err := os.Stat(poolsDir); err != nil || !info.IsDir() {
		os.RemoveAll(localDir)
		return nil, fmt.Errorf("not a valid registry: missing pools/ directory")
	}

	// Step 5: valid registry — disable sparse checkout and fetch all files
	cmd = exec.Command("git", "-C", localDir, "sparse-checkout", "disable")
	cmd.Run()

	return &Repo{URL: repoURL, LocalDir: localDir}, nil
}

// ReadFile reads a file from the local clone.
func (r *Repo) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(r.LocalDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return data, nil
}

// ListDir lists directory entries in the local clone.
func (r *Repo) ListDir(path string) ([]string, error) {
	fullPath := filepath.Join(r.LocalDir, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing %s: %w", path, err)
	}

	var names []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// FileExists checks if a file exists in the local clone.
func (r *Repo) FileExists(path string) bool {
	fullPath := filepath.Join(r.LocalDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// Pull fetches the latest changes.
func (r *Repo) Pull() error {
	cmd := exec.Command("git", "-C", r.LocalDir, "pull", "--ff-only", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pulling %s: %s", r.URL, strings.TrimSpace(string(out)))
	}
	return nil
}

// Remove deletes the local clone.
func (r *Repo) Remove() error {
	return os.RemoveAll(r.LocalDir)
}

// EnsureGitURL normalizes a repo identifier to a git-cloneable URL.
// "owner/repo" becomes "https://github.com/owner/repo.git".
// Full URLs are returned as-is.
func EnsureGitURL(input string) string {
	s := strings.TrimSpace(input)
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "git@") {
		return s
	}
	return "https://github.com/" + s + ".git"
}

func isCloned(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// dirName creates a filesystem-safe directory name from a git URL.
func dirName(repoURL string) string {
	s := repoURL
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	// git@github.com:owner/repo → github.com_owner_repo
	if strings.HasPrefix(s, "git@") {
		s = strings.TrimPrefix(s, "git@")
		s = strings.ReplaceAll(s, ":", "_")
	}

	// https://github.com/owner/repo → github.com_owner_repo
	for _, prefix := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, prefix)
	}

	s = strings.ReplaceAll(s, "/", "_")
	return s
}
