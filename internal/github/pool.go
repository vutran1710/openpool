package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
)

type Pool struct {
	client *Client
}

func NewPool(repo, token string) *Pool {
	return &Pool{client: NewClient(repo, token)}
}

func (p *Pool) GetManifest() (*PoolManifest, error) {
	data, err := p.client.GetFile("pool.json")
	if err != nil {
		return nil, fmt.Errorf("reading pool manifest: %w", err)
	}

	var manifest PoolManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing pool manifest: %w", err)
	}
	return &manifest, nil
}

func (p *Pool) GetProfile(publicID string) (*UserProfile, error) {
	data, err := p.client.GetFile("users/" + publicID + "/public.json")
	if err != nil {
		return nil, fmt.Errorf("reading profile: %w", err)
	}

	var profile UserProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parsing profile: %w", err)
	}
	return &profile, nil
}

func (p *Pool) ListProfilesByIndex(indexType, value string) ([]string, error) {
	path := fmt.Sprintf("index/by-%s/%s", indexType, value)
	return p.client.ListDir(path)
}

func (p *Pool) ListOpenProfiles() ([]string, error) {
	return p.client.ListDir("index/by-status/open")
}

func (p *Pool) DiscoverRandom(exclude string) (*UserProfile, error) {
	ids, err := p.ListOpenProfiles()
	if err != nil {
		return nil, err
	}

	var candidates []string
	for _, id := range ids {
		if id != exclude {
			candidates = append(candidates, id)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	pick := candidates[rand.Intn(len(candidates))]
	return p.GetProfile(pick)
}

func (p *Pool) Client() *Client {
	return p.client
}

func (p *Pool) RegisterProfile(profile UserProfile, signature, templateBody string) (int, error) {
	profileJSON, _ := json.MarshalIndent(profile, "", "  ")
	symlinkContent := []byte(fmt.Sprintf("../../users/%s/public.json", profile.PublicID))

	body := fmt.Sprintf("New member **%s** wants to join.\n\nSignature: %s", profile.DisplayName, signature)
	if templateBody != "" {
		body = templateBody + "\n\n---\n\n" + body
	}

	pr := PRRequest{
		Title:  fmt.Sprintf("Join: %s (%s)", profile.DisplayName, profile.PublicID),
		Body:   body,
		Branch: fmt.Sprintf("join/%s", profile.PublicID),
		Files: []PRFile{
			{Path: fmt.Sprintf("users/%s/public.json", profile.PublicID), Content: profileJSON},
			{Path: fmt.Sprintf("index/by-status/open/%s", profile.PublicID), Content: symlinkContent},
		},
	}

	return p.client.CreatePullRequest(pr)
}

func (p *Pool) CreateLikePR(likerID, likedID, signature string) (int, error) {
	hash := matchHash(likerID, likedID)

	likerProfile, err := p.GetProfile(likerID)
	if err != nil {
		return 0, fmt.Errorf("fetching liker profile: %w", err)
	}
	likedProfile, err := p.GetProfile(likedID)
	if err != nil {
		return 0, fmt.Errorf("fetching liked profile: %w", err)
	}

	likerJSON, _ := json.MarshalIndent(likerProfile, "", "  ")
	likedJSON, _ := json.MarshalIndent(likedProfile, "", "  ")

	pr := PRRequest{
		Title:  fmt.Sprintf("Like: %s -> %s", likerID, likedID),
		Body:   fmt.Sprintf("**%s** likes **%s**\n\nSignature: %s", likerID, likedID, signature),
		Branch: fmt.Sprintf("like/%s", hash),
		Labels: []string{fmt.Sprintf("like:%s", likedID)},
		Files: []PRFile{
			{Path: fmt.Sprintf("matches/%s/%s.json", hash, likerID), Content: likerJSON},
			{Path: fmt.Sprintf("matches/%s/%s.json", hash, likedID), Content: likedJSON},
		},
	}

	return p.client.CreatePullRequest(pr)
}

func matchHash(idA, idB string) string {
	combined := idA + ":" + idB
	if idA > idB {
		combined = idB + ":" + idA
	}
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])[:12]
}

func (p *Pool) ListIncomingLikes(publicID string) ([]PullRequest, error) {
	prs, err := p.client.ListPullRequests("open")
	if err != nil {
		return nil, err
	}

	var incoming []PullRequest
	for _, pr := range prs {
		for _, label := range pr.Labels {
			if label.Name == "like:"+publicID {
				incoming = append(incoming, pr)
				break
			}
		}
	}
	return incoming, nil
}

func (p *Pool) AcceptLike(prNumber int) error {
	return p.client.MergePullRequest(prNumber)
}

func (p *Pool) IsProfileRegistered(publicID string) bool {
	return p.client.FileExists(fmt.Sprintf("users/%s/public.json", publicID))
}

func (p *Pool) ListMatches(publicID string) ([]string, error) {
	return p.client.ListDir("matches")
}
