package wsutil

import "time"

type InboundFrame struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	Body           string `json:"body,omitempty"`
}

type OutboundMessage struct {
	Type           string    `json:"type"`
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderPublicID string    `json:"sender_public_id"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

type MatchNotification struct {
	Type           string    `json:"type"`
	MatchID        string    `json:"match_id"`
	ConversationID string    `json:"conversation_id"`
	WithUser       MatchUser `json:"with_user"`
}

type MatchUser struct {
	PublicID    string `json:"public_id"`
	DisplayName string `json:"display_name"`
}

type CommitmentNotification struct {
	Type         string `json:"type"`
	CommitmentID string `json:"commitment_id"`
	FromPublicID string `json:"from_public_id"`
}

type ErrorFrame struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
