package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/models"
)

func (s *Store) CreateProfileIndex(ctx context.Context, userID uuid.UUID, displayName, publicID, avatarURL string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO profiles_index (user_id, display_name, public_id, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, displayName, publicID, avatarURL)
	if err != nil {
		return fmt.Errorf("creating profile index: %w", err)
	}
	return nil
}

func (s *Store) UpdateProfileIndex(ctx context.Context, userID uuid.UUID, bio, city string, interests []string, lookingFor string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE profiles_index
		SET bio = $2, city = $3, interests = $4, looking_for = $5, updated_at = now()
		WHERE user_id = $1
	`, userID, bio, city, interests, lookingFor)
	if err != nil {
		return fmt.Errorf("updating profile index: %w", err)
	}
	return nil
}

func (s *Store) GetProfileByPublicID(ctx context.Context, publicID string) (*models.ProfileIndex, error) {
	var p models.ProfileIndex
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, display_name, bio, city, interests, looking_for, discoverable, avatar_url, public_id, updated_at
		FROM profiles_index WHERE public_id = $1
	`, publicID).Scan(
		&p.UserID, &p.DisplayName, &p.Bio, &p.City, &p.Interests,
		&p.LookingFor, &p.Discoverable, &p.AvatarURL, &p.PublicID, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}
	return &p, nil
}

func (s *Store) DiscoverProfiles(ctx context.Context, requesterID uuid.UUID, city, interest string, limit int) ([]models.ProfileIndex, error) {
	if limit <= 0 || limit > 10 {
		limit = 1
	}

	var cityParam, interestParam *string
	if city != "" {
		cityParam = &city
	}
	if interest != "" {
		interestParam = &interest
	}

	rows, err := s.pool.Query(ctx, `SELECT * FROM discover_profiles($1, $2, $3, $4)`,
		requesterID, cityParam, interestParam, limit)
	if err != nil {
		return nil, fmt.Errorf("discovering profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.ProfileIndex
	for rows.Next() {
		var p models.ProfileIndex
		if err := rows.Scan(
			&p.UserID, &p.DisplayName, &p.Bio, &p.City, &p.Interests,
			&p.LookingFor, &p.Discoverable, &p.AvatarURL, &p.PublicID, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}
