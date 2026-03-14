package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type LikeResult struct {
	LikeID  uuid.UUID
	MatchID *uuid.UUID
	ConvID  *uuid.UUID
	IsMatch bool
}

func (s *Store) CreateLike(ctx context.Context, likerID, likedID uuid.UUID) (*LikeResult, error) {
	var result LikeResult
	var matchID, convID *uuid.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT like_id, match_id, conv_id, is_match FROM create_like_and_maybe_match($1, $2)`,
		likerID, likedID,
	).Scan(&result.LikeID, &matchID, &convID, &result.IsMatch)
	if err != nil {
		return nil, fmt.Errorf("creating like: %w", err)
	}

	result.MatchID = matchID
	result.ConvID = convID
	return &result, nil
}
