package api

import (
	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/models"
)

type WhoamiResponse struct {
	PublicID    string `json:"public_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Provider    string `json:"provider"`
	Status      string `json:"status"`
}

type DiscoverResponse struct {
	Profiles []models.ProfileIndex `json:"profiles"`
}

type LikeRequest struct {
	TargetPublicID string `json:"target_public_id"`
}

type LikeResponse struct {
	Liked          bool       `json:"liked"`
	Matched        bool       `json:"matched"`
	MatchID        *uuid.UUID `json:"match_id,omitempty"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
}

type MatchSummary struct {
	MatchID        uuid.UUID `json:"match_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	WithUser       MatchUser `json:"with_user"`
}

type MatchUser struct {
	PublicID    string `json:"public_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type MatchesResponse struct {
	Matches []MatchSummary `json:"matches"`
}

type MessagesResponse struct {
	Messages []models.Message `json:"messages"`
	HasMore  bool             `json:"has_more"`
}

type CommitmentRequest struct {
	MatchID uuid.UUID `json:"match_id"`
}

type CommitmentResponse struct {
	Commitment models.Commitment `json:"commitment"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
