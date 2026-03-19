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

type Server struct {
	hub      *Hub
	gh       *GitHubCache
	poolURL  string
	salt     string
	upgrader websocket.Upgrader
}

type ServerConfig struct {
	PoolURL    string
	Salt       string
	RawBaseURL string // override for testing
}

func NewServer(cfg ServerConfig) *Server {
	baseURL := cfg.RawBaseURL
	if baseURL == "" {
		baseURL = "https://raw.githubusercontent.com/" + cfg.PoolURL + "/main"
	}
	return &Server{
		hub:     NewHub(),
		gh:      NewGitHubCache(baseURL),
		poolURL: cfg.PoolURL,
		salt:    cfg.Salt,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) GitHubCache() *GitHubCache { return s.gh }

func (s *Server) BinHash(idHash string) string {
	h := sha256.Sum256([]byte(s.salt + ":" + idHash))
	return hex.EncodeToString(h[:])[:16]
}

func (s *Server) MatchHash(binHash string) string {
	h := sha256.Sum256([]byte(s.salt + ":" + binHash))
	return hex.EncodeToString(h[:])[:16]
}

func PairHash(matchHashA, matchHashB string) string {
	a, b := matchHashA, matchHashB
	if a > b {
		a, b = b, a
	}
	h := sha256.Sum256([]byte(a + ":" + b))
	return hex.EncodeToString(h[:])[:12]
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", s.HandleWS)
	mux.HandleFunc("GET /health", s.HandleHealth)
	return mux
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	idHash := r.URL.Query().Get("id")
	matchHash := r.URL.Query().Get("match")
	sigHex := r.URL.Query().Get("sig")

	if idHash == "" || matchHash == "" || sigHex == "" {
		http.Error(w, "missing id, match, or sig", http.StatusUnauthorized)
		return
	}

	binHash := s.BinHash(idHash)
	expectedMatch := s.MatchHash(binHash)
	if expectedMatch != matchHash {
		http.Error(w, "invalid identity", http.StatusUnauthorized)
		return
	}

	pubkey, err := s.gh.FetchPubkey(binHash)
	if err != nil {
		if err.Error() == "unknown user: "+binHash {
			http.Error(w, "unknown user", http.StatusUnauthorized)
		} else {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}
		return
	}

	if !crypto.TOTPVerify(sigHex, pubkey) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	sess := newSession(conn, s, matchHash)
	go sess.run()
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"online": s.hub.OnlineCount(),
		"queued": s.hub.QueuedCount(),
	})
}

func (s *Server) ListenAndServe(addr string) error {
	log.Printf("relay server listening on %s (pool=%s)", addr, s.poolURL)
	return http.ListenAndServe(addr, s.Handler())
}

func Addr(port string) string {
	if port == "" {
		port = "8081"
	}
	return fmt.Sprintf(":%s", port)
}
