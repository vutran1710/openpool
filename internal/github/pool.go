package github

import (
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

func (p *Pool) RegisterProfile(profile UserProfile, signature string) error {
	return p.client.TriggerWorkflow("register.yml", map[string]string{
		"public_id":    profile.PublicID,
		"display_name": profile.DisplayName,
		"bio":          profile.Bio,
		"city":         profile.City,
		"public_key":   profile.PublicKey,
		"looking_for":  profile.LookingFor,
		"status":       profile.Status,
		"signature":    signature,
	})
}

func (p *Pool) CreateLikePR(likerID, likedID, signature string) error {
	return p.client.TriggerWorkflow("like.yml", map[string]string{
		"liker_id":  likerID,
		"liked_id":  likedID,
		"signature": signature,
	})
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

func (p *Pool) AcceptLike(prNumber int, publicID, signature string) error {
	return p.client.TriggerWorkflow("accept.yml", map[string]string{
		"pr_number": fmt.Sprintf("%d", prNumber),
		"public_id": publicID,
		"signature": signature,
	})
}

func (p *Pool) ListMatches(publicID string) ([]string, error) {
	return p.client.ListDir("matches")
}
