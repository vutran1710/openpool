package relay

import (
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	matchHashRawLen = 8    // 16 hex chars = 8 bytes
	maxFrameSize    = 8192 // 8 KB — encrypted chat + overhead
)

type Session struct {
	conn      *websocket.Conn
	server    *Server
	writeMu   sync.Mutex
	matchHash string
}

func newSession(conn *websocket.Conn, srv *Server, matchHash string) *Session {
	return &Session{
		conn:      conn,
		server:    srv,
		matchHash: matchHash,
	}
}

func (s *Session) run() {
	s.server.hub.Register(s.matchHash, s)
	log.Printf("session: connected %s", s.matchHash)

	defer func() {
		s.server.hub.Unregister(s.matchHash, s)
		s.conn.Close()
		log.Printf("session: disconnected %s", s.matchHash)
	}()

	for {
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}
		if len(data) > maxFrameSize {
			s.sendText("frame too large")
			continue
		}
		if len(data) <= matchHashRawLen {
			s.sendText("invalid frame")
			continue
		}

		targetMatchHash := hex.EncodeToString(data[:matchHashRawLen])
		ciphertext := data[matchHashRawLen:]

		pairHash := PairHash(s.matchHash, targetMatchHash)
		matched, err := s.server.gh.CheckMatch(pairHash)
		if err != nil {
			s.sendText("match check failed")
			continue
		}
		if !matched {
			s.sendText("not matched")
			continue
		}

		senderBytes, _ := hex.DecodeString(s.matchHash)
		forwarded := append(senderBytes, ciphertext...)

		status := s.server.hub.Send(targetMatchHash, forwarded)
		if status != "" {
			s.sendText(status)
		}
	}
}

func (s *Session) sendText(msg string) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	s.conn.WriteMessage(websocket.TextMessage, []byte(msg))
}
