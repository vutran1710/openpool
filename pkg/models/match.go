package models

import (
	"time"

	"github.com/google/uuid"
)

type Like struct {
	ID        uuid.UUID `json:"id" db:"id"`
	LikerID   uuid.UUID `json:"liker_id" db:"liker_id"`
	LikedID   uuid.UUID `json:"liked_id" db:"liked_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Match struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserA     uuid.UUID `json:"user_a" db:"user_a"`
	UserB     uuid.UUID `json:"user_b" db:"user_b"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Conversation struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	MatchID       uuid.UUID  `json:"match_id" db:"match_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty" db:"last_message_at"`
}
