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
type Session struct {
	conn    *websocket.Conn
	server  *Server
	writeMu sync.Mutex

	// Set after authentication
	hashID   string
	userID   string
	provider string

	// Auth state machine
	nonce  []byte
	authed bool
}

// newSession creates a session for an incoming connection.
func newSession(conn *websocket.Conn, srv *Server) *Session {
	return &Session{
		conn:   conn,
		server: srv,
	}
}

// run is the main read loop for the session.
func (s *Session) run() {
	defer func() {
		if s.hashID != "" {
			s.server.hub.Unregister(s.hashID, s)
		}
		s.conn.Close()
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
	case *protocol.AuthRequest:
		s.handleAuth(f)
	case *protocol.AuthResponse:
		s.handleAuthResponse(f)
	case *protocol.RefreshRequest:
		s.handleRefresh(f)
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

func (s *Session) handleAuth(req *protocol.AuthRequest) {
	if req.UserID == "" || req.Provider == "" {
		s.sendError(protocol.ErrInternal, "missing auth fields")
		return
	}

	// Look up user in store (pool is implicit — server is per-pool)
	entry := s.server.store.LookupUser(req.UserID, req.Provider)
	if entry == nil {
		s.sendError(protocol.ErrUserNotFound, "User not registered in this pool")
		return
	}

	// Generate nonce challenge
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		s.sendError(protocol.ErrInternal, "failed to generate nonce")
		return
	}

	s.nonce = nonce
	s.userID = req.UserID
	s.provider = req.Provider

	s.sendFrame(protocol.Challenge{
		Type:  protocol.TypeChallenge,
		Nonce: hex.EncodeToString(nonce),
	})
}

func (s *Session) handleAuthResponse(resp *protocol.AuthResponse) {
	if s.nonce == nil {
		s.sendError(protocol.ErrAuthFailed, "no pending challenge")
		return
	}

	entry := s.server.store.LookupUser(s.userID, s.provider)
	if entry == nil {
		s.sendError(protocol.ErrUserNotFound, "user disappeared during auth")
		return
	}

	// Verify signature
	sigBytes, err := hex.DecodeString(resp.Signature)
	if err != nil {
		s.sendError(protocol.ErrAuthFailed, "invalid signature encoding")
		return
	}

	if !verifyEd25519(entry.PubKey, s.nonce, sigBytes) {
		s.sendError(protocol.ErrAuthFailed, "Signature verification failed")
		return
	}

	// Authentication successful
	s.nonce = nil
	s.authed = true
	s.hashID = entry.HashID

	// Issue token
	token, expiresAt := s.server.tokens.Issue(entry.HashID)

	// Register in hub
	s.server.hub.Register(s.hashID, s)

	s.sendFrame(protocol.Authenticated{
		Type:      protocol.TypeAuthenticated,
		Token:     token,
		ExpiresAt: expiresAt,
		HashID:    entry.HashID,
		PoolURL:   s.server.poolURL,
	})

	log.Printf("session: authenticated %s (hash=%s)", s.userID, s.hashID)
}

func (s *Session) handleRefresh(req *protocol.RefreshRequest) {
	hashID, ok := s.server.tokens.Validate(req.Token)
	if !ok {
		s.sendError(protocol.ErrTokenExpired, "token expired or invalid")
		return
	}

	// Revoke old, issue new
	s.server.tokens.Revoke(req.Token)
	newToken, expiresAt := s.server.tokens.Issue(hashID)

	s.sendFrame(protocol.Authenticated{
		Type:      protocol.TypeAuthenticated,
		Token:     newToken,
		ExpiresAt: expiresAt,
		HashID:    hashID,
		PoolURL:   s.server.poolURL,
	})
}

func (s *Session) handleMessage(msg *protocol.Message) {
	if !s.authed {
		s.sendError(protocol.ErrTokenInvalid, "not authenticated")
		return
	}

	// Validate token
	hashID, ok := s.server.tokens.Validate(msg.Token)
	if !ok {
		s.sendError(protocol.ErrTokenExpired, "token expired")
		return
	}

	// Verify sender matches token
	if hashID != msg.SourceHash {
		s.sendError(protocol.ErrAuthFailed, "token does not match source_hash")
		return
	}

	// Check match
	if !s.server.store.IsMatched(msg.SourceHash, msg.TargetHash) {
		s.sendError(protocol.ErrNotMatched, "users are not matched")
		return
	}

	// Generate msg_id
	msgID := generateMsgID()

	// Route to target — server populates PoolURL
	targetSession := s.server.hub.Lookup(msg.TargetHash)
	if targetSession != nil {
		outMsg := protocol.Message{
			Type:       protocol.TypeMsg,
			MsgID:      msgID,
			SourceHash: msg.SourceHash,
			TargetHash: msg.TargetHash,
			PoolURL:    s.server.poolURL,
			Body:       msg.Body,
			Ts:         msg.Ts,
			Encrypted:  msg.Encrypted,
		}
		targetSession.sendFrame(outMsg)
	}
	// TODO: queue for offline delivery
}

func (s *Session) handleKeyRequest(req *protocol.KeyRequest) {
	if !s.authed {
		s.sendError(protocol.ErrTokenInvalid, "not authenticated")
		return
	}

	if req.TargetHash == "" {
		s.sendError(protocol.ErrInternal, "missing target_hash")
		return
	}

	// Verify requester is matched with target
	if !s.server.store.IsMatched(s.hashID, req.TargetHash) {
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
	if !s.authed {
		s.sendError(protocol.ErrTokenInvalid, "not authenticated")
		return
	}
	// TODO: mark message as acked in persistent store
	log.Printf("session: ack %s from %s", ack.MsgID, s.hashID)
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
