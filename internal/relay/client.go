package relay

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeTimeout = 10 * time.Second
	pongTimeout  = 60 * time.Second
	pingInterval = 30 * time.Second
	sendBufSize  = 256
)

type Client struct {
	conn      *websocket.Conn
	hub       *Hub
	send      chan []byte
	UserHash  string
	pubKey    ed25519.PublicKey
	PoolRepo  string
	PoolToken string
	authed    bool
	nonce     []byte
}

func NewClient(conn *websocket.Conn, hub *Hub, poolRepo, poolToken string) *Client {
	return &Client{
		conn:      conn,
		hub:       hub,
		send:      make(chan []byte, sendBufSize),
		PoolRepo:  poolRepo,
		PoolToken: poolToken,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var frame InboundFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			c.sendError("invalid frame")
			continue
		}

		c.handleFrame(frame)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, msg)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleFrame(f InboundFrame) {
	switch f.Type {
	case "auth":
		c.handleAuth(f)
	case "auth_response":
		c.handleAuthResponse(f)
	case "msg":
		c.handleMsg(f)
	default:
		c.sendError("unknown frame type")
	}
}

func (c *Client) handleAuth(f InboundFrame) {
	if f.UserHash == "" {
		c.sendError("missing user_hash")
		return
	}

	bin, err := FetchUserBin(c.PoolRepo, c.PoolToken, f.UserHash)
	if err != nil {
		c.sendError("user not registered in pool")
		return
	}

	pubKey, err := ExtractPubKey(bin)
	if err != nil {
		c.sendError("invalid user data")
		return
	}

	nonce, err := GenerateNonce()
	if err != nil {
		c.sendError("internal error")
		return
	}

	c.UserHash = f.UserHash
	c.pubKey = pubKey
	c.nonce = nonce

	c.sendFrame(OutboundMessage{
		Type:  "challenge",
		Nonce: hex.EncodeToString(nonce),
	})
}

func (c *Client) handleAuthResponse(f InboundFrame) {
	if c.nonce == nil || c.pubKey == nil {
		c.sendError("no pending challenge")
		return
	}

	if f.Signature == "" {
		c.sendError("missing signature")
		return
	}

	if !VerifySignature(c.pubKey, c.nonce, f.Signature) {
		c.sendError("authentication failed")
		return
	}

	c.authed = true
	c.nonce = nil
	c.hub.Register(c)

	c.sendFrame(OutboundMessage{Type: "authenticated"})
	log.Printf("authenticated: %s", c.UserHash[:8])
}

func (c *Client) handleMsg(f InboundFrame) {
	if !c.authed {
		c.sendError("not authenticated")
		return
	}

	if f.To == "" || f.Body == "" {
		c.sendError("missing to or body")
		return
	}

	if !MatchExists(c.PoolRepo, c.PoolToken, c.UserHash, f.To) {
		c.sendError("no match with target")
		return
	}

	c.hub.Send(f.To, OutboundMessage{
		Type: "msg",
		From: c.UserHash,
		Body: f.Body,
		Ts:   time.Now().Unix(),
	})
}

func (c *Client) sendFrame(msg OutboundMessage) {
	data, _ := json.Marshal(msg)
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) sendError(msg string) {
	c.sendFrame(OutboundMessage{Type: "error", Error: msg})
}
