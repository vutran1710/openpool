package relay

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

// Session handles a single WebSocket connection's lifecycle.
// TOTP auth is already verified at upgrade — no auth frames needed.
type Session struct {
	conn    *websocket.Conn
	server  *Server
	writeMu sync.Mutex
	binHash string
}

// newSession creates a session for an authenticated connection.
func newSession(conn *websocket.Conn, srv *Server, binHash string) *Session {
	return &Session{
		conn:    conn,
		server:  srv,
		binHash: binHash,
	}
}

// run is the main read loop for the session.
func (s *Session) run() {
	s.server.hub.Register(s.binHash, s)
	log.Printf("session: connected %s", s.binHash)

	defer func() {
		s.server.hub.Unregister(s.binHash, s)
		s.conn.Close()
		log.Printf("session: disconnected %s", s.binHash)
	}()

	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		frame, err := protocol.DecodeFrame(data)
		if err != nil {
			s.sendError(protocol.ErrInternal, "invalid frame")
			continue
		}

		s.handleFrame(frame)
	}
}

func (s *Session) handleFrame(frame any) {
	switch f := frame.(type) {
	case *protocol.Message:
		s.handleMessage(f)
	case *protocol.Ack:
		s.handleAck(f)
	case *protocol.KeyRequest:
		s.handleKeyRequest(f)
	default:
		s.sendError(protocol.ErrInternal, fmt.Sprintf("unexpected frame type: %T", frame))
	}
}

func (s *Session) handleMessage(msg *protocol.Message) {
	// Verify sender matches session
	if msg.SourceHash != s.binHash {
		s.sendError(protocol.ErrAuthFailed, "source_hash does not match session")
		return
	}

	// Check match
	if !s.server.store.IsMatched(msg.SourceHash, msg.TargetHash) {
		s.sendError(protocol.ErrNotMatched, "users are not matched")
		return
	}

	// Generate msg_id
	msgID := generateMsgID()

	// Route to target
	targetSession := s.server.hub.Lookup(msg.TargetHash)
	if targetSession != nil {
		outMsg := protocol.Message{
			Type:       protocol.TypeMsg,
			MsgID:      msgID,
			SourceHash: msg.SourceHash,
			TargetHash: msg.TargetHash,
			Body:       msg.Body,
			Ts:         msg.Ts,
			Encrypted:  msg.Encrypted,
		}
		targetSession.sendFrame(outMsg)
	}
	// TODO: queue for offline delivery
}

func (s *Session) handleKeyRequest(req *protocol.KeyRequest) {
	if req.TargetHash == "" {
		s.sendError(protocol.ErrInternal, "missing target_hash")
		return
	}

	// Verify requester is matched with target
	if !s.server.store.IsMatched(s.binHash, req.TargetHash) {
		s.sendError(protocol.ErrNotMatched, "not matched with target")
		return
	}

	pubkey := s.server.store.PubKeyByHash(req.TargetHash)
	if pubkey == nil {
		s.sendError(protocol.ErrUserNotFound, "target not found")
		return
	}

	s.sendFrame(protocol.KeyResponse{
		Type:       protocol.TypeKeyResponse,
		TargetHash: req.TargetHash,
		PubKey:     hex.EncodeToString(pubkey),
	})
}

func (s *Session) handleAck(ack *protocol.Ack) {
	// TODO: mark message as acked in persistent store
	log.Printf("session: ack %s from %s", ack.MsgID, s.binHash)
}

func (s *Session) sendFrame(v any) {
	data, err := protocol.Encode(v)
	if err != nil {
		log.Printf("session: encode error: %v", err)
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *Session) sendError(code, message string) {
	s.sendFrame(protocol.Error{
		Type:    protocol.TypeError,
		Code:    code,
		Message: message,
	})
}

func generateMsgID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
