package handler

import (
	"net/http"
	"strconv"

	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
	"github.com/vutran1710/dating-dev/pkg/models"
)

type DiscoverHandler struct {
	store *store.Store
}

func NewDiscoverHandler(s *store.Store) *DiscoverHandler {
	return &DiscoverHandler{store: s}
}

func (h *DiscoverHandler) Discover(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	city := r.URL.Query().Get("city")
	interest := r.URL.Query().Get("interest")

	limit := 1
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	profiles, err := h.store.DiscoverProfiles(r.Context(), userID, city, interest, limit)
	if err != nil {
		http.Error(w, `{"error":"discovery_failed"}`, http.StatusInternalServerError)
		return
	}

	if profiles == nil {
		profiles = []models.ProfileIndex{}
	}

	writeJSON(w, http.StatusOK, api.DiscoverResponse{Profiles: profiles})
}
