package pooldb

import "fmt"

// InsertProfile inserts or replaces a profile in the database.
func (p *PoolDB) InsertProfile(profile Profile) error {
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO profiles (match_hash, filters, vector, display_name, about, bio, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		profile.MatchHash,
		profile.Filters,
		profile.Vector,
		profile.DisplayName,
		profile.About,
		profile.Bio,
		profile.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("inserting profile %s: %w", profile.MatchHash, err)
	}
	return nil
}

// ClearProfiles deletes all profiles from the database. Used before a full rebuild.
func (p *PoolDB) ClearProfiles() error {
	_, err := p.db.Exec(`DELETE FROM profiles`)
	if err != nil {
		return fmt.Errorf("clearing profiles: %w", err)
	}
	return nil
}
