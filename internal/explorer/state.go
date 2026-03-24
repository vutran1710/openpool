package explorer

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// State manages local explorer state (known constants, checkpoints, seen).
type State struct {
	db *sql.DB
}

func openState(path string) (*State, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS constants (
			match_hash TEXT PRIMARY KEY,
			profile_constant INTEGER,
			learned_at INTEGER
		);
		CREATE TABLE IF NOT EXISTS checkpoints (
			bucket_id TEXT,
			permutation INTEGER,
			position INTEGER,
			chain_hint INTEGER,
			UNIQUE(bucket_id, permutation)
		);
		CREATE TABLE IF NOT EXISTS seen (
			match_hash TEXT PRIMARY KEY,
			seen_at INTEGER,
			action TEXT
		);
	`)
	if err != nil {
		return nil, err
	}

	return &State{db: db}, nil
}

func (s *State) Close() error { return s.db.Close() }

// ClearCheckpoints removes all checkpoint data (call after index.db is updated).
// Constants and seen data are preserved — they carry across rebuilds.
func (s *State) ClearCheckpoints() {
	s.db.Exec("DELETE FROM checkpoints")
}

func (s *State) GetConstant(matchHash string) (int, bool) {
	var c int
	err := s.db.QueryRow("SELECT profile_constant FROM constants WHERE match_hash = ?", matchHash).Scan(&c)
	if err != nil {
		return 0, false
	}
	return c, true
}

func (s *State) SaveConstant(matchHash string, constant int) {
	s.db.Exec("INSERT OR REPLACE INTO constants (match_hash, profile_constant, learned_at) VALUES (?, ?, ?)",
		matchHash, constant, time.Now().Unix())
}

func (s *State) IsSeen(matchHash string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM seen WHERE match_hash = ?", matchHash).Scan(&count)
	return count > 0
}

func (s *State) MarkSeen(matchHash, action string) {
	s.db.Exec("INSERT OR REPLACE INTO seen (match_hash, seen_at, action) VALUES (?, ?, ?)",
		matchHash, time.Now().Unix(), action)
}

func (s *State) GetCheckpoint(bucketID string, perm int) (position int, hint int, ok bool) {
	err := s.db.QueryRow(
		"SELECT position, chain_hint FROM checkpoints WHERE bucket_id = ? AND permutation = ?",
		bucketID, perm).Scan(&position, &hint)
	return position, hint, err == nil
}

func (s *State) SaveCheckpoint(bucketID string, perm, position, hint int) {
	s.db.Exec("INSERT OR REPLACE INTO checkpoints (bucket_id, permutation, position, chain_hint) VALUES (?, ?, ?, ?)",
		bucketID, perm, position, hint)
}
