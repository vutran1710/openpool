package relay

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultMatchTTL = 5 * time.Minute
	pubkeySize      = 32
)

type matchCacheEntry struct {
	exists    bool
	fetchedAt time.Time
}

// GitHubCache fetches and caches pubkeys and match files from GitHub raw content.
type GitHubCache struct {
	baseURL  string
	client   *http.Client
	pubkeys  sync.Map // bin_hash → []byte
	matches  sync.Map // pair_hash → matchCacheEntry
	matchTTL time.Duration
}

func NewGitHubCache(baseURL string) *GitHubCache {
	return &GitHubCache{
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 10 * time.Second},
		matchTTL: defaultMatchTTL,
	}
}

// FetchPubkey returns the ed25519 pubkey (first 32 bytes of .bin file).
// Cached forever (pubkeys are immutable).
func (gc *GitHubCache) FetchPubkey(binHash string) ([]byte, error) {
	if v, ok := gc.pubkeys.Load(binHash); ok {
		return v.([]byte), nil
	}
	url := gc.baseURL + "/users/" + binHash + ".bin"
	resp, err := gc.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching pubkey: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("unknown user: %s", binHash)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching pubkey: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading pubkey: %w", err)
	}
	if len(data) < pubkeySize {
		return nil, fmt.Errorf("bin file too short: %d bytes", len(data))
	}
	pubkey := make([]byte, pubkeySize)
	copy(pubkey, data[:pubkeySize])
	gc.pubkeys.Store(binHash, pubkey)
	return pubkey, nil
}

// CheckMatch checks if a match file exists for the given pair_hash.
// Only HTTP 200 and 404 are cached. 5xx/network errors are NOT cached.
func (gc *GitHubCache) CheckMatch(pairHash string) (bool, error) {
	if v, ok := gc.matches.Load(pairHash); ok {
		entry := v.(matchCacheEntry)
		if time.Since(entry.fetchedAt) < gc.matchTTL {
			return entry.exists, nil
		}
		gc.matches.Delete(pairHash)
	}
	url := gc.baseURL + "/matches/" + pairHash + ".json"
	resp, err := gc.client.Get(url)
	if err != nil {
		return false, fmt.Errorf("match check failed")
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == 200 {
		gc.matches.Store(pairHash, matchCacheEntry{exists: true, fetchedAt: time.Now()})
		return true, nil
	}
	if resp.StatusCode == 404 {
		gc.matches.Store(pairHash, matchCacheEntry{exists: false, fetchedAt: time.Now()})
		return false, nil
	}
	return false, fmt.Errorf("match check failed")
}

// SetPubkey injects a pubkey into the cache (for testing).
func (gc *GitHubCache) SetPubkey(binHash string, pubkey []byte) {
	gc.pubkeys.Store(binHash, pubkey)
}

// SetMatch injects a match result into the cache (for testing).
func (gc *GitHubCache) SetMatch(pairHash string, exists bool) {
	gc.matches.Store(pairHash, matchCacheEntry{exists: exists, fetchedAt: time.Now()})
}
