package pooldb

import (
	"database/sql"
	"fmt"
	"time"
)

// ListProfiles returns all profiles in the database.
func (p *PoolDB) ListProfiles() ([]Profile, error) {
	rows, err := p.db.Query(
		`SELECT match_hash, filters, vector, display_name, about, bio, updated_at FROM profiles`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var pr Profile
		var updatedAt int64
		if err := rows.Scan(&pr.MatchHash, &pr.Filters, &pr.Vector, &pr.DisplayName, &pr.About, &pr.Bio, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning profile: %w", err)
		}
		pr.UpdatedAt = time.Unix(updatedAt, 0)
		profiles = append(profiles, pr)
	}
	return profiles, rows.Err()
}

// GetProfile returns a single profile by match_hash, or nil if not found.
func (p *PoolDB) GetProfile(matchHash string) (*Profile, error) {
	var pr Profile
	var updatedAt int64
	err := p.db.QueryRow(
		`SELECT match_hash, filters, vector, display_name, about, bio, updated_at FROM profiles WHERE match_hash = ?`,
		matchHash,
	).Scan(&pr.MatchHash, &pr.Filters, &pr.Vector, &pr.DisplayName, &pr.About, &pr.Bio, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting profile %s: %w", matchHash, err)
	}
	pr.UpdatedAt = time.Unix(updatedAt, 0)
	return &pr, nil
}

// ProfileCount returns the number of profiles in the database.
func (p *PoolDB) ProfileCount() (int, error) {
	var count int
	err := p.db.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting profiles: %w", err)
	}
	return count, nil
}
