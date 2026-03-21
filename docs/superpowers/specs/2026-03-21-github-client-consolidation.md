# GitHub Client Consolidation

## Goal

Consolidate all GitHub interactions into a single `internal/github` package with two implementations behind one interface: `CLIClient` (shells out to `gh`) and `HTTPClient` (direct HTTP with PAT).

## Current State

GitHub interactions are scattered:
- `internal/github/client.go` — HTTP client with token, used by pool operations
- `internal/cli/pool.go` — calls `gh auth token` via `exec.Command`
- `internal/cli/tui/screens/matches.go` — calls `gh auth token` via `exec.Command`
- `cmd/matchcrypt/main.go` — no GitHub client (bash handles gh calls in Action)
- `cmd/regcrypt/main.go` — no GitHub client (bash handles gh calls in Action)

## Design

### Interface

```go
// internal/github/github.go

type Client interface {
    // Issues
    CreateIssue(ctx context.Context, title, body string, labels []string) (int, error)
    CloseIssue(ctx context.Context, number int, reason string) error
    CommentIssue(ctx context.Context, number int, body string) error
    GetIssue(ctx context.Context, number int) (*Issue, error)
    ListIssueComments(ctx context.Context, number int) ([]Comment, error)

    // Pull Requests
    CreatePR(ctx context.Context, title, body, base, head string, labels []string) (int, error)
    ClosePR(ctx context.Context, number int) error
    CommentPR(ctx context.Context, number int, body string) error
    ListPRs(ctx context.Context, state string) ([]PR, error)

    // Git (for Action tools)
    AddCommitPush(files []string, message string) error
}
```

### Types

Keep existing types, ensure both clients return the same structs:

```go
type Issue struct {
    Number      int
    State       string
    StateReason string
    Title       string
    Body        string
    User        User
    Labels      []Label
}

type Comment struct {
    Body string
    User User
}

type PR struct {
    Number int
    Title  string
    Body   string
    State  string
    Labels []Label
    User   User
}

type User struct {
    Login string
    ID    int
}

type Label struct {
    Name string
}
```

### CLIClient

```go
// internal/github/cli.go

type CLIClient struct {
    repo string
}

func NewCLI(repo string) *CLIClient {
    return &CLIClient{repo: repo}
}
```

Each method shells out to `gh`:

```go
func (c *CLIClient) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
    args := []string{"issue", "create", "--repo", c.repo, "--title", title, "--body", body}
    for _, l := range labels {
        args = append(args, "--label", l)
    }
    // exec gh, parse issue number from output
}

func (c *CLIClient) ListIssueComments(ctx context.Context, number int) ([]Comment, error) {
    // gh api repos/{repo}/issues/{number}/comments --jq ...
}

func (c *CLIClient) AddCommitPush(files []string, message string) error {
    // git config, git add, git commit, git pull --rebase, git push (with retry)
}
```

Auth: `gh` reads `GH_TOKEN` env var (Actions) or `gh auth` credentials (user machine). No token management needed.

### HTTPClient

```go
// internal/github/http.go

type HTTPClient struct {
    repo  string
    token string
}

func NewHTTP(repo, token string) *HTTPClient {
    return &HTTPClient{repo: repo, token: token}
}
```

Each method makes direct HTTP calls to `api.github.com`:

```go
func (c *HTTPClient) CreateIssue(ctx context.Context, title, body string, labels []string) (int, error) {
    // POST https://api.github.com/repos/{repo}/issues
    // Authorization: Bearer {token}
}

func (c *HTTPClient) AddCommitPush(files []string, message string) error {
    // Not supported — return error
    // HTTPClient is for read/write via API, not git operations
}
```

Note: `AddCommitPush` is only available on `CLIClient` (needs local git). `HTTPClient` returns an error if called. This is fine — Action tools use `CLIClient`, CLI app doesn't need `AddCommitPush`.

### Usage

```go
// Action tools (regcrypt, matchcrypt)
client := github.NewCLI(os.Getenv("REPO"))

// CLI app — try gh first, fall back to stored PAT
client, err := github.NewCLIOrHTTP(repo, storedToken)

// Or explicitly:
client := github.NewHTTP(repo, decryptedPAT)
client := github.NewCLI(repo)
```

### Helper: NewCLIOrHTTP

```go
func NewCLIOrHTTP(repo, fallbackToken string) (Client, error) {
    // Check if gh is available and authenticated
    if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
        token := strings.TrimSpace(string(out))
        if token != "" {
            return NewCLI(repo), nil
        }
    }
    // Fall back to HTTP with stored token
    if fallbackToken != "" {
        return NewHTTP(repo, fallbackToken), nil
    }
    return nil, fmt.Errorf("no GitHub auth: run 'gh auth login' or configure token")
}
```

## Migration

### What gets replaced

| Current | New |
|---------|-----|
| `internal/github/client.go` (custom HTTP) | `internal/github/http.go` (implements Client interface) |
| `exec.Command("gh", "auth", "token")` in pool.go | `github.NewCLI(repo)` or `github.NewCLIOrHTTP()` |
| `exec.Command("gh", "auth", "token")` in matches.go | Same |
| Bash `gh` calls in Action templates | `github.NewCLI(repo)` in Go tools (later, Action refactor) |

### What stays

- `internal/github/pool.go` — pool-level operations (PollRegistrationResult, etc.) — these use the Client interface
- `internal/github/types.go` — schema types (PoolSchema, etc.) — unrelated to GitHub API

### File Plan

| File | Action |
|------|--------|
| Create: `internal/github/github.go` | Client interface + types |
| Create: `internal/github/cli.go` | CLIClient implementation |
| Rename: `internal/github/client.go` → `internal/github/http.go` | HTTPClient, refactored to implement interface |
| Create: `internal/github/cli_test.go` | CLIClient tests (mock gh binary) |
| Modify: `internal/github/http_test.go` | Update existing tests for interface |
| Modify: `internal/github/pool.go` | Use Client interface instead of concrete type |
| Modify: `internal/cli/pool.go` | Use NewCLIOrHTTP, remove gh exec calls |
| Modify: `internal/cli/tui/screens/matches.go` | Use NewCLIOrHTTP, remove gh exec calls |
| Modify: `internal/cli/chat.go` | Remove resolveGHToken if applicable |
| Modify: `internal/cli/svc/svc.go` | Update GitHubService interface if needed |

## Testing

- Unit tests for CLIClient: mock `gh` binary that returns expected JSON
- Unit tests for HTTPClient: httptest server
- Integration: both clients produce identical results for same operations
- Existing tests updated to use interface
