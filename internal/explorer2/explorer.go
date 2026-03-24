// Package explorer2 reads chain-encrypted index.db and grinds to unlock profiles.
// Tracks state (known constants, checkpoints, seen) in a local SQLite database.
// Does NOT replace the old discover screen.
package explorer2

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
	"github.com/vutran1710/dating-dev/internal/chainenc"
)

// BucketInfo holds metadata about a bucket.
type BucketInfo struct {
	ID           string
	Partitions   map[string]string
	ProfileCount int
	Permutations int
}

// GrindResult is a single unlocked profile.
type GrindResult struct {
	Tag      string
	Profile  map[string]any
	Constant int
	Attempts int
	Warm     bool
}

// Explorer reads chain-encrypted index.db and grinds to unlock profiles.
type Explorer struct {
	indexDB *sql.DB
	state   *State
}

// Open creates an Explorer from an index.db and a state.db path.
func Open(indexPath, statePath string) (*Explorer, error) {
	indexDB, err := sql.Open("sqlite", indexPath)
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}

	state, err := openState(statePath)
	if err != nil {
		indexDB.Close()
		return nil, fmt.Errorf("opening state: %w", err)
	}

	return &Explorer{indexDB: indexDB, state: state}, nil
}

func (e *Explorer) Close() error {
	e.state.Close()
	return e.indexDB.Close()
}

// ListBuckets returns all buckets in the index.
func (e *Explorer) ListBuckets() ([]BucketInfo, error) {
	rows, err := e.indexDB.Query(`
		SELECT b.bucket_id, b.partition_values, b.profile_count,
		       (SELECT COUNT(*) FROM chains c WHERE c.bucket_id = b.bucket_id) as perms
		FROM buckets b`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []BucketInfo
	for rows.Next() {
		var b BucketInfo
		var partJSON string
		if err := rows.Scan(&b.ID, &partJSON, &b.ProfileCount, &b.Permutations); err != nil {
			continue
		}
		b.Partitions = make(map[string]string)
		json.Unmarshal([]byte(partJSON), &b.Partitions)
		buckets = append(buckets, b)
	}
	return buckets, nil
}

// GrindNext unlocks the next unseen profile in a bucket permutation.
// Returns nil result if all profiles in the chain are seen.
// Skips seen profiles. Uses warm solving for known constants.
func (e *Explorer) GrindNext(bucketID string, permutation int) (*GrindResult, error) {
	// Load chain metadata
	var seed []byte
	var nonceSpace int
	err := e.indexDB.QueryRow(
		"SELECT seed, nonce_space FROM chains WHERE bucket_id = ? AND permutation = ?",
		bucketID, permutation).Scan(&seed, &nonceSpace)
	if err != nil {
		return nil, fmt.Errorf("chain not found: %w", err)
	}

	// Load entries
	rows, err := e.indexDB.Query(
		"SELECT position, tag, ciphertext, gcm_nonce FROM entries WHERE bucket_id = ? AND permutation = ? ORDER BY position",
		bucketID, permutation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type posEntry struct {
		pos   int
		entry chainenc.Entry
	}
	var entries []posEntry
	for rows.Next() {
		var pe posEntry
		if err := rows.Scan(&pe.pos, &pe.entry.Tag, &pe.entry.Ciphertext, &pe.entry.Nonce); err != nil {
			continue
		}
		pe.entry.Context = chainenc.ChainContext(seed, pe.pos)
		entries = append(entries, pe)
	}

	hint := chainenc.SeedHint(seed)
	startPos := -1

	// Resume from checkpoint if available
	if cpPos, cpHint, ok := e.state.GetCheckpoint(bucketID, permutation); ok {
		hint = cpHint
		startPos = cpPos
	}

	for _, pe := range entries {
		// Skip entries before checkpoint
		if pe.pos <= startPos {
			continue
		}
		entry := pe.entry

		// Seen → still need to advance hint via warm solve (need next_hint)
		if e.state.IsSeen(entry.Tag) {
			if constant, ok := e.state.GetConstant(entry.Tag); ok {
				payload, _, err := chainenc.SolveWarm(entry, hint, constant, nonceSpace)
				if err == nil {
					hint = payload.NextHint
					continue
				}
			}
			// Can't advance — chain broken at this point
			break
		}

		// Try warm first
		var result GrindResult
		if constant, ok := e.state.GetConstant(entry.Tag); ok {
			payload, attempts, err := chainenc.SolveWarm(entry, hint, constant, nonceSpace)
			if err == nil {
				result = GrindResult{
					Tag:      entry.Tag,
					Profile:  payload.Profile,
					Constant: payload.MyConstant,
					Attempts: attempts,
					Warm:     true,
				}
				hint = payload.NextHint
			}
		}

		// Cold solve if warm didn't work
		if result.Tag == "" {
			payload, attempts, err := chainenc.SolveCold(entry, hint, chainenc.ConstSpace, nonceSpace)
			if err != nil {
				break
			}
			result = GrindResult{
				Tag:      entry.Tag,
				Profile:  payload.Profile,
				Constant: payload.MyConstant,
				Attempts: attempts,
				Warm:     false,
			}
			e.state.SaveConstant(entry.Tag, payload.MyConstant)
			hint = payload.NextHint
		}

		// Save checkpoint and return the single result
		e.state.SaveCheckpoint(bucketID, permutation, pe.pos, hint)
		return &result, nil
	}

	// All profiles seen or chain broken
	return nil, nil
}

// MarkSeen marks a profile as seen with the given action.
func (e *Explorer) MarkSeen(matchHash, action string) {
	e.state.MarkSeen(matchHash, action)
}
