package relay

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

const maxQueueSize = 20

type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*Session // match_hash → session
	queue    map[string][][]byte // match_hash → pending binary messages
}

func NewHub() *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
		queue:    make(map[string][][]byte),
	}
}

// Register adds a session, closing any existing one for the same match_hash.
// Flushes queued messages to the new session.
func (h *Hub) Register(matchHash string, s *Session) {
	h.mu.Lock()
	if old, ok := h.sessions[matchHash]; ok && old != s {
		old.conn.Close()
	}
	h.sessions[matchHash] = s
	queued := h.queue[matchHash]
	delete(h.queue, matchHash)
	h.mu.Unlock()

	for _, msg := range queued {
		s.writeMu.Lock()
		s.conn.WriteMessage(websocket.BinaryMessage, msg)
		s.writeMu.Unlock()
	}
	log.Printf("hub: registered %s (flushed %d queued)", matchHash, len(queued))
}

func (h *Hub) Unregister(matchHash string, s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.sessions[matchHash]; ok && existing == s {
		delete(h.sessions, matchHash)
		log.Printf("hub: unregistered %s", matchHash)
	}
}

// Send routes a binary message to a target. Returns "" if delivered,
// "queued" if offline, "queue full" if at capacity.
func (h *Hub) Send(targetMatchHash string, data []byte) string {
	h.mu.RLock()
	s, online := h.sessions[targetMatchHash]
	h.mu.RUnlock()

	if online {
		s.writeMu.Lock()
		s.conn.WriteMessage(websocket.BinaryMessage, data)
		s.writeMu.Unlock()
		return ""
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	q := h.queue[targetMatchHash]
	if len(q) >= maxQueueSize {
		return "queue full"
	}
	h.queue[targetMatchHash] = append(q, data)
	return "queued"
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

func (h *Hub) QueuedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, q := range h.queue {
		total += len(q)
	}
	return total
}
