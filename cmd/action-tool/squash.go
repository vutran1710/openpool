package main

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"os"
)

func cmdSquash() {
	maxStr := os.Getenv("MAX_COMMIT_COUNT")
	if maxStr == "" {
		maxStr = "50"
	}
	maxCount, _ := strconv.Atoi(maxStr)

	out, _ := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))

	if count <= maxCount {
		log.Printf("commits: %d (max %d) — skipping", count, maxCount)
		return
	}

	log.Printf("squashing %d commits into 1...", count)
	exec.Command("git", "config", "user.name", "openpool-bot").Run()
	exec.Command("git", "config", "user.email", "bot@openpool.dev").Run()
	exec.Command("git", "checkout", "--orphan", "squashed").Run()
	exec.Command("git", "add", "-A").Run()
	msg := fmt.Sprintf("Pool state (squashed %s)", time.Now().UTC().Format(time.RFC3339))
	exec.Command("git", "commit", "-m", msg).Run()
	if out, err := exec.Command("git", "push", "--force", "origin", "squashed:main").CombinedOutput(); err != nil {
		writeError("force push: " + string(out))
	}
	log.Printf("squashed: %d → 1", count)
}
