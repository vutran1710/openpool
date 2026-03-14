package relay

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

type Server struct {
	hub       *Hub
	poolToken string
	upgrader  websocket.Upgrader
}

func NewServer() *Server {
	return &Server{
		hub:       NewHub(),
		poolToken: os.Getenv("POOL_TOKEN"),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	client := NewClient(conn, s.hub, s.poolToken)
	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","online":` + itoa(s.hub.OnlineCount()) + `}`))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
