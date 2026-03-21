# GitHub Client Consolidation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate all GitHub interactions into a single `internal/github` package with two implementations behind one interface: `CLIClient` (gh CLI) and `HTTPClient` (direct HTTP).

**Architecture:** Define a `GitHubClient` interface in `internal/github/github.go`. Refactor existing `Client` into `HTTPClient` (same code, implements interface). Create `CLIClient` that shells out to `gh`. Update `Pool` struct to accept the interface. Update callers to use `NewCLIOrHTTP`. Remove scattered `exec.Command("gh", ...)` calls.

**Tech Stack:** Go, `os/exec` (for gh CLI), `net/http` (for HTTP client), `net/http/httptest` (for testing)

---

## File Structure

### New Files
- `internal/github/github.go` — `GitHubClient` interface + shared types
- `internal/github/cli.go` — `CLIClient` implementation
- `internal/github/cli_test.go` — CLIClient tests

### Modified Files
- `internal/github/client.go` — Rename `Client` → `HTTPClient`, implement interface
- `internal/github/types.go` — Move `IssueComment` here (currently in client.go)
- `internal/github/pool.go` — Accept `GitHubClient` interface instead of `*Client`
- `internal/github/pullrequest.go` — Methods move to `HTTPClient`
- `internal/github/template.go` — Methods move to `HTTPClient`
- `internal/cli/pool.go` — Remove `resolveGHToken` exec calls, use `NewCLIOrHTTP`
- `internal/cli/tui/screens/matches.go` — Remove `resolveGHToken`, use interface
- `internal/cli/chat.go` — Remove `resolveGHToken`

### Test Files
- `internal/github/cli_test.go` — New
- `internal/github/client_test.go` — Update for interface compliance

---

### Task 1: Define GitHubClient interface + shared types

**Files:**
- Create: `internal/github/github.go`

- [ ] **Step 1: Create the interface file**

```go
package github

import "context"

// GitHubClient is the interface for all GitHub operations.
// Implemented by HTTPClient (direct API) and CLIClient (gh CLI).
type GitHubClient interface {
	// Issues
	CreateIssue(ctx context.Context, title, body string, labels []string) (int, error)
	GetIssue(ctx context.Context, number int) (*Issue, error)
	CloseIssue(ctx context.Context, number int, reason string) error
	ListIssueComments(ctx context.Context, number int) ([]IssueComment, error)
	CommentIssue(ctx context.Context, number int, body string) error

	// Pull Requests
	ListPullRequests(ctx context.Context, state string) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, pr PRRequest) (int, error)
	ClosePullRequest(ctx context.Context, number int) error
	MergePullRequest(ctx context.Context, number int) error
	CommentPR(ctx context.Context, number int, body string) error

	// Files
	GetFile(ctx context.Context, path string) ([]byte, error)
	ListDir(ctx context.Context, path string) ([]string, error)
	FileExists(ctx context.Context, path string) bool

	// Misc
	StarRepo(ctx context.Context) error
}

// IssueComment represents a GitHub issue/PR comment.
type IssueComment struct {
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

// NewCLIOrHTTP returns a CLIClient if gh is available, else HTTPClient with the given token.
func NewCLIOrHTTP(repo, fallbackToken string) (GitHubClient, error) {
	if cli, err := NewCLI(repo); err == nil {
		return cli, nil
	}
	if fallbackToken != "" {
		return NewHTTP(repo, fallbackToken), nil
	}
	return nil, fmt.Errorf("no GitHub auth: run 'gh auth login' or configure token")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/github/`
Expected: May fail (CLIClient/HTTPClient don't exist yet). That's fine — Task 2 and 3 create them.

- [ ] **Step 3: Commit**

```bash
git add internal/github/github.go
git commit -m "feat: define GitHubClient interface"
```

---

### Task 2: Refactor existing Client → HTTPClient

**Files:**
- Modify: `internal/github/client.go`
- Modify: `internal/github/pullrequest.go`
- Modify: `internal/github/template.go`

- [ ] **Step 1: Rename Client to HTTPClient**

In `client.go`:
- `type Client struct` → `type HTTPClient struct`
- `func NewClient(repo, token string) *Client` → `func NewHTTP(repo, token string) *HTTPClient`
- All `func (c *Client)` → `func (c *HTTPClient)`
- Move `IssueComment` type to `github.go` (done in Task 1)

In `pullrequest.go`:
- All `func (c *Client)` → `func (c *HTTPClient)`

In `template.go`:
- All `func (c *Client)` → `func (c *HTTPClient)`

- [ ] **Step 2: Add missing interface methods to HTTPClient**

The interface requires `CloseIssue`, `CommentIssue`, `CommentPR` which don't exist yet. Add them:

```go
func (c *HTTPClient) CloseIssue(ctx context.Context, number int, reason string) error {
	payload := map[string]string{"state": "closed", "state_reason": reason}
	data, _ := json.Marshal(payload)
	resp, err := c.do(ctx, "PATCH", c.apiURL(fmt.Sprintf("/issues/%d", number)), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("close issue failed (%d)", resp.StatusCode)
	}
	return nil
}

func (c *HTTPClient) CommentIssue(ctx context.Context, number int, body string) error {
	payload := map[string]string{"body": body}
	data, _ := json.Marshal(payload)
	resp, err := c.do(ctx, "POST", c.apiURL(fmt.Sprintf("/issues/%d/comments", number)), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("comment failed (%d)", resp.StatusCode)
	}
	return nil
}

func (c *HTTPClient) CommentPR(ctx context.Context, number int, body string) error {
	// PRs use the issues API for comments
	return c.CommentIssue(ctx, number, body)
}
```

- [ ] **Step 3: Keep NewClient as alias for backward compat (temporary)**

```go
// NewClient is an alias for NewHTTP. Deprecated: use NewHTTP.
func NewClient(repo, token string) *HTTPClient {
	return NewHTTP(repo, token)
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/github/`

- [ ] **Step 5: Run existing tests**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/github/ -count=1`
Expected: PASS (rename shouldn't break tests since they use the concrete type)

- [ ] **Step 6: Commit**

```bash
git add internal/github/client.go internal/github/pullrequest.go internal/github/template.go
git commit -m "refactor: rename Client → HTTPClient, implement GitHubClient interface"
```

---

### Task 3: Create CLIClient

**Files:**
- Create: `internal/github/cli.go`
- Create: `internal/github/cli_test.go`

- [ ] **Step 1: Write CLIClient tests**

```go
package github

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func hasGH() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func TestCLIClient_NewCLI(t *testing.T) {
	if !hasGH() {
		t.Skip("gh CLI not installed")
	}
	cli, err := NewCLI("vutran1710/dating-test-pool")
	if err != nil {
		t.Fatal(err)
	}
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCLIClient_ListPullRequests(t *testing.T) {
	if !hasGH() {
		t.Skip("gh CLI not installed")
	}
	cli, _ := NewCLI("vutran1710/dating-test-pool")
	prs, err := cli.ListPullRequests(context.Background(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	// Should return some PRs (test pool has closed PRs)
	t.Logf("found %d closed PRs", len(prs))
}

func TestCLIClient_GetIssue(t *testing.T) {
	if !hasGH() {
		t.Skip("gh CLI not installed")
	}
	cli, _ := NewCLI("vutran1710/dating-test-pool")
	// Issue 44 exists (from e2e test)
	issue, err := cli.GetIssue(context.Background(), 44)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 44 {
		t.Fatalf("expected issue 44, got %d", issue.Number)
	}
}
```

- [ ] **Step 2: Implement CLIClient**

```go
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CLIClient uses the gh CLI for all GitHub operations.
// Auth: reads GH_TOKEN env var or gh auth credentials.
type CLIClient struct {
	repo string
}

// NewCLI creates a CLIClient. Returns error if gh is not installed or not authenticated.
func NewCLI(repo string) (*CLIClient, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not found: %w", err)
	}
	return &CLIClient{repo: NormalizeRepo(repo)}, nil
}

func (c *CLIClient) gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), string(out))
	}
	return out, nil
}

func (c *CLIClient) api(method, path string, result any) error {
	args := []string{"api", "repos/" + c.repo + path, "--method", method}
	out, err := c.gh(args...)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(out, result)
	}
	return nil
}

func (c *CLIClient) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
	args := []string{"issue", "create", "--repo", c.repo, "--title", title, "--body", body}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	out, err := c.gh(args...)
	if err != nil {
		return 0, err
	}
	// Output is the issue URL, parse number from it
	url := strings.TrimSpace(string(out))
	parts := strings.Split(url, "/")
	num, _ := strconv.Atoi(parts[len(parts)-1])
	return num, nil
}

func (c *CLIClient) GetIssue(ctx context.Context, number int) (*Issue, error) {
	var issue Issue
	err := c.api("GET", fmt.Sprintf("/issues/%d", number), &issue)
	return &issue, err
}

func (c *CLIClient) CloseIssue(ctx context.Context, number int, reason string) error {
	args := []string{"issue", "close", strconv.Itoa(number), "--repo", c.repo}
	if reason == "not_planned" {
		args = append(args, "--reason", "not planned")
	}
	_, err := c.gh(args...)
	return err
}

func (c *CLIClient) ListIssueComments(ctx context.Context, number int) ([]IssueComment, error) {
	var comments []IssueComment
	err := c.api("GET", fmt.Sprintf("/issues/%d/comments", number), &comments)
	return comments, err
}

func (c *CLIClient) CommentIssue(ctx context.Context, number int, body string) error {
	_, err := c.gh("issue", "comment", strconv.Itoa(number), "--repo", c.repo, "--body", body)
	return err
}

func (c *CLIClient) ListPullRequests(ctx context.Context, state string) ([]PullRequest, error) {
	out, err := c.gh("pr", "list", "--repo", c.repo, "--state", state, "--json", "number,title,body,state,labels,createdAt", "--limit", "100")
	if err != nil {
		return nil, err
	}
	var prs []PullRequest
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

func (c *CLIClient) CreatePullRequest(ctx context.Context, pr PRRequest) (int, error) {
	args := []string{"pr", "create", "--repo", c.repo, "--title", pr.Title, "--body", pr.Body, "--head", pr.Head, "--base", pr.Base}
	for _, l := range pr.Labels {
		args = append(args, "--label", l)
	}
	out, err := c.gh(args...)
	if err != nil {
		return 0, err
	}
	url := strings.TrimSpace(string(out))
	parts := strings.Split(url, "/")
	num, _ := strconv.Atoi(parts[len(parts)-1])
	return num, nil
}

func (c *CLIClient) ClosePullRequest(ctx context.Context, number int) error {
	_, err := c.gh("pr", "close", strconv.Itoa(number), "--repo", c.repo)
	return err
}

func (c *CLIClient) MergePullRequest(ctx context.Context, number int) error {
	_, err := c.gh("pr", "merge", strconv.Itoa(number), "--repo", c.repo, "--merge")
	return err
}

func (c *CLIClient) CommentPR(ctx context.Context, number int, body string) error {
	_, err := c.gh("pr", "comment", strconv.Itoa(number), "--repo", c.repo, "--body", body)
	return err
}

func (c *CLIClient) GetFile(ctx context.Context, path string) ([]byte, error) {
	out, err := c.gh("api", "repos/"+c.repo+"/contents/"+path, "--jq", ".content")
	if err != nil {
		return nil, err
	}
	return decodeBase64(strings.TrimSpace(string(out)))
}

func (c *CLIClient) ListDir(ctx context.Context, path string) ([]string, error) {
	out, err := c.gh("api", "repos/"+c.repo+"/contents/"+path, "--jq", ".[].name")
	if err != nil {
		return nil, err
	}
	names := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(names) == 1 && names[0] == "" {
		return nil, nil
	}
	return names, nil
}

func (c *CLIClient) FileExists(ctx context.Context, path string) bool {
	_, err := c.gh("api", "repos/"+c.repo+"/contents/"+path, "--silent")
	return err == nil
}

func (c *CLIClient) StarRepo(ctx context.Context) error {
	_, err := c.gh("api", "user/starred/"+c.repo, "--method", "PUT")
	return err
}

func (c *CLIClient) TriggerWorkflow(ctx context.Context, workflowFile string, inputs map[string]string) error {
	args := []string{"workflow", "run", workflowFile, "--repo", c.repo}
	for k, v := range inputs {
		args = append(args, "-f", k+"="+v)
	}
	_, err := c.gh(args...)
	return err
}
```

- [ ] **Step 3: Run tests**

Run: `DATING_HOME=$(mktemp -d) go test -v -run TestCLIClient ./internal/github/ -count=1`
Expected: PASS (or skip if gh not installed)

- [ ] **Step 4: Commit**

```bash
git add internal/github/cli.go internal/github/cli_test.go
git commit -m "feat: CLIClient — gh CLI-based GitHub client"
```

---

### Task 4: Update Pool to use GitHubClient interface

**Files:**
- Modify: `internal/github/pool.go`

- [ ] **Step 1: Change Pool.client from *Client to GitHubClient**

```go
type Pool struct {
	client GitHubClient     // was *Client
	repo   *gitrepo.Repo
}

func NewPool(repoURL, token string) *Pool {
	return &Pool{client: NewHTTP(repoURL, token)}
}

// NewPoolWithClient creates a pool with an explicit GitHub client.
func NewPoolWithClient(client GitHubClient) *Pool {
	return &Pool{client: client}
}

// Client returns the underlying GitHub client.
func (p *Pool) Client() GitHubClient {
	return p.client
}
```

Note: some Pool methods call `p.client.GetPRTemplate`, `p.client.TriggerWorkflow` etc. which may not be on the interface. Either add them to the interface or use type assertion. For methods only on HTTPClient (like `TriggerWorkflow`, `GetPRTemplate`), add them to the interface since CLIClient can implement them too.

- [ ] **Step 2: Add any missing methods to interface**

Check all `p.client.XXX` calls in pool.go and ensure XXX is on the interface. Missing ones to add:
- `TriggerWorkflow`
- `GetPRTemplate` (on template.go)
- `FileExists`

Add these to the `GitHubClient` interface in `github.go`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/github/`

- [ ] **Step 4: Run all github package tests**

Run: `DATING_HOME=$(mktemp -d) go test ./internal/github/ -count=1`

- [ ] **Step 5: Commit**

```bash
git add internal/github/pool.go internal/github/github.go
git commit -m "refactor: Pool accepts GitHubClient interface"
```

---

### Task 5: Update CLI callers — remove scattered gh exec calls

**Files:**
- Modify: `internal/cli/pool.go`
- Modify: `internal/cli/tui/screens/matches.go`
- Modify: `internal/cli/chat.go`

- [ ] **Step 1: Find all resolveGHToken / exec.Command("gh") calls**

```bash
grep -rn "resolveGHToken\|resolveGitHubToken\|exec.Command.*gh\|gh.*auth.*token" internal/cli/ --include='*.go'
```

- [ ] **Step 2: Replace with NewCLIOrHTTP pattern**

Where the code currently does:
```go
ghToken, err := resolveGHToken()
client := gh.NewPool(pool.Repo, ghToken)
```

Replace with:
```go
ghClient, err := github.NewCLIOrHTTP(pool.Repo, decryptedToken)
pool := github.NewPoolWithClient(ghClient)
```

Or if the caller just needs the Pool:
```go
pool := github.NewPool(pool.Repo, "")  // CLIClient via NewCLIOrHTTP internally
```

- [ ] **Step 3: Remove resolveGHToken functions**

Delete `resolveGHToken()` and `resolveGitHubTokenNonInteractive()` from wherever they're defined — the `CLIClient` handles auth via `gh` automatically.

- [ ] **Step 4: Verify full build**

Run: `go build ./...`

- [ ] **Step 5: Run full test suite**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "refactor: use GitHubClient interface, remove scattered gh exec calls"
```

---

### Task 6: Clean up — remove NewClient alias, verify interface compliance

**Files:**
- Modify: `internal/github/client.go`

- [ ] **Step 1: Verify interface compliance with compile-time check**

Add to `client.go` and `cli.go`:

```go
// Compile-time interface checks
var _ GitHubClient = (*HTTPClient)(nil)
var _ GitHubClient = (*CLIClient)(nil)
```

- [ ] **Step 2: Remove NewClient alias if no longer used**

Search: `grep -rn "NewClient\b" --include='*.go'`

If nothing references `NewClient` outside the github package, remove the alias.

- [ ] **Step 3: Run full test suite**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1`

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "clean: interface compliance checks, remove deprecated NewClient"
```

---

### Task 7: Run full test suite + E2E verification

- [ ] **Step 1: Run all tests**

Run: `DATING_HOME=$(mktemp -d) go test ./... -count=1 -race`
Expected: PASS

- [ ] **Step 2: Verify CLIClient works with real pool**

```bash
# Quick smoke test — list issues via CLIClient
go run -exec '' -mod=mod <<'EOF'
package main

import (
    "context"
    "fmt"
    "github.com/vutran1710/dating-dev/internal/github"
)

func main() {
    cli, _ := github.NewCLI("vutran1710/dating-test-pool")
    prs, _ := cli.ListPullRequests(context.Background(), "closed")
    fmt.Printf("CLIClient: %d closed PRs\n", len(prs))

    http := github.NewHTTP("vutran1710/dating-test-pool", "")
    prs2, _ := http.ListPullRequests(context.Background(), "closed")
    fmt.Printf("HTTPClient: %d closed PRs\n", len(prs2))
}
EOF
```

Both clients should return the same count.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test: full suite pass + E2E CLIClient verification"
```
