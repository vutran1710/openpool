package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Server is the relay WebSocket + HTTP server.
type Server struct {
	hub      *Hub
	tokens   *TokenStore
	store    *Store
	upgrader websocket.Upgrader
}

// ServerConfig holds relay server configuration.
type ServerConfig struct {
	TokenTTL time.Duration
}

// NewServer creates a relay server with the given config.
func NewServer(cfg ServerConfig) *Server {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 15 * time.Minute
	}
	return &Server{
		hub:    NewHub(),
		tokens: NewTokenStore(cfg.TokenTTL),
		store:  NewStore(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Store returns the server's user/match store for seeding data.
func (s *Server) Store() *Store {
	return s.store
}

// Handler returns an http.Handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.HandleWS)
	mux.HandleFunc("GET /health", s.HandleHealth)
	return mux
}

// HandleWS upgrades to WebSocket and starts a session.
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	sess := newSession(conn, s)
	go sess.run()
}

// HandleHealth returns server status.
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"online": s.hub.OnlineCount(),
	})
}

// ListenAndServe starts the relay server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("relay server listening on %s", addr)
	return http.ListenAndServe(addr, s.Handler())
}

// Addr returns the listen address string for a given port.
func Addr(port string) string {
	if port == "" {
		port = "8081"
	}
	return fmt.Sprintf(":%s", port)
}
