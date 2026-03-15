package github

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"

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

func (p *Pool) GetManifest() (*PoolManifest, error) {
	data, err := p.client.GetFile("pool.json")
	if err != nil {
		return nil, fmt.Errorf("reading pool manifest: %w", err)
	}

	var manifest PoolManifest
	if err := decodeJSON(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing pool manifest: %w", err)
	}
	return &manifest, nil
}

func (p *Pool) GetUserBlob(userHash string) ([]byte, error) {
	return p.client.GetFile("users/" + userHash + ".bin")
}

func (p *Pool) ListUsers() ([]string, error) {
	entries, err := p.client.ListDir("users")
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

func (p *Pool) DiscoverRandom(excludeHash string) (string, error) {
	hashes, err := p.ListUsers()
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

func (p *Pool) IsUserRegistered(userHash string) bool {
	if p.repo != nil {
		return p.repo.FileExists("users/" + userHash + ".bin")
	}
	return p.client.FileExists("users/" + userHash + ".bin")
}

func (p *Pool) RegisterUser(userHash string, encryptedBlob []byte, signature, identityProof, templateBody string) (int, error) {
	body := fmt.Sprintf(
		"New member `%s` wants to join.\n\nSignature: `%s`\n\n**Identity proof** (encrypted for operator):\n```\n%s\n```",
		userHash[:12], signature, identityProof,
	)
	if templateBody != "" {
		body = templateBody + "\n\n---\n\n" + body
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Join: %s", userHash[:12]),
		Body:   body,
		Branch: fmt.Sprintf("join/%s", userHash[:12]),
		Files: []PRFile{
			{Path: fmt.Sprintf("users/%s.bin", userHash), Content: encryptedBlob},
		},
	}

	return p.client.CreatePullRequest(pr)
}

// RegisterUserViaIssue submits a registration request as a GitHub issue.
// A GitHub Action will process the issue, commit the .bin file, and close it.
func (p *Pool) RegisterUserViaIssue(userHash string, encryptedBlob []byte, pubKeyHex, signature, identityProof string) (int, error) {
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

	return p.client.CreateIssue("Registration Request", body, []string{"registration"})
}

func (p *Pool) CreateLikePR(likerHash, likedHash, signature string) (int, error) {
	ph := pairHash(likerHash, likedHash)

	likerBlob, err := p.GetUserBlob(likerHash)
	if err != nil {
		return 0, fmt.Errorf("fetching liker: %w", err)
	}
	likedBlob, err := p.GetUserBlob(likedHash)
	if err != nil {
		return 0, fmt.Errorf("fetching liked: %w", err)
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Like: %s -> %s", likerHash[:8], likedHash[:8]),
		Body:   fmt.Sprintf("`%s` likes `%s`\n\nSignature: `%s`", likerHash[:8], likedHash[:8], signature),
		Branch: fmt.Sprintf("like/%s", ph),
		Labels: []string{fmt.Sprintf("like:%s", likedHash[:12])},
		Files: []PRFile{
			{Path: fmt.Sprintf("matches/%s/%s.bin", ph, likerHash), Content: likerBlob},
			{Path: fmt.Sprintf("matches/%s/%s.bin", ph, likedHash), Content: likedBlob},
		},
	}

	return p.client.CreatePullRequest(pr)
}

func (p *Pool) ListIncomingLikes(userHash string) ([]PullRequest, error) {
	return p.listPRsByLabel("like:" + userHash[:12])
}

func (p *Pool) AcceptLike(prNumber int) error {
	return p.client.MergePullRequest(prNumber)
}

func (p *Pool) ListMatches() ([]string, error) {
	return p.client.ListDir("matches")
}

func (p *Pool) CreateProposePR(proposerHash, targetHash, signature string) (int, error) {
	ph := pairHash(proposerHash, targetHash)

	proposerBlob, _ := p.GetUserBlob(proposerHash)
	targetBlob, _ := p.GetUserBlob(targetHash)

	pr := PRRequest{
		Title:  fmt.Sprintf("Propose: %s -> %s", proposerHash[:8], targetHash[:8]),
		Body:   fmt.Sprintf("`%s` proposes to `%s`\n\nSignature: `%s`", proposerHash[:8], targetHash[:8], signature),
		Branch: fmt.Sprintf("propose/%s", ph),
		Labels: []string{fmt.Sprintf("propose:%s", targetHash[:12])},
		Files: []PRFile{
			{Path: fmt.Sprintf("relationships/%s/%s.bin", ph, proposerHash), Content: proposerBlob},
			{Path: fmt.Sprintf("relationships/%s/%s.bin", ph, targetHash), Content: targetBlob},
		},
	}

	return p.client.CreatePullRequest(pr)
}

func (p *Pool) ListIncomingProposals(userHash string) ([]PullRequest, error) {
	return p.listPRsByLabel("propose:" + userHash[:12])
}

func (p *Pool) AcceptPropose(prNumber int) error {
	return p.client.MergePullRequest(prNumber)
}

func (p *Pool) ListRelationships() ([]string, error) {
	return p.client.ListDir("relationships")
}

func (p *Pool) listPRsByLabel(label string) ([]PullRequest, error) {
	prs, err := p.client.ListPullRequests("open")
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
