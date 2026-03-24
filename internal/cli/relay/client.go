package relay

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/openpool/internal/crypto"
)

type Client struct {
	conn      *websocket.Conn
	url       string
	poolURL   string
	idHash    string
	matchHash string
	pub       ed25519.PublicKey
	priv      ed25519.PrivateKey

	mu       sync.Mutex
	onMsg    func(senderMatchHash string, plaintext []byte)
	onCtrl   func(msg string)
	done     chan struct{}
	closed   bool

	keys     map[string][]byte           // peer_match_hash → conversation key
	peerKeys map[string]ed25519.PublicKey // peer_match_hash → peer pubkey
}

type Config struct {
	RelayURL  string
	PoolURL   string
	IDHash    string
	MatchHash string
	Pub       ed25519.PublicKey
	Priv      ed25519.PrivateKey
}

func NewClient(cfg Config) *Client {
	return &Client{
		url:       cfg.RelayURL,
		poolURL:   cfg.PoolURL,
		idHash:    cfg.IDHash,
		matchHash: cfg.MatchHash,
		pub:       cfg.Pub,
		priv:      cfg.Priv,
		done:      make(chan struct{}),
		keys:      make(map[string][]byte),
		peerKeys:  make(map[string]ed25519.PublicKey),
	}
}

func (c *Client) SetPeerKey(peerMatchHash string, peerPub ed25519.PublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.peerKeys[peerMatchHash] = peerPub
}

func (c *Client) Connect(ctx context.Context) error {
	// Extract host from relay URL for channel binding
	relayHost := extractHost(c.url)
	sig := crypto.TOTPSign(c.priv, relayHost)
	wsURL := c.wsURL() + "/ws?id=" + c.idHash + "&match=" + c.matchHash + "&sig=" + sig

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

func (c *Client) MatchHash() string {
	return c.matchHash
}

func (c *Client) OnMessage(fn func(senderMatchHash string, plaintext []byte)) {
	c.onMsg = fn
}

func (c *Client) OnControl(fn func(msg string)) {
	c.onCtrl = fn
}

func (c *Client) SendMessage(targetMatchHash string, plaintext string) error {
	key, err := c.getOrDeriveKey(targetMatchHash)
	if err != nil {
		return fmt.Errorf("deriving key: %w", err)
	}
	sealed, err := crypto.SealRaw(key, []byte(plaintext))
	if err != nil {
		return fmt.Errorf("encrypting: %w", err)
	}
	targetBytes, err := hex.DecodeString(targetMatchHash)
	if err != nil {
		return fmt.Errorf("invalid target match hash: %w", err)
	}
	frame := append(targetBytes, sealed...)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

func (c *Client) getOrDeriveKey(peerMatchHash string) ([]byte, error) {
	c.mu.Lock()
	key, ok := c.keys[peerMatchHash]
	c.mu.Unlock()
	if ok {
		return key, nil
	}

	c.mu.Lock()
	peerPub, hasPub := c.peerKeys[peerMatchHash]
	c.mu.Unlock()
	if !hasPub {
		return nil, fmt.Errorf("no pubkey for peer %s — check match notifications", peerMatchHash)
	}

	shared, err := crypto.SharedSecret(c.priv, peerPub)
	if err != nil {
		return nil, fmt.Errorf("computing shared secret: %w", err)
	}
	key, err = crypto.DeriveConversationKey(shared, c.matchHash, peerMatchHash, c.poolURL)
	if err != nil {
		return nil, fmt.Errorf("deriving conversation key: %w", err)
	}
	c.mu.Lock()
	c.keys[peerMatchHash] = key
	c.mu.Unlock()
	return key, nil
}

func extractHost(rawURL string) string {
	// Remove scheme
	host := rawURL
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	// Remove port and path
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return host
}

func (c *Client) wsURL() string {
	u := c.url
	u = strings.TrimSuffix(u, "/")
	u = strings.Replace(u, "http://", "ws://", 1)
	u = strings.Replace(u, "https://", "wss://", 1)
	return u
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		if msgType == websocket.TextMessage {
			if c.onCtrl != nil {
				c.onCtrl(string(data))
			}
			continue
		}

		if msgType != websocket.BinaryMessage || len(data) <= 8 {
			continue
		}

		senderMatchHash := hex.EncodeToString(data[:8])
		ciphertext := data[8:]

		go func() {
			key, err := c.getOrDeriveKey(senderMatchHash)
			if err != nil {
				return
			}
			plaintext, err := crypto.OpenRaw(key, ciphertext)
			if err != nil {
				return
			}
			if c.onMsg != nil {
				c.onMsg(senderMatchHash, plaintext)
			}
		}()
	}
}
