package pooldb

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Profile represents a user profile stored in the pool database.
type Profile struct {
	MatchHash   string
	Filters     string // raw JSON
	Vector      []byte // raw bytes (caller decodes float32s)
	DisplayName string
	About       string
	Bio         string
	UpdatedAt   time.Time
}

// Score represents a computed ranking score for a profile.
type Score struct {
	MatchHash   string
	TotalScore  float64
	FieldScores string // raw JSON
	Passed      bool
	RankedAt    time.Time
}

// PoolDB wraps a SQLite database for pool profile storage and scoring.
type PoolDB struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS profiles (
    match_hash   TEXT PRIMARY KEY,
    filters      TEXT,
    vector       BLOB,
    display_name TEXT,
    about        TEXT,
    bio          TEXT,
    updated_at   INTEGER
);

CREATE TABLE IF NOT EXISTS scores (
    match_hash   TEXT PRIMARY KEY,
    total_score  REAL,
    field_scores TEXT,
    passed       INTEGER,
    ranked_at    INTEGER
);

CREATE TABLE IF NOT EXISTS seen (
    match_hash TEXT PRIMARY KEY,
    seen_at    INTEGER,
    action     TEXT
);
`

// Open opens (or creates) a pool SQLite database at path and ensures the schema exists.
func Open(path string) (*PoolDB, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening pool db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating pool db schema: %w", err)
	}

	return &PoolDB{db: db}, nil
}

// Close closes the underlying database connection.
func (p *PoolDB) Close() error {
	return p.db.Close()
}
