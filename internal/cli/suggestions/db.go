package suggestions

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	gh "github.com/vutran1710/dating-dev/internal/github"
	_ "modernc.org/sqlite"
)

// DB is a local SQLite database for discovery suggestions.
type DB struct {
	conn *sql.DB
}

// Record is a match_hash + vector pair.
type Record struct {
	MatchHash string
	Vector    []float32
}

// Open opens or creates a suggestions database.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	conn.Exec("PRAGMA journal_mode=WAL")
	conn.Exec("PRAGMA synchronous=NORMAL")
	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS vectors (
		match_hash TEXT PRIMARY KEY,
		vector     BLOB NOT NULL
	)`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating table: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Upsert inserts or replaces a vector record.
func (db *DB) Upsert(matchHash string, vector []float32) error {
	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO vectors (match_hash, vector) VALUES (?, ?)",
		matchHash, encodeVector(vector),
	)
	return err
}

// Exists checks if a match_hash exists in the database.
func (db *DB) Exists(matchHash string) bool {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM vectors WHERE match_hash = ?", matchHash).Scan(&count)
	return count > 0
}

// LoadAll loads all records into memory.
func (db *DB) LoadAll() ([]Record, error) {
	rows, err := db.conn.Query("SELECT match_hash, vector FROM vectors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var mh string
		var blob []byte
		if err := rows.Scan(&mh, &blob); err != nil {
			return nil, err
		}
		records = append(records, Record{
			MatchHash: mh,
			Vector:    decodeVector(blob),
		})
	}
	return records, rows.Err()
}

// SyncFromDir reads .vec files from a directory and upserts new/changed records.
// Returns the number of records added/updated.
func (db *DB) SyncFromDir(dir string) (int, error) {
	vecRecords, err := gh.ReadVecDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading vec dir: %w", err)
	}

	added := 0
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO vectors (match_hash, vector) VALUES (?, ?)")
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	for _, r := range vecRecords {
		if db.Exists(r.MatchHash) {
			continue
		}
		if _, err := stmt.Exec(r.MatchHash, encodeVector(r.Vector)); err != nil {
			tx.Rollback()
			return 0, err
		}
		added++
	}

	return added, tx.Commit()
}

func encodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeVector(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
