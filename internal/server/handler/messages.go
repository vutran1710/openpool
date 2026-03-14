package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
)

type MessagesHandler struct {
	store *store.Store
}

func NewMessagesHandler(s *store.Store) *MessagesHandler {
	return &MessagesHandler{store: s}
}

func (h *MessagesHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	convIDStr := r.PathValue("conv_id")

	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid_conversation_id"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.ValidateConversationAccess(r.Context(), userID, convID); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = &t
		}
	}

	messages, err := h.store.GetMessages(r.Context(), convID, before, limit)
	if err != nil {
		http.Error(w, `{"error":"fetch_failed"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, api.MessagesResponse{
		Messages: messages,
		HasMore:  len(messages) == limit,
	})
}

func (h *MessagesHandler) Send(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	convIDStr := r.PathValue("conv_id")

	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid_conversation_id"}`, http.StatusBadRequest)
		return
	}

	if err := h.store.ValidateConversationAccess(r.Context(), userID, convID); err != nil {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Body == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	msg, err := h.store.CreateMessage(r.Context(), convID, userID, req.Body)
	if err != nil {
		http.Error(w, `{"error":"send_failed"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}
