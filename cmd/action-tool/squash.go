package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/vutran1710/openpool/internal/github"
	"github.com/vutran1710/openpool/internal/gitrepo"
	"github.com/vutran1710/openpool/internal/schema"
)

func cmdSquash() {
	// 1. Squash commits if over threshold
	maxStr := os.Getenv("MAX_COMMIT_COUNT")
	if maxStr == "" {
		maxStr = "50"
	}
	maxCount, _ := strconv.Atoi(maxStr)

	msg := fmt.Sprintf("Pool state (squashed %s)", time.Now().UTC().Format(time.RFC3339))
	squashed, err := gitrepo.SquashHistory(maxCount, msg)
	if err != nil {
		writeError("squash: " + err.Error())
	}
	if squashed {
		log.Printf("squashed to 1 commit")
	} else {
		log.Printf("commits within limit — skipping squash")
	}

	// 2. Close stale interest issues (older than interest_expiry)
	closeStaleInterests()
}

func closeStaleInterests() {
	repo := os.Getenv("REPO")
	if repo == "" {
		return
	}

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
