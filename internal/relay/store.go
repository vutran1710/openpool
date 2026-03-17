package relay

import (
	"crypto/ed25519"
	"sync"
)

// UserEntry is a user in the relay's index.
type UserEntry struct {
	PubKey    ed25519.PublicKey
	UserID    string
	Provider  string
	BinHash   string // primary key
	MatchHash string // TOTP shared secret
}

// Store is an in-memory store for users and matches.
type Store struct {
	mu      sync.RWMutex
	users   map[string]*UserEntry // key: bin_hash
	matches map[string]bool       // key: sorted pair "hashA:hashB"
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{
		users:   make(map[string]*UserEntry),
		matches: make(map[string]bool),
	}
}

// UpsertUser adds or updates a user in the index, keyed by BinHash.
func (s *Store) UpsertUser(entry UserEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[entry.BinHash] = &entry
}

// LookupByHash finds a user by bin_hash.
func (s *Store) LookupByHash(binHash string) *UserEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[binHash]
}

// AddMatch registers a match between two bin_hashes.
func (s *Store) AddMatch(hashA, hashB string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches[matchKey(hashA, hashB)] = true
}

// IsMatched checks if two bin_hashes are matched.
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

// PubKeyByHash returns the public key for a user identified by bin_hash.
func (s *Store) PubKeyByHash(binHash string) ed25519.PublicKey {
	entry := s.LookupByHash(binHash)
	if entry == nil {
		return nil
	}
	return entry.PubKey
}
