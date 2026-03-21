package github

import (
	"context"
	"os/exec"
	"testing"
)

func skipIfNoGH(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed, skipping")
	}
}

func TestCLIClient_NewCLI(t *testing.T) {
	skipIfNoGH(t)

	cli, err := NewCLI("vutran1710/dating-test-pool")
	if err != nil {
		t.Fatalf("NewCLI() error: %v", err)
	}
	if cli.repo != "vutran1710/dating-test-pool" {
		t.Errorf("repo = %q, want %q", cli.repo, "vutran1710/dating-test-pool")
	}

	// Verify it implements the interface
	var _ GitHubClient = cli
}

func TestCLIClient_ListPullRequests(t *testing.T) {
	skipIfNoGH(t)

	cli, err := NewCLI("vutran1710/dating-test-pool")
	if err != nil {
		t.Fatalf("NewCLI() error: %v", err)
	}

	ctx := context.Background()
	prs, err := cli.ListPullRequests(ctx, "all")
	if err != nil {
		t.Fatalf("ListPullRequests() error: %v", err)
	}

	// Just verify we get a result without error; the test pool may or may not have PRs
	t.Logf("found %d pull requests", len(prs))
}

func TestCLIClient_GetIssue(t *testing.T) {
	skipIfNoGH(t)

	cli, err := NewCLI("vutran1710/dating-test-pool")
	if err != nil {
		t.Fatalf("NewCLI() error: %v", err)
	}

	ctx := context.Background()
	issue, err := cli.GetIssue(ctx, 44)
	if err != nil {
		t.Fatalf("GetIssue(44) error: %v", err)
	}

	if issue.Number != 44 {
		t.Errorf("issue.Number = %d, want 44", issue.Number)
	}
	t.Logf("issue #44 state=%s", issue.State)
}
