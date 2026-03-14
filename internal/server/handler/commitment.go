package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
)

type CommitmentHandler struct {
	store *store.Store
}

func NewCommitmentHandler(s *store.Store) *CommitmentHandler {
	return &CommitmentHandler{store: s}
}

func (h *CommitmentHandler) Propose(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req api.CommitmentRequest
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	matches, err := h.store.GetMatches(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	var accepterPublicID string
	for _, m := range matches {
		if m.MatchID == req.MatchID {
			accepterPublicID = m.WithUser.PublicID
			break
		}
	}
	if accepterPublicID == "" {
		http.Error(w, `{"error":"match_not_found"}`, http.StatusNotFound)
		return
	}

	accepter, err := h.store.GetUserByPublicID(r.Context(), accepterPublicID)
	if err != nil {
		http.Error(w, `{"error":"user_not_found"}`, http.StatusNotFound)
		return
	}

	commitment, err := h.store.CreateCommitment(r.Context(), req.MatchID, userID, accepter.ID)
	if err != nil {
		http.Error(w, `{"error":"commitment_failed"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, api.CommitmentResponse{Commitment: *commitment})
}

func (h *CommitmentHandler) Accept(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	commitmentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid_id"}`, http.StatusBadRequest)
		return
	}

	commitment, err := h.store.AcceptCommitment(r.Context(), commitmentID, userID)
	if err != nil {
		http.Error(w, `{"error":"accept_failed"}`, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, api.CommitmentResponse{Commitment: *commitment})
}

func (h *CommitmentHandler) Decline(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	commitmentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid_id"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.DeclineCommitment(r.Context(), commitmentID, userID); err != nil {
		http.Error(w, `{"error":"decline_failed"}`, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CommitmentHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	commitment, err := h.store.GetActiveCommitment(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"commitment": nil})
		return
	}

	writeJSON(w, http.StatusOK, api.CommitmentResponse{Commitment: *commitment})
}
