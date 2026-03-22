package pooldb

import "fmt"

// SyncFromIndex merges profiles from a remote index.db into this database.
// It replaces updated profiles, removes profiles no longer in the index,
// and deletes scores for profiles whose data changed.
// Returns the number of profiles synced.
func (p *PoolDB) SyncFromIndex(indexDBPath string) (int, error) {
	// Attach the remote index database
	_, err := p.db.Exec(`ATTACH DATABASE ? AS remote`, indexDBPath)
	if err != nil {
		return 0, fmt.Errorf("attaching index db: %w", err)
	}
	defer p.db.Exec(`DETACH DATABASE remote`)

	// Delete scores for profiles that changed (updated_at differs or filters/vector differ)
	_, err = p.db.Exec(`
		DELETE FROM scores WHERE match_hash IN (
			SELECT p.match_hash FROM profiles p
			INNER JOIN remote.profiles r ON p.match_hash = r.match_hash
			WHERE p.updated_at != r.updated_at
			   OR p.filters != r.filters
			   OR p.vector != r.vector
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("invalidating scores: %w", err)
	}

	// Delete scores for profiles that no longer exist in remote
	_, err = p.db.Exec(`
		DELETE FROM scores WHERE match_hash NOT IN (
			SELECT match_hash FROM remote.profiles
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("deleting orphaned scores: %w", err)
	}

	// Insert or replace all profiles from remote
	_, err = p.db.Exec(`
		INSERT OR REPLACE INTO profiles (match_hash, filters, vector, display_name, about, bio, updated_at)
		SELECT match_hash, filters, vector, display_name, about, bio, updated_at
		FROM remote.profiles
	`)
	if err != nil {
		return 0, fmt.Errorf("syncing profiles: %w", err)
	}

	// Delete local profiles not present in remote
	_, err = p.db.Exec(`
		DELETE FROM profiles WHERE match_hash NOT IN (
			SELECT match_hash FROM remote.profiles
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("removing deleted profiles: %w", err)
	}

	// Count synced profiles
	var count int
	err = p.db.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting synced profiles: %w", err)
	}

	return count, nil
}
