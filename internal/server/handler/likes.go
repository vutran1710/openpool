package handler

import (
	"net/http"
	"strings"

	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
)

type LikesHandler struct {
	store *store.Store
}

func NewLikesHandler(s *store.Store) *LikesHandler {
	return &LikesHandler{store: s}
}

func (h *LikesHandler) Like(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req api.LikeRequest
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	target, err := h.store.GetUserByPublicID(r.Context(), req.TargetPublicID)
	if err != nil {
		http.Error(w, `{"error":"user_not_found"}`, http.StatusNotFound)
		return
	}

	result, err := h.store.CreateLike(r.Context(), userID, target.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeJSON(w, http.StatusConflict, api.ErrorResponse{Error: "already_liked"})
			return
		}
		http.Error(w, `{"error":"like_failed"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, api.LikeResponse{
		Liked:          true,
		Matched:        result.IsMatch,
		MatchID:        result.MatchID,
		ConversationID: result.ConvID,
	})
}
