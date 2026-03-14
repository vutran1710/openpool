package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/models"
)

func (s *Store) CreateCommitment(ctx context.Context, matchID, proposerID, accepterID uuid.UUID) (*models.Commitment, error) {
	var c models.Commitment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO commitments (match_id, proposer_id, accepter_id)
		VALUES ($1, $2, $3)
		RETURNING id, match_id, proposer_id, accepter_id, status, proposed_at, resolved_at
	`, matchID, proposerID, accepterID).Scan(
		&c.ID, &c.MatchID, &c.ProposerID, &c.AccepterID, &c.Status, &c.ProposedAt, &c.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating commitment: %w", err)
	}
	return &c, nil
}

func (s *Store) AcceptCommitment(ctx context.Context, commitmentID, accepterID uuid.UUID) (*models.Commitment, error) {
	var c models.Commitment
	err := s.pool.QueryRow(ctx, `
		UPDATE commitments SET status = 'accepted', resolved_at = now()
		WHERE id = $1 AND accepter_id = $2 AND status = 'proposed'
		RETURNING id, match_id, proposer_id, accepter_id, status, proposed_at, resolved_at
	`, commitmentID, accepterID).Scan(
		&c.ID, &c.MatchID, &c.ProposerID, &c.AccepterID, &c.Status, &c.ProposedAt, &c.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("accepting commitment: %w", err)
	}
	return &c, nil
}

func (s *Store) DeclineCommitment(ctx context.Context, commitmentID, accepterID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE commitments SET status = 'declined', resolved_at = now()
		WHERE id = $1 AND accepter_id = $2 AND status = 'proposed'
	`, commitmentID, accepterID)
	if err != nil {
		return fmt.Errorf("declining commitment: %w", err)
	}
	return nil
}

func (s *Store) GetActiveCommitment(ctx context.Context, userID uuid.UUID) (*models.Commitment, error) {
	var c models.Commitment
	err := s.pool.QueryRow(ctx, `
		SELECT id, match_id, proposer_id, accepter_id, status, proposed_at, resolved_at
		FROM commitments
		WHERE (proposer_id = $1 OR accepter_id = $1) AND status IN ('proposed', 'accepted')
		LIMIT 1
	`, userID).Scan(
		&c.ID, &c.MatchID, &c.ProposerID, &c.AccepterID, &c.Status, &c.ProposedAt, &c.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
