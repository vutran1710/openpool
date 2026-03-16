// Package relay provides a WebSocket client for the dating relay server.
// Uses MessagePack-encoded frames as defined in internal/protocol.
package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

// Client is a relay WebSocket client.
type Client struct {
	conn     *websocket.Conn
	url      string
	token    string
	hashID   string
	poolURL  string
	userID   string
	provider string
	pub      ed25519.PublicKey
	priv     ed25519.PrivateKey

	mu       sync.Mutex
	onMsg    func(protocol.Message)
	onError  func(protocol.Error)
	done     chan struct{}
	closed   bool

	keys     map[string][]byte                // target_hash → conversation key
	keyReqs  map[string]chan ed25519.PublicKey // inflight key request channels
	onRawMsg func(protocol.Message)
}

// Config holds the parameters needed to connect.
type Config struct {
	RelayURL string
	PoolURL  string
	UserID   string
	Provider string
	Pub      ed25519.PublicKey
	Priv     ed25519.PrivateKey
}

// NewClient creates a relay client (not yet connected).
func NewClient(cfg Config) *Client {
	return &Client{
		url:      cfg.RelayURL,
		poolURL:  cfg.PoolURL,
		userID:   cfg.UserID,
		provider: cfg.Provider,
		pub:      cfg.Pub,
		priv:     cfg.Priv,
		done:     make(chan struct{}),
		keys:     make(map[string][]byte),
		keyReqs:  make(map[string]chan ed25519.PublicKey),
	}
}

// Connect establishes the WebSocket connection and authenticates.
func (c *Client) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.url+"/ws", nil)
	if err != nil {
		return fmt.Errorf("connecting to relay: %w", err)
	}
	c.conn = conn

	// Authenticate
	if err := c.authenticate(ctx); err != nil {
		c.conn.Close()
		return fmt.Errorf("authentication: %w", err)
	}

	// Start read loop
	go c.readLoop()

	// Start token refresh loop
	go c.refreshLoop(ctx)

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

// HashID returns the authenticated user's hash_id.
func (c *Client) HashID() string {
	return c.hashID
}

// Token returns the current auth token.
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
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

	key, err = crypto.DeriveConversationKey(shared, c.hashID, targetHash, c.poolURL)
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
		Token:      c.Token(),
		SourceHash: c.hashID,
		TargetHash: targetHash,
		PoolURL:    c.poolURL,
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
		Token:      c.Token(),
		SourceHash: c.hashID,
		TargetHash: targetHash,
		PoolURL:    c.poolURL,
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

// QueryIdentity asks the relay for the user's hash_id in a pool.
func (c *Client) QueryIdentity(ctx context.Context, poolURL string) (string, error) {
	if err := c.send(protocol.IdentityRequest{
		Type:    protocol.TypeIdentity,
		PoolURL: poolURL,
	}); err != nil {
		return "", err
	}

	// Wait for response (blocking read with timeout)
	data, err := c.readFrame(ctx)
	if err != nil {
		return "", err
	}

	frame, err := protocol.DecodeFrame(data)
	if err != nil {
		return "", err
	}

	switch f := frame.(type) {
	case *protocol.IdentityResponse:
		return f.HashID, nil
	case *protocol.Error:
		return "", fmt.Errorf("%s: %s", f.Code, f.Message)
	default:
		return "", fmt.Errorf("unexpected response type: %T", frame)
	}
}

// --- Internal ---

func (c *Client) authenticate(ctx context.Context) error {
	// Step 1: Send auth request
	if err := c.send(protocol.AuthRequest{
		Type:     protocol.TypeAuth,
		UserID:   c.userID,
		Provider: c.provider,
		PoolURL:  c.poolURL,
	}); err != nil {
		return fmt.Errorf("sending auth: %w", err)
	}

	// Step 2: Read challenge
	data, err := c.readFrame(ctx)
	if err != nil {
		return fmt.Errorf("reading challenge: %w", err)
	}

	frame, err := protocol.DecodeFrame(data)
	if err != nil {
		return err
	}

	switch f := frame.(type) {
	case *protocol.Challenge:
		// Step 3: Sign nonce and respond
		nonceBytes, err := hex.DecodeString(f.Nonce)
		if err != nil {
			return fmt.Errorf("decoding nonce: %w", err)
		}
		signature := crypto.Sign(c.priv, nonceBytes)

		if err := c.send(protocol.AuthResponse{
			Type:      protocol.TypeAuthResponse,
			Signature: signature,
		}); err != nil {
			return fmt.Errorf("sending signature: %w", err)
		}

	case *protocol.Error:
		return fmt.Errorf("%s: %s", f.Code, f.Message)
	default:
		return fmt.Errorf("expected challenge, got %T", frame)
	}

	// Step 4: Read authenticated response
	data, err = c.readFrame(ctx)
	if err != nil {
		return fmt.Errorf("reading auth result: %w", err)
	}

	frame, err = protocol.DecodeFrame(data)
	if err != nil {
		return err
	}

	switch f := frame.(type) {
	case *protocol.Authenticated:
		c.mu.Lock()
		c.token = f.Token
		c.hashID = f.HashID
		c.mu.Unlock()
		return nil
	case *protocol.Error:
		return fmt.Errorf("%s: %s", f.Code, f.Message)
	default:
		return fmt.Errorf("expected authenticated, got %T", frame)
	}
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

func (c *Client) readFrame(ctx context.Context) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		_, data, err := c.conn.ReadMessage()
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
			if f.Encrypted {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				key, err := c.getOrDeriveKey(ctx, f.SourceHash)
				cancel()
				if err == nil {
					if plain, err := crypto.OpenMessage(key, f.Body); err == nil {
						f.Body = plain
						f.Encrypted = false
					}
				}
			}
			if c.onMsg != nil {
				c.onMsg(*f)
			}
		case *protocol.Error:
			if c.onError != nil {
				c.onError(*f)
			}
		case *protocol.Authenticated:
			// Token refresh response
			c.mu.Lock()
			c.token = f.Token
			c.mu.Unlock()
		}
	}
}

func (c *Client) refreshLoop(ctx context.Context) {
	// Refresh token every 13 minutes (token lives 15 min)
	ticker := time.NewTicker(13 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.send(protocol.RefreshRequest{
				Type:  protocol.TypeRefresh,
				Token: c.Token(),
			})
		}
	}
}

// generateNonce creates a random 32-byte hex string (for testing).
func generateNonce() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
