package relay

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TokenStore manages opaque auth tokens with expiry.
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*tokenEntry
	ttl    time.Duration
}

type tokenEntry struct {
	hashID    string
	expiresAt time.Time
}

// NewTokenStore creates a token store with the given TTL.
func NewTokenStore(ttl time.Duration) *TokenStore {
	ts := &TokenStore{
		tokens: make(map[string]*tokenEntry),
		ttl:    ttl,
	}
	go ts.reapLoop()
	return ts
}

// Issue creates a new token for the given user.
func (ts *TokenStore) Issue(hashID string) (token string, expiresAt int64) {
	b := make([]byte, 32)
	rand.Read(b)
	token = hex.EncodeToString(b)

	exp := time.Now().Add(ts.ttl)

	ts.mu.Lock()
	ts.tokens[token] = &tokenEntry{
		hashID:    hashID,
		expiresAt: exp,
	}
	ts.mu.Unlock()

	return token, exp.Unix()
}

// Validate checks a token and returns the associated hashID.
func (ts *TokenStore) Validate(token string) (hashID string, ok bool) {
	ts.mu.RLock()
	entry, exists := ts.tokens[token]
	ts.mu.RUnlock()

	if !exists || time.Now().After(entry.expiresAt) {
		return "", false
	}
	return entry.hashID, true
}

// Revoke removes a token.
func (ts *TokenStore) Revoke(token string) {
	ts.mu.Lock()
	delete(ts.tokens, token)
	ts.mu.Unlock()
}

// reapLoop periodically removes expired tokens.
func (ts *TokenStore) reapLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ts.reap()
	}
}

func (ts *TokenStore) reap() {
	now := time.Now()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for tok, entry := range ts.tokens {
		if now.After(entry.expiresAt) {
			delete(ts.tokens, tok)
		}
	}
}
