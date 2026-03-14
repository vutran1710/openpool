package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID `json:"id" db:"id"`
	ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`
	SenderID       uuid.UUID `json:"sender_id" db:"sender_id"`
	Body           string    `json:"body" db:"body"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}
