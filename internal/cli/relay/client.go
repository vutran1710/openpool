// Package relay provides a WebSocket client for the dating relay server.
// Uses TOTP signature at WebSocket upgrade for authentication.
package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

// Client is a relay WebSocket client.
type Client struct {
	conn      *websocket.Conn
	url       string // relay base URL
	binHash   string
	matchHash string
	poolURL   string
	pub       ed25519.PublicKey
	priv      ed25519.PrivateKey

	mu       sync.Mutex
	onMsg    func(protocol.Message)
	onError  func(protocol.Error)
	onRawMsg func(protocol.Message)
	done     chan struct{}
	closed   bool

	keys    map[string][]byte                // target_hash → conversation key
	keyReqs map[string]chan ed25519.PublicKey // inflight key request channels
}

// Config holds the parameters needed to connect.
type Config struct {
	RelayURL  string
	PoolURL   string // for conversation key derivation
	BinHash   string // from registration
	MatchHash string // from registration (TOTP secret)
	Pub       ed25519.PublicKey
	Priv      ed25519.PrivateKey
}

// NewClient creates a relay client (not yet connected).
func NewClient(cfg Config) *Client {
	return &Client{
		url:       cfg.RelayURL,
		poolURL:   cfg.PoolURL,
		binHash:   cfg.BinHash,
		matchHash: cfg.MatchHash,
		pub:       cfg.Pub,
		priv:      cfg.Priv,
		done:      make(chan struct{}),
		keys:      make(map[string][]byte),
		keyReqs:   make(map[string]chan ed25519.PublicKey),
	}
}

// Connect computes a TOTP signature and dials the relay WebSocket.
func (c *Client) Connect(ctx context.Context) error {
	sig := crypto.TOTPSign(c.binHash, c.matchHash, c.priv)
	wsURL := c.wsURL() + "/ws?bin=" + c.binHash + "&sig=" + sig

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 401 {
			return fmt.Errorf("authentication failed")
		}
		return fmt.Errorf("connecting to relay: %w", err)
	}
	c.conn = conn
	go c.readLoop()
	return nil
}

// Close shuts down the connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
}

// BinHash returns the client's bin_hash.
func (c *Client) BinHash() string {
	return c.binHash
}

// OnMessage sets the callback for incoming messages.
func (c *Client) OnMessage(fn func(protocol.Message)) {
	c.onMsg = fn
}

// OnError sets the callback for error frames.
func (c *Client) OnError(fn func(protocol.Error)) {
	c.onError = fn
}

// OnRawMessage sets a callback that fires before decryption (for testing).
func (c *Client) OnRawMessage(fn func(protocol.Message)) {
	c.onRawMsg = fn
}

// RequestKey requests a peer's ed25519 public key from the relay.
func (c *Client) RequestKey(ctx context.Context, targetHash string) (ed25519.PublicKey, error) {
	c.mu.Lock()
	ch, exists := c.keyReqs[targetHash]
	if !exists {
		ch = make(chan ed25519.PublicKey, 1)
		c.keyReqs[targetHash] = ch
	}
	c.mu.Unlock()

	if !exists {
		if err := c.send(protocol.KeyRequest{
			Type:       protocol.TypeKeyRequest,
			TargetHash: targetHash,
		}); err != nil {
			c.mu.Lock()
			delete(c.keyReqs, targetHash)
			c.mu.Unlock()
			return nil, fmt.Errorf("sending key request: %w", err)
		}
	}

	select {
	case pub := <-ch:
		return pub, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.keyReqs, targetHash)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (c *Client) getOrDeriveKey(ctx context.Context, targetHash string) ([]byte, error) {
	c.mu.Lock()
	key, ok := c.keys[targetHash]
	c.mu.Unlock()
	if ok {
		return key, nil
	}

	peerPub, err := c.RequestKey(ctx, targetHash)
	if err != nil {
		return nil, fmt.Errorf("requesting peer key: %w", err)
	}

	shared, err := crypto.SharedSecret(c.priv, peerPub)
	if err != nil {
		return nil, fmt.Errorf("computing shared secret: %w", err)
	}

	key, err = crypto.DeriveConversationKey(shared, c.binHash, targetHash, c.poolURL)
	if err != nil {
		return nil, fmt.Errorf("deriving conversation key: %w", err)
	}

	c.mu.Lock()
	c.keys[targetHash] = key
	c.mu.Unlock()

	return key, nil
}

// SendMessage sends an encrypted chat message to a target user.
func (c *Client) SendMessage(targetHash, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key, err := c.getOrDeriveKey(ctx, targetHash)
	if err != nil {
		return fmt.Errorf("deriving key: %w", err)
	}

	encBody, err := crypto.SealMessage(key, body)
	if err != nil {
		return fmt.Errorf("encrypting message: %w", err)
	}

	msg := protocol.Message{
		Type:       protocol.TypeMsg,
		SourceHash: c.binHash,
		TargetHash: targetHash,
		Body:       encBody,
		Encrypted:  true,
		Ts:         time.Now().Unix(),
	}
	return c.send(msg)
}

// SendMessagePlain sends a plaintext (unencrypted) chat message.
func (c *Client) SendMessagePlain(targetHash, body string) error {
	msg := protocol.Message{
		Type:       protocol.TypeMsg,
		SourceHash: c.binHash,
		TargetHash: targetHash,
		Body:       body,
		Ts:         time.Now().Unix(),
	}
	return c.send(msg)
}

// AckMessage acknowledges a received message.
func (c *Client) AckMessage(msgID string) error {
	return c.send(protocol.Ack{
		Type:  protocol.TypeAck,
		MsgID: msgID,
	})
}

// --- Internal ---

// wsURL returns the WebSocket base URL.
func (c *Client) wsURL() string {
	u := c.url
	u = strings.TrimSuffix(u, "/")
	u = strings.Replace(u, "http://", "ws://", 1)
	u = strings.Replace(u, "https://", "wss://", 1)
	return u
}

func (c *Client) send(v any) error {
	data, err := protocol.Encode(v)
	if err != nil {
		return fmt.Errorf("encoding frame: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		frame, err := protocol.DecodeFrame(data)
		if err != nil {
			continue
		}

		switch f := frame.(type) {
		case *protocol.KeyResponse:
			pubBytes, err := hex.DecodeString(f.PubKey)
			if err == nil && len(pubBytes) == ed25519.PublicKeySize {
				c.mu.Lock()
				ch, ok := c.keyReqs[f.TargetHash]
				if ok {
					select {
					case ch <- ed25519.PublicKey(pubBytes):
					default:
					}
					delete(c.keyReqs, f.TargetHash)
				}
				c.mu.Unlock()
			}
		case *protocol.Message:
			if c.onRawMsg != nil {
				c.onRawMsg(*f)
			}
			go func(msg protocol.Message) {
				if msg.Encrypted {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					key, err := c.getOrDeriveKey(ctx, msg.SourceHash)
					cancel()
					if err == nil {
						if plain, err := crypto.OpenMessage(key, msg.Body); err == nil {
							msg.Body = plain
							msg.Encrypted = false
						}
					}
				}
				if c.onMsg != nil {
					c.onMsg(msg)
				}
			}(*f)
		case *protocol.Error:
			if c.onError != nil {
				c.onError(*f)
			}
		}
	}
}

// generateNonce creates a random 32-byte hex string (for testing).
func generateNonce() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
