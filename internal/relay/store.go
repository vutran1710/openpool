package relay

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// UserEntry is a user in the relay's index.
type UserEntry struct {
	PubKey    ed25519.PublicKey
	UserID    string
	Provider  string
	BinHash   string // primary key
	MatchHash string // TOTP shared secret
}

// Store manages users and matches in SQLite.
type Store struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewStore creates a store backed by SQLite at the given path.
// Use ":memory:" for in-memory (tests).
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			bin_hash   TEXT PRIMARY KEY,
			match_hash TEXT NOT NULL,
			pubkey     TEXT NOT NULL,
			user_id    TEXT NOT NULL DEFAULT '',
			provider   TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS matches (
			pair_key TEXT PRIMARY KEY
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertUser adds or updates a user.
func (s *Store) UpsertUser(entry UserEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(
		`INSERT OR REPLACE INTO users (bin_hash, match_hash, pubkey, user_id, provider) VALUES (?, ?, ?, ?, ?)`,
		entry.BinHash, entry.MatchHash, hex.EncodeToString(entry.PubKey), entry.UserID, entry.Provider,
	)
}

// LookupByHash finds a user by bin_hash.
func (s *Store) LookupByHash(binHash string) *UserEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matchHash, pubkeyHex, userID, provider string
	err := s.db.QueryRow(
		"SELECT match_hash, pubkey, user_id, provider FROM users WHERE bin_hash = ?", binHash,
	).Scan(&matchHash, &pubkeyHex, &userID, &provider)
	if err != nil {
		return nil
	}

	pubBytes, _ := hex.DecodeString(pubkeyHex)
	return &UserEntry{
		BinHash:   binHash,
		MatchHash: matchHash,
		PubKey:    ed25519.PublicKey(pubBytes),
		UserID:    userID,
		Provider:  provider,
	}
}

// AddMatch registers a match between two bin_hashes.
func (s *Store) AddMatch(hashA, hashB string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec("INSERT OR IGNORE INTO matches (pair_key) VALUES (?)", matchKey(hashA, hashB))
}

// IsMatched checks if two bin_hashes are matched.
func (s *Store) IsMatched(hashA, hashB string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM matches WHERE pair_key = ?", matchKey(hashA, hashB)).Scan(&count)
	return count > 0
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

// UserCount returns the number of registered users.
func (s *Store) UserCount() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count
}

// MatchCount returns the number of matches.
func (s *Store) MatchCount() int {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM matches").Scan(&count)
	return count
}
