package gitrepo

// GitOps defines the interface for git repository operations.
// Implementations: package-level functions (real), MockGitOps (test).
type GitOps interface {
	Clone(repoURL string) (*Repo, error)
	CloneRegistry(repoURL string) (*Repo, error)
	ReadFile(repo *Repo, path string) ([]byte, error)
	ListDir(repo *Repo, path string) ([]string, error)
	FileExists(repo *Repo, path string) bool
}
