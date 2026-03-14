package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/api"
)

func (s *Store) GetMatches(ctx context.Context, userID uuid.UUID) ([]api.MatchSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, c.id, u.public_id, u.display_name, u.avatar_url
		FROM matches m
		JOIN conversations c ON c.match_id = m.id
		JOIN users u ON u.id = CASE
			WHEN m.user_a = $1 THEN m.user_b
			ELSE m.user_a
		END
		WHERE m.user_a = $1 OR m.user_b = $1
		ORDER BY c.last_message_at DESC NULLS LAST, m.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("getting matches: %w", err)
	}
	defer rows.Close()

	var matches []api.MatchSummary
	for rows.Next() {
		var ms api.MatchSummary
		if err := rows.Scan(
			&ms.MatchID, &ms.ConversationID,
			&ms.WithUser.PublicID, &ms.WithUser.DisplayName, &ms.WithUser.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scanning match: %w", err)
		}
		matches = append(matches, ms)
	}
	return matches, nil
}

func (s *Store) ValidateConversationAccess(ctx context.Context, userID, convID uuid.UUID) error {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversations c
			JOIN matches m ON m.id = c.match_id
			WHERE c.id = $1 AND (m.user_a = $2 OR m.user_b = $2)
		)
	`, convID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validating conversation access: %w", err)
	}
	if !exists {
		return fmt.Errorf("forbidden: not a participant")
	}
	return nil
}

func (s *Store) GetConversationPartner(ctx context.Context, convID, userID uuid.UUID) (uuid.UUID, error) {
	var partnerID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT CASE
			WHEN m.user_a = $2 THEN m.user_b
			ELSE m.user_a
		END
		FROM conversations c
		JOIN matches m ON m.id = c.match_id
		WHERE c.id = $1
	`, convID, userID).Scan(&partnerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("getting conversation partner: %w", err)
	}
	return partnerID, nil
}
