package pooldb

import (
	"fmt"
	"time"
)

// SaveScore inserts or replaces a score for a profile.
func (p *PoolDB) SaveScore(matchHash string, totalScore float64, fieldScores string, passed bool) error {
	passedInt := 0
	if passed {
		passedInt = 1
	}
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO scores (match_hash, total_score, field_scores, passed, ranked_at)
		 VALUES (?, ?, ?, ?, ?)`,
		matchHash, totalScore, fieldScores, passedInt, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("saving score for %s: %w", matchHash, err)
	}
	return nil
}

// MarkSeen records that a profile has been seen with the given action (skip, like).
func (p *PoolDB) MarkSeen(matchHash, action string) error {
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO seen (match_hash, seen_at, action) VALUES (?, ?, ?)`,
		matchHash, time.Now().Unix(), action,
	)
	if err != nil {
		return fmt.Errorf("marking seen %s: %w", matchHash, err)
	}
	return nil
}

// GetSeen returns a map of match_hash → true for all seen profiles.
func (p *PoolDB) GetSeen() (map[string]bool, error) {
	rows, err := p.db.Query(`SELECT match_hash FROM seen`)
	if err != nil {
		return nil, fmt.Errorf("getting seen: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("scanning seen: %w", err)
		}
		seen[hash] = true
	}
	return seen, rows.Err()
}

// ListUnseen returns profiles that have not been marked as seen.
func (p *PoolDB) ListUnseen() ([]Profile, error) {
	rows, err := p.db.Query(`
		SELECT p.match_hash, p.filters, p.vector, p.display_name, p.about, p.bio, p.updated_at
		FROM profiles p
		LEFT JOIN seen s ON p.match_hash = s.match_hash
		WHERE s.match_hash IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("listing unseen: %w", err)
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var pr Profile
		var updatedAt int64
		if err := rows.Scan(&pr.MatchHash, &pr.Filters, &pr.Vector, &pr.DisplayName, &pr.About, &pr.Bio, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning unseen profile: %w", err)
		}
		pr.UpdatedAt = time.Unix(updatedAt, 0)
		profiles = append(profiles, pr)
	}
	return profiles, rows.Err()
}
