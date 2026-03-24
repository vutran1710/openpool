package gitclient

import (
	"fmt"
	"os/exec"
	"strings"
)

// AddCommitPush stages files, commits, pulls with rebase, and pushes.
// Must be called from within a git repo directory.
func AddCommitPush(files []string, message string) error {
	// Set git identity (required in CI/Actions environments)
	exec.Command("git", "config", "user.name", "openpool-bot").Run()
	exec.Command("git", "config", "user.email", "bot@openpool.dev").Run()

	// git add
	addArgs := append([]string{"add"}, files...)
	cmd := exec.Command("git", addArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s", string(out))
	}

	// git commit
	cmd = exec.Command("git", "commit", "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", string(out))
	}

	// Stash any unstaged changes (e.g. downloaded binaries in CI)
	exec.Command("git", "stash", "--include-untracked").Run()

	// git pull --rebase + push (retry up to 3 times)
	for attempt := 0; attempt < 3; attempt++ {
		cmd = exec.Command("git", "pull", "--rebase", "origin", "main")
		if out, err := cmd.CombinedOutput(); err != nil {
			outStr := string(out)
			if strings.Contains(outStr, "fatal") || strings.Contains(outStr, "divergent") || strings.Contains(outStr, "unrelated") {
				exec.Command("git", "rebase", "--abort").Run()
				exec.Command("git", "fetch", "--depth=1", "origin", "main").Run()
				exec.Command("git", "reset", "--soft", "origin/main").Run()
				exec.Command("git", "commit", "-m", message).Run()
			} else {
				return fmt.Errorf("git pull --rebase: %s", outStr)
			}
		}
		cmd = exec.Command("git", "push", "origin", "main")
		if out, err := cmd.CombinedOutput(); err != nil {
			if strings.Contains(string(out), "rejected") {
				continue
			}
			exec.Command("git", "stash", "drop").Run()
			return fmt.Errorf("git push: %s", string(out))
		}
		exec.Command("git", "stash", "drop").Run()
		return nil
	}
	exec.Command("git", "stash", "drop").Run()
	return fmt.Errorf("git push failed after 3 retries")
}

// SquashHistory squashes all commits into one if count exceeds max.
// Must be called from within a git repo with fetch-depth=0.
func SquashHistory(maxCount int, message string) (squashed bool, err error) {
	out, _ := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)

	if count <= maxCount {
		return false, nil
	}

	exec.Command("git", "config", "user.name", "openpool-bot").Run()
	exec.Command("git", "config", "user.email", "bot@openpool.dev").Run()
	exec.Command("git", "checkout", "--orphan", "squashed").Run()
	exec.Command("git", "add", "-A").Run()
	exec.Command("git", "commit", "-m", message).Run()

	if out, err := exec.Command("git", "push", "--force", "origin", "squashed:main").CombinedOutput(); err != nil {
		return false, fmt.Errorf("force push: %s", string(out))
	}

	return true, nil
}
