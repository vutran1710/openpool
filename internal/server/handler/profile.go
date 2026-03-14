package handler

import (
	"net/http"

	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
)

type ProfileHandler struct {
	store *store.Store
}

func NewProfileHandler(s *store.Store) *ProfileHandler {
	return &ProfileHandler{store: s}
}

func (h *ProfileHandler) View(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("public_id")

	profile, err := h.store.GetProfileByPublicID(r.Context(), publicID)
	if err != nil {
		http.Error(w, `{"error":"profile_not_found"}`, http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Bio        string   `json:"bio"`
		City       string   `json:"city"`
		Interests  []string `json:"interests"`
		LookingFor string   `json:"looking_for"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateProfileIndex(r.Context(), userID, req.Bio, req.City, req.Interests, req.LookingFor); err != nil {
		http.Error(w, `{"error":"update_failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
