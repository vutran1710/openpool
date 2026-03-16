package relay

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

// ==================== Integration Tests ====================
// Full WebSocket round-trip tests with a mock relay server.

// mockRelay is a test WebSocket server that speaks the relay protocol.
type mockRelay struct {
	srv      *httptest.Server
	upgrader websocket.Upgrader
	mu       sync.Mutex
	onConn   func(conn *websocket.Conn)
}

func newMockRelay(handler func(conn *websocket.Conn)) *mockRelay {
	m := &mockRelay{
		onConn: handler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := m.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		m.mu.Lock()
		h := m.onConn
		m.mu.Unlock()
		if h != nil {
			h(conn)
		}
	}))
	return m
}

func (m *mockRelay) wsURL() string {
	return "ws" + strings.TrimPrefix(m.srv.URL, "http")
}

func (m *mockRelay) close() {
	m.srv.Close()
}

// wsReadFrame reads and decodes a protocol frame from the connection.
func wsReadFrame(conn *websocket.Conn) (any, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return protocol.DecodeFrame(data)
}

// wsWriteFrame encodes and sends a protocol frame.
func wsWriteFrame(conn *websocket.Conn, v any) error {
	data, err := protocol.Encode(v)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

// doAuth performs the full auth handshake from the mock server side.
func doAuth(t *testing.T, conn *websocket.Conn, pub ed25519.PublicKey) {
	t.Helper()

	frame, err := wsReadFrame(conn)
	if err != nil {
		t.Fatalf("doAuth: read auth: %v", err)
	}
	if _, ok := frame.(*protocol.AuthRequest); !ok {
		t.Fatalf("doAuth: expected AuthRequest, got %T", frame)
	}

	nonce := generateNonce()
	if err := wsWriteFrame(conn, protocol.Challenge{
		Type:  protocol.TypeChallenge,
		Nonce: nonce,
	}); err != nil {
		t.Fatalf("doAuth: write challenge: %v", err)
	}

	frame, err = wsReadFrame(conn)
	if err != nil {
		t.Fatalf("doAuth: read response: %v", err)
	}
	resp, ok := frame.(*protocol.AuthResponse)
	if !ok {
		t.Fatalf("doAuth: expected AuthResponse, got %T", frame)
	}

	nonceBytes, _ := hex.DecodeString(nonce)
	sigBytes, _ := hex.DecodeString(resp.Signature)
	if !ed25519.Verify(pub, nonceBytes, sigBytes) {
		t.Fatal("doAuth: signature verification failed")
	}

	if err := wsWriteFrame(conn, protocol.Authenticated{
		Type:   protocol.TypeAuthenticated,
		Token:  "test-token-123",
		HashID: "abc123",
	}); err != nil {
		t.Fatalf("doAuth: write authenticated: %v", err)
	}
}

// keepAlive reads frames until connection closes (blocks the goroutine).
func keepAlive(conn *websocket.Conn) {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// --- Auth flow tests ---

func TestIntegration_Connect_FullHandshake(t *testing.T) {
	pub, priv := genTestKeys()

	var authUserID, authProvider, authPoolURL string

	relay := newMockRelay(func(conn *websocket.Conn) {
		frame, _ := wsReadFrame(conn)
		auth := frame.(*protocol.AuthRequest)
		authUserID = auth.UserID
		authProvider = auth.Provider
		authPoolURL = auth.PoolURL

		nonce := generateNonce()
		wsWriteFrame(conn, protocol.Challenge{Type: protocol.TypeChallenge, Nonce: nonce})

		frame, _ = wsReadFrame(conn)
		resp := frame.(*protocol.AuthResponse)

		nonceBytes, _ := hex.DecodeString(nonce)
		sigBytes, _ := hex.DecodeString(resp.Signature)
		if !ed25519.Verify(pub, nonceBytes, sigBytes) {
			wsWriteFrame(conn, protocol.Error{
				Type: protocol.TypeError, Code: protocol.ErrAuthFailed, Message: "bad sig",
			})
			return
		}

		wsWriteFrame(conn, protocol.Authenticated{
			Type: protocol.TypeAuthenticated, Token: "token-abc", HashID: "hash-xyz",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(),
		PoolURL:  "owner/my-pool",
		UserID:   "alice",
		Provider: "github",
		Pub:      pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	// Verify auth request fields sent correctly
	if authUserID != "alice" {
		t.Errorf("auth user_id = %q, want alice", authUserID)
	}
	if authProvider != "github" {
		t.Errorf("auth provider = %q, want github", authProvider)
	}
	if authPoolURL != "owner/my-pool" {
		t.Errorf("auth pool_url = %q, want owner/my-pool", authPoolURL)
	}

	// Verify client state after auth
	if c.HashID() != "hash-xyz" {
		t.Errorf("hashID = %q, want hash-xyz", c.HashID())
	}
	if c.Token() != "token-abc" {
		t.Errorf("token = %q, want token-abc", c.Token())
	}
}

func TestIntegration_Connect_UserNotFound(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		wsReadFrame(conn)
		wsWriteFrame(conn, protocol.Error{
			Type: protocol.TypeError, Code: protocol.ErrUserNotFound, Message: "not in pool",
		})
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "nobody", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "user_not_found") {
		t.Errorf("error = %q, want user_not_found", err)
	}
}

func TestIntegration_Connect_SignatureRejected(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		wsReadFrame(conn)
		wsWriteFrame(conn, protocol.Challenge{Type: protocol.TypeChallenge, Nonce: generateNonce()})
		wsReadFrame(conn)
		wsWriteFrame(conn, protocol.Error{
			Type: protocol.TypeError, Code: protocol.ErrAuthFailed, Message: "bad signature",
		})
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "test", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "auth_failed") {
		t.Errorf("error = %q, want auth_failed", err)
	}
}

func TestIntegration_Connect_Unreachable(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		RelayURL: "ws://127.0.0.1:1",
		PoolURL:  "owner/pool", UserID: "test", Provider: "github",
		Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for unreachable relay")
	}
	if !strings.Contains(err.Error(), "connecting to relay") {
		t.Errorf("error = %q, want 'connecting to relay'", err)
	}
}

func TestIntegration_Connect_ContextCanceled(t *testing.T) {
	pub, priv := genTestKeys()

	// Server that never responds to challenge — hangs after auth request
	relay := newMockRelay(func(conn *websocket.Conn) {
		wsReadFrame(conn)
		// Don't respond — let context expire
		time.Sleep(10 * time.Second)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "test", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
	// Should contain context deadline or canceled
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("error = %q, expected context-related error", err)
	}
}

func TestIntegration_Connect_UnexpectedFrameType(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		wsReadFrame(conn)
		// Send a Message frame instead of Challenge — protocol violation
		wsWriteFrame(conn, protocol.Message{
			Type: protocol.TypeMsg, Body: "surprise",
		})
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "test", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for unexpected frame type")
	}
	if !strings.Contains(err.Error(), "expected challenge") {
		t.Errorf("error = %q, want 'expected challenge'", err)
	}
}

// --- Messaging tests ---

func TestIntegration_SendMessage_Delivered(t *testing.T) {
	pub, priv := genTestKeys()
	received := make(chan protocol.Message, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)

		frame, err := wsReadFrame(conn)
		if err != nil {
			return
		}
		msg := frame.(*protocol.Message)
		received <- *msg
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	if err := c.SendMessagePlain("bob-hash", "hey bob!"); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case msg := <-received:
		if msg.Body != "hey bob!" {
			t.Errorf("body = %q, want 'hey bob!'", msg.Body)
		}
		if msg.TargetHash != "bob-hash" {
			t.Errorf("target = %q, want bob-hash", msg.TargetHash)
		}
		if msg.SourceHash != "abc123" {
			t.Errorf("source = %q, want abc123", msg.SourceHash)
		}
		if msg.PoolURL != "owner/pool" {
			t.Errorf("pool = %q, want owner/pool", msg.PoolURL)
		}
		if msg.Token != "test-token-123" {
			t.Errorf("token = %q, want test-token-123", msg.Token)
		}
		if msg.Ts == 0 {
			t.Error("ts should be set")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestIntegration_SendMessage_ConcurrentSafe(t *testing.T) {
	pub, priv := genTestKeys()
	var count atomic.Int32

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		for {
			_, err := wsReadFrame(conn)
			if err != nil {
				return
			}
			count.Add(1)
		}
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	// Send 20 messages concurrently — should not panic or race
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.SendMessagePlain("target", "msg")
		}(i)
	}
	wg.Wait()

	// Give mock relay time to read all
	time.Sleep(200 * time.Millisecond)

	if got := count.Load(); got != 20 {
		t.Errorf("received %d messages, want 20", got)
	}
}

func TestIntegration_AckMessage(t *testing.T) {
	pub, priv := genTestKeys()
	received := make(chan protocol.Ack, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		frame, err := wsReadFrame(conn)
		if err != nil {
			return
		}
		ack := frame.(*protocol.Ack)
		received <- *ack
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	if err := c.AckMessage("uuid-abc"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	select {
	case ack := <-received:
		if ack.MsgID != "uuid-abc" {
			t.Errorf("msg_id = %q, want uuid-abc", ack.MsgID)
		}
		if ack.Type != protocol.TypeAck {
			t.Errorf("type = %q, want ack", ack.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ack")
	}
}

// --- Callback tests ---

func TestIntegration_OnMessage_Received(t *testing.T) {
	pub, priv := genTestKeys()
	msgCh := make(chan protocol.Message, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		wsWriteFrame(conn, protocol.Message{
			Type: protocol.TypeMsg, MsgID: "inc-1",
			SourceHash: "bob", TargetHash: "abc123",
			PoolURL: "owner/pool", Body: "hey alice", Ts: 1710720000,
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	c.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	select {
	case msg := <-msgCh:
		if msg.Body != "hey alice" {
			t.Errorf("body = %q, want 'hey alice'", msg.Body)
		}
		if msg.MsgID != "inc-1" {
			t.Errorf("msg_id = %q, want inc-1", msg.MsgID)
		}
		if msg.SourceHash != "bob" {
			t.Errorf("source = %q, want bob", msg.SourceHash)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message callback")
	}
}

func TestIntegration_OnMessage_MultipleMessages(t *testing.T) {
	pub, priv := genTestKeys()
	msgCh := make(chan protocol.Message, 5)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		for i := 0; i < 5; i++ {
			wsWriteFrame(conn, protocol.Message{
				Type: protocol.TypeMsg, Body: "msg",
				SourceHash: "bob", TargetHash: "abc123", PoolURL: "p",
			})
		}
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	c.OnMessage(func(msg protocol.Message) { msgCh <- msg })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	for i := 0; i < 5; i++ {
		select {
		case <-msgCh:
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for message %d", i+1)
		}
	}
}

func TestIntegration_OnMessage_NoCallback_NoPanic(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		// Send a message with no callback set — should not crash readLoop
		wsWriteFrame(conn, protocol.Message{
			Type: protocol.TypeMsg, Body: "dropped",
			SourceHash: "bob", TargetHash: "abc123", PoolURL: "p",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	// Deliberately NOT setting OnMessage

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Wait for readLoop to process the message without crashing
	time.Sleep(200 * time.Millisecond)
	c.Close()
}

func TestIntegration_OnError_Received(t *testing.T) {
	pub, priv := genTestKeys()
	errCh := make(chan protocol.Error, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		wsWriteFrame(conn, protocol.Error{
			Type: protocol.TypeError, Code: protocol.ErrNotMatched, Message: "not matched",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	c.OnError(func(e protocol.Error) { errCh <- e })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrNotMatched {
			t.Errorf("code = %q, want not_matched", e.Code)
		}
		if e.Message != "not matched" {
			t.Errorf("message = %q, want 'not matched'", e.Message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for error callback")
	}
}

func TestIntegration_OnError_NoCallback_NoPanic(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		wsWriteFrame(conn, protocol.Error{
			Type: protocol.TypeError, Code: protocol.ErrInternal, Message: "boom",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	c.Close()
}

// --- Token refresh tests ---

func TestIntegration_TokenRefresh_ViaReadLoop(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		// Simulate token refresh response
		wsWriteFrame(conn, protocol.Authenticated{
			Type: protocol.TypeAuthenticated, Token: "refreshed-tok", HashID: "abc123",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	time.Sleep(200 * time.Millisecond)

	if c.Token() != "refreshed-tok" {
		t.Errorf("token = %q, want refreshed-tok", c.Token())
	}
}

func TestIntegration_TokenRefresh_MultipleUpdates(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)

		// Send 3 token refreshes rapidly
		for _, tok := range []string{"tok-2", "tok-3", "tok-4"} {
			wsWriteFrame(conn, protocol.Authenticated{
				Type: protocol.TypeAuthenticated, Token: tok, HashID: "abc123",
			})
			time.Sleep(50 * time.Millisecond)
		}
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	time.Sleep(500 * time.Millisecond)

	// Should have the latest token
	if c.Token() != "tok-4" {
		t.Errorf("token = %q, want tok-4", c.Token())
	}
}

// --- Close/cleanup tests ---

func TestIntegration_Close_StopsReadLoop(t *testing.T) {
	pub, priv := genTestKeys()
	serverDone := make(chan struct{})

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		// Server reads until client disconnects
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(serverDone)
				return
			}
		}
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	c.Close()

	select {
	case <-serverDone:
		// Server detected disconnect — readLoop stopped
	case <-time.After(3 * time.Second):
		t.Fatal("server did not detect client disconnect")
	}
}

func TestIntegration_Close_SendAfterClose(t *testing.T) {
	pub, priv := genTestKeys()

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	c.Close()

	// Sending after close should error, not panic
	err := c.SendMessagePlain("target", "late message")
	if err == nil {
		t.Error("expected error sending after close")
	}
}

// --- Server disconnect tests ---

func TestIntegration_ServerDisconnect_ReadLoopExits(t *testing.T) {
	pub, priv := genTestKeys()
	errCh := make(chan protocol.Error, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)
		// Close immediately after auth — simulate server crash
		conn.Close()
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	c.OnError(func(e protocol.Error) { errCh <- e })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	// readLoop should exit gracefully without panic
	time.Sleep(200 * time.Millisecond)
}

// --- Mixed frame ordering tests ---

func TestIntegration_MixedFrames_MessageThenError(t *testing.T) {
	pub, priv := genTestKeys()
	msgCh := make(chan protocol.Message, 1)
	errCh := make(chan protocol.Error, 1)

	relay := newMockRelay(func(conn *websocket.Conn) {
		doAuth(t, conn, pub)

		wsWriteFrame(conn, protocol.Message{
			Type: protocol.TypeMsg, Body: "first",
			SourceHash: "bob", TargetHash: "abc123", PoolURL: "p",
		})
		time.Sleep(50 * time.Millisecond)
		wsWriteFrame(conn, protocol.Error{
			Type: protocol.TypeError, Code: protocol.ErrRateLimited, Message: "slow down",
		})
		keepAlive(conn)
	})
	defer relay.close()

	c := NewClient(Config{
		RelayURL: relay.wsURL(), PoolURL: "owner/pool",
		UserID: "alice", Provider: "github", Pub: pub, Priv: priv,
	})
	c.OnMessage(func(msg protocol.Message) { msgCh <- msg })
	c.OnError(func(e protocol.Error) { errCh <- e })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()

	// Should receive both in order
	select {
	case msg := <-msgCh:
		if msg.Body != "first" {
			t.Errorf("body = %q, want first", msg.Body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	select {
	case e := <-errCh:
		if e.Code != protocol.ErrRateLimited {
			t.Errorf("code = %q, want rate_limited", e.Code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for error")
	}
}
