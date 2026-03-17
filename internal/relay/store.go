package relay

import (
	"crypto/ed25519"
	"sync"
)

// UserEntry is a user in the relay's index.
type UserEntry struct {
	PubKey   ed25519.PublicKey
	UserID   string
	Provider string
	HashID   string
}

// Store is an in-memory store for users and matches.
type Store struct {
	mu      sync.RWMutex
	users   map[string]*UserEntry // key: userID + ":" + provider
	matches map[string]bool       // key: sorted pair "hashA:hashB"
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		users:   make(map[string]*UserEntry),
		matches: make(map[string]bool),
	}
}

// UpsertUser adds or updates a user in the index.
func (s *Store) UpsertUser(entry UserEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := entry.UserID + ":" + entry.Provider
	s.users[key] = &entry
}

// LookupUser finds a user by user_id + provider.
func (s *Store) LookupUser(userID, provider string) *UserEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := userID + ":" + provider
	return s.users[key]
}

// LookupByHash finds a user by hash_id (scans all entries).
func (s *Store) LookupByHash(hashID string) *UserEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, entry := range s.users {
		if entry.HashID == hashID {
			return entry
		}
	}
	return nil
}

// AddMatch registers a match between two hash_ids.
func (s *Store) AddMatch(hashA, hashB string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches[matchKey(hashA, hashB)] = true
}

// IsMatched checks if two hash_ids are matched.
func (s *Store) IsMatched(hashA, hashB string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matches[matchKey(hashA, hashB)]
}

func matchKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + ":" + b
}

// PubKeyByHash returns the public key for a user identified by hash_id.
func (s *Store) PubKeyByHash(hashID string) ed25519.PublicKey {
	entry := s.LookupByHash(hashID)
	if entry == nil {
		return nil
	}
	return entry.PubKey
}
