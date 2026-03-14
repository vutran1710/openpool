package relay

import (
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.UserHash] = c
	log.Printf("registered: %s", c.UserHash[:8])
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.clients[c.UserHash]; ok && existing == c {
		delete(h.clients, c.UserHash)
		log.Printf("unregistered: %s", c.UserHash[:8])
	}
}

func (h *Hub) Send(targetHash string, msg OutboundMessage) {
	h.mu.RLock()
	client, ok := h.clients[targetHash]
	h.mu.RUnlock()

	if !ok {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case client.send <- data:
	default:
		h.Unregister(client)
	}
}

func (h *Hub) IsOnline(userHash string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userHash]
	return ok
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
