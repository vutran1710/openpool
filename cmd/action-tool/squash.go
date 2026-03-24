package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/schema"
)

func cmdSquash() {
	// 1. Squash commits if over threshold
	maxStr := os.Getenv("MAX_COMMIT_COUNT")
	if maxStr == "" {
		maxStr = "50"
	}
	maxCount, _ := strconv.Atoi(maxStr)

	out, _ := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))

	if count > maxCount {
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
	} else {
		log.Printf("commits: %d (max %d) — skipping squash", count, maxCount)
	}

	// 2. Close stale interest issues (older than interest_expiry)
	closeStaleInterests()
}

func closeStaleInterests() {
	repo := os.Getenv("REPO")
	if repo == "" {
		return
	}

	// Load schema for interest_expiry
	s, err := schema.Load("pool.yaml")
	if err != nil {
		log.Printf("skipping stale interest cleanup: %v", err)
		return
	}
	expiry, err := s.ParseInterestExpiry()
	if err != nil {
		log.Printf("skipping stale interest cleanup: %v", err)
		return
	}

	gh, err := github.NewCLI(repo)
	if err != nil {
		log.Printf("skipping stale interest cleanup: %v", err)
		return
	}

	ctx := context.Background()
	issues, err := gh.ListIssues(ctx, "open", "interest")
	if err != nil {
		log.Printf("error listing interest issues: %v", err)
		return
	}

	cutoff := time.Now().Add(-expiry)
	closed := 0
	for _, iss := range issues {
		if iss.CreatedAt.Before(cutoff) {
			gh.CloseIssue(ctx, iss.Number, "not_planned")
			gh.LockIssue(ctx, iss.Number, "resolved")
			closed++
		}
	}

	if closed > 0 {
		log.Printf("closed %d stale interest issues (older than %s)", closed, s.InterestExpiry)
	}
}
