package handler

import (
	"net/http"

	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
)

type MatchesHandler struct {
	store *store.Store
}

func NewMatchesHandler(s *store.Store) *MatchesHandler {
	return &MatchesHandler{store: s}
}

func (h *MatchesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	matches, err := h.store.GetMatches(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"fetch_failed"}`, http.StatusInternalServerError)
		return
	}

	if matches == nil {
		matches = []api.MatchSummary{}
	}

	writeJSON(w, http.StatusOK, api.MatchesResponse{Matches: matches})
}
