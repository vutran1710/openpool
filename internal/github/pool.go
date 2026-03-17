package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

type Pool struct {
	client *Client       // for writes (issues, PRs)
	repo   *gitrepo.Repo // for reads (local clone)
}

func NewPool(repoURL, token string) *Pool {
	return &Pool{client: NewClient(repoURL, token)}
}

// NewLocalPool creates a pool from a local git clone (read-only).
func NewLocalPool(repo *gitrepo.Repo) *Pool {
	return &Pool{repo: repo}
}

// ClonePool clones the pool repo and returns a Pool with local read access.
func ClonePool(repoURL string) (*Pool, error) {
	repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(repoURL))
	if err != nil {
		return nil, fmt.Errorf("cloning pool: %w", err)
	}
	return &Pool{repo: repo}, nil
}

func (p *Pool) Client() *Client {
	return p.client
}

func (p *Pool) GetManifest(ctx context.Context) (*PoolManifest, error) {
	data, err := p.client.GetFile(ctx, "pool.json")
	if err != nil {
		return nil, fmt.Errorf("reading pool manifest: %w", err)
	}

	var manifest PoolManifest
	if err := decodeJSON(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing pool manifest: %w", err)
	}
	return &manifest, nil
}

func (p *Pool) GetUserBlob(ctx context.Context, userHash string) ([]byte, error) {
	return p.client.GetFile(ctx, "users/"+userHash+".bin")
}

func (p *Pool) ListUsers(ctx context.Context) ([]string, error) {
	entries, err := p.client.ListDir(ctx, "users")
	if err != nil {
		return nil, err
	}

	var hashes []string
	for _, name := range entries {
		hash := strings.TrimSuffix(name, ".bin")
		if hash != name {
			hashes = append(hashes, hash)
		}
	}
	return hashes, nil
}

func (p *Pool) DiscoverRandom(ctx context.Context, excludeHash string) (string, error) {
	hashes, err := p.ListUsers(ctx)
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, h := range hashes {
		if h != excludeHash {
			candidates = append(candidates, h)
		}
	}

	if len(candidates) == 0 {
		return "", nil
	}

	return candidates[rand.Intn(len(candidates))], nil
}

type PoolStats struct {
	Members       int
	Matches       int
	Relationships int
}

// Stats returns pool statistics from the local clone.
func (p *Pool) Stats() PoolStats {
	var stats PoolStats
	if p.repo == nil {
		return stats
	}
	if users, err := p.repo.ListDir("users"); err == nil {
		for _, u := range users {
			if strings.HasSuffix(u, ".bin") {
				stats.Members++
			}
		}
	}
	if dirs, err := p.repo.ListDir("matches"); err == nil {
		stats.Matches = len(dirs)
	}
	if dirs, err := p.repo.ListDir("relationships"); err == nil {
		stats.Relationships = len(dirs)
	}
	return stats
}

func (p *Pool) IsUserRegistered(ctx context.Context, userHash string) bool {
	if p.repo != nil {
		return p.repo.FileExists("users/" + userHash + ".bin")
	}
	return p.client.FileExists(ctx, "users/"+userHash+".bin")
}

func (p *Pool) RegisterUser(ctx context.Context, userHash string, encryptedBlob []byte, signature, identityProof, templateBody string) (int, error) {
	body := fmt.Sprintf(
		"New member `%s` wants to join.\n\nSignature: `%s`\n\n**Identity proof** (encrypted for operator):\n```\n%s\n```",
		crypto.ShortHash(userHash), signature, identityProof,
	)
	if templateBody != "" {
		body = templateBody + "\n\n---\n\n" + body
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Join: %s", crypto.ShortHash(userHash)),
		Body:   body,
		Branch: fmt.Sprintf("join/%s", userHash),
		Files: []PRFile{
			{Path: fmt.Sprintf("users/%s.bin", userHash), Content: encryptedBlob},
		},
	}

	return p.client.CreatePullRequest(ctx, pr)
}

// RegisterUserViaIssue submits a registration request as a GitHub issue.
// A GitHub Action will process the issue, commit the .bin file, and close it.
func (p *Pool) RegisterUserViaIssue(ctx context.Context, userHash string, encryptedBlob []byte, pubKeyHex, signature, identityProof string) (int, error) {
	blobHex := hex.EncodeToString(encryptedBlob)

	body := fmt.Sprintf(
		"<!-- registration-request -->\n\n"+
			"**User Hash:**\n```\n%s\n```\n\n"+
			"**Public Key:**\n```\n%s\n```\n\n"+
			"**Profile Blob:**\n```\n%s\n```\n\n"+
			"**Signature:**\n```\n%s\n```\n\n"+
			"**Identity Proof:**\n```\n%s\n```",
		userHash, pubKeyHex, blobHex, signature, identityProof,
	)

	return p.client.CreateIssue(ctx, "Registration Request", body, []string{"registration"})
}

// CreateLikeIssue creates a like as a GitHub Issue.
// A Pool Action will process it and create the actual PR.
func (p *Pool) CreateLikeIssue(ctx context.Context, likerHash, likedHash, encryptedMsg, signature string) (int, error) {
	body := fmt.Sprintf("%s\n%s\n%s", likerHash, encryptedMsg, signature)
	title := fmt.Sprintf("Like: %s", crypto.ShortHash(likerHash))
	return p.client.CreateIssue(ctx, title, body, []string{"like"})
}

// CreateLikePR is called by the Pool Action (bot) after validating the like issue.
// Creates a PR with match file using sorted hash filenames.
func (p *Pool) CreateLikePR(ctx context.Context, likerHash, likedHash, encryptedMsg, signature string) (int, error) {
	sortedA, sortedB := likerHash, likedHash
	if sortedA > sortedB {
		sortedA, sortedB = sortedB, sortedA
	}
	matchFile := fmt.Sprintf("matches/%s_%s.json", sortedA, sortedB)
	matchContent := []byte(fmt.Sprintf(`{"created_at":%d}`, time.Now().Unix()))

	pr := PRRequest{
		Title:  fmt.Sprintf("Like: %s -> %s", crypto.ShortHash(likerHash), crypto.ShortHash(likedHash)),
		Body:   fmt.Sprintf("%s\n%s\n%s\n%s", likerHash, likedHash, encryptedMsg, signature),
		Branch: fmt.Sprintf("like/%s_%s", sortedA, sortedB),
		Labels: []string{fmt.Sprintf("like:%s", likedHash)},
		Files: []PRFile{
			{Path: matchFile, Content: matchContent},
		},
	}

	return p.client.CreatePullRequest(ctx, pr)
}

func (p *Pool) ListIncomingLikes(ctx context.Context, userHash string) ([]PullRequest, error) {
	return p.listPRsByLabel(ctx, "like:"+userHash)
}

func (p *Pool) AcceptLike(ctx context.Context, prNumber int) error {
	return p.client.MergePullRequest(ctx, prNumber)
}

func (p *Pool) RejectLike(ctx context.Context, prNumber int) error {
	return p.client.ClosePullRequest(ctx, prNumber)
}

func (p *Pool) ListMatches(ctx context.Context) ([]string, error) {
	return p.client.ListDir(ctx, "matches")
}

func (p *Pool) CreateProposePR(ctx context.Context, proposerHash, targetHash, signature string) (int, error) {
	ph := pairHash(proposerHash, targetHash)

	proposerBlob, err := p.GetUserBlob(ctx, proposerHash)
	if err != nil {
		return 0, fmt.Errorf("fetching proposer: %w", err)
	}
	targetBlob, err := p.GetUserBlob(ctx, targetHash)
	if err != nil {
		return 0, fmt.Errorf("fetching target: %w", err)
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Propose: %s -> %s", crypto.ShortHash(proposerHash), crypto.ShortHash(targetHash)),
		Body:   fmt.Sprintf("`%s` proposes to `%s`\n\nSignature: `%s`", crypto.ShortHash(proposerHash), crypto.ShortHash(targetHash), signature),
		Branch: fmt.Sprintf("propose/%s", ph),
		Labels: []string{fmt.Sprintf("propose:%s", targetHash)},
		Files: []PRFile{
			{Path: fmt.Sprintf("relationships/%s/%s.bin", ph, proposerHash), Content: proposerBlob},
			{Path: fmt.Sprintf("relationships/%s/%s.bin", ph, targetHash), Content: targetBlob},
		},
	}

	return p.client.CreatePullRequest(ctx, pr)
}

func (p *Pool) ListIncomingProposals(ctx context.Context, userHash string) ([]PullRequest, error) {
	return p.listPRsByLabel(ctx, "propose:"+userHash)
}

func (p *Pool) AcceptPropose(ctx context.Context, prNumber int) error {
	return p.client.MergePullRequest(ctx, prNumber)
}

func (p *Pool) ListRelationships(ctx context.Context) ([]string, error) {
	return p.client.ListDir(ctx, "relationships")
}

func (p *Pool) listPRsByLabel(ctx context.Context, label string) ([]PullRequest, error) {
	prs, err := p.client.ListPullRequests(ctx, "open")
	if err != nil {
		return nil, err
	}

	var filtered []PullRequest
	for _, pr := range prs {
		for _, l := range pr.Labels {
			if l.Name == label {
				filtered = append(filtered, pr)
				break
			}
		}
	}
	return filtered, nil
}

func pairHash(a, b string) string {
	combined := a + ":" + b
	if a > b {
		combined = b + ":" + a
	}
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])[:12]
}
