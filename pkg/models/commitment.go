package models

import (
	"time"

	"github.com/google/uuid"
)

type Commitment struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	MatchID    uuid.UUID  `json:"match_id" db:"match_id"`
	ProposerID uuid.UUID `json:"proposer_id" db:"proposer_id"`
	AccepterID uuid.UUID `json:"accepter_id" db:"accepter_id"`
	Status     string    `json:"status" db:"status"`
	ProposedAt time.Time `json:"proposed_at" db:"proposed_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
}
