package relay

import (
	"log"
	"sync"
)

// Hub manages active WebSocket connections keyed by hash_id.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*Session // hash_id → session
}

// NewHub creates an empty connection hub.
func NewHub() *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
	}
}

// Register adds a session to the hub, replacing any existing session for the same hash_id.
func (h *Hub) Register(hashID string, s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.sessions[hashID]; ok && old != s {
		old.conn.Close()
	}
	h.sessions[hashID] = s
	log.Printf("hub: registered %s", hashID)
}

// Unregister removes a session (only if it matches the current one).
func (h *Hub) Unregister(hashID string, s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.sessions[hashID]; ok && existing == s {
		delete(h.sessions, hashID)
		log.Printf("hub: unregistered %s", hashID)
	}
}

// Lookup returns the session for the given hash_id, or nil if offline.
func (h *Hub) Lookup(hashID string) *Session {
	h.mu.RLock()
	s, ok := h.sessions[hashID]
	h.mu.RUnlock()
	if !ok {
		return nil
	}
	return s
}

// IsOnline checks if a hash_id has an active connection.
func (h *Hub) IsOnline(hashID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.sessions[hashID]
	return ok
}

// OnlineCount returns the number of active connections.
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}
