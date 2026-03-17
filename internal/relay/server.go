package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

// Server is the relay WebSocket + HTTP server.
type Server struct {
	hub      *Hub
	store    *Store
	poolURL  string
	salt     string
	upgrader websocket.Upgrader
}

// ServerConfig holds relay server configuration.
type ServerConfig struct {
	PoolURL string
	Salt    string
}

// NewServer creates a relay server with the given config.
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		hub:     NewHub(),
		store:   NewStore(),
		poolURL: cfg.PoolURL,
		salt:    cfg.Salt,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Store returns the server's user/match store for seeding data.
func (s *Server) Store() *Store {
	return s.store
}

// BinHash computes the bin_hash from an id_hash using the server's salt.
func (s *Server) BinHash(idHash string) string {
	h := sha256.Sum256([]byte(s.salt + ":" + idHash))
	return hex.EncodeToString(h[:])[:16]
}

// MatchHash computes the match_hash from a bin_hash using the server's salt.
func (s *Server) MatchHash(binHash string) string {
	h := sha256.Sum256([]byte(s.salt + ":" + binHash))
	return hex.EncodeToString(h[:])[:16]
}

// Handler returns an http.Handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.HandleWS)
	mux.HandleFunc("GET /health", s.HandleHealth)
	return mux
}

// HandleWS verifies TOTP signature, upgrades to WebSocket, and starts a session.
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	binHash := r.URL.Query().Get("bin")
	sigHex := r.URL.Query().Get("sig")

	if binHash == "" || sigHex == "" {
		http.Error(w, "missing bin or sig", http.StatusUnauthorized)
		return
	}

	entry := s.store.LookupByHash(binHash)
	if entry == nil {
		http.Error(w, "unknown user", http.StatusUnauthorized)
		return
	}

	if !crypto.TOTPVerify(binHash, entry.MatchHash, sigHex, entry.PubKey) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	sess := newSession(conn, s, binHash)
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
	log.Printf("relay server listening on %s (pool=%s)", addr, s.poolURL)
	return http.ListenAndServe(addr, s.Handler())
}

// Addr returns the listen address string for a given port.
func Addr(port string) string {
	if port == "" {
		port = "8081"
	}
	return fmt.Sprintf(":%s", port)
}
