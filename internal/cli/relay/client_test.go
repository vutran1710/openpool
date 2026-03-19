package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"
	"time"
)

func genTestKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return pub, priv
}

func TestNewClient_FieldMapping(t *testing.T) {
	pub, priv := genTestKeys()
	cfg := Config{
		RelayURL:  "ws://localhost:8081",
		PoolURL:   "owner/pool",
		IDHash:    "abcd1234abcd1234",
		MatchHash: "efgh5678efgh5678",
		Pub:       pub,
		Priv:      priv,
	}

	c := NewClient(cfg)

	if c.url != "ws://localhost:8081" {
		t.Errorf("url = %q", c.url)
	}
	if c.poolURL != "owner/pool" {
		t.Errorf("poolURL = %q", c.poolURL)
	}
	if c.idHash != "abcd1234abcd1234" {
		t.Errorf("idHash = %q", c.idHash)
	}
	if c.matchHash != "efgh5678efgh5678" {
		t.Errorf("matchHash = %q", c.matchHash)
	}
	if c.closed {
		t.Error("new client should not be closed")
	}
	if c.done == nil {
		t.Error("done channel should be initialized")
	}
	if c.keys == nil {
		t.Error("keys map should be initialized")
	}
	if c.peerKeys == nil {
		t.Error("peerKeys map should be initialized")
	}
}

func TestMatchHash_ReturnsConfigValue(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		MatchHash: "test_match_hash123",
		Pub:       pub, Priv: priv,
	})
	if c.MatchHash() != "test_match_hash123" {
		t.Errorf("MatchHash() = %q", c.MatchHash())
	}
}

func TestMatchHash_EmptyWhenNotSet(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.MatchHash() != "" {
		t.Errorf("MatchHash should be empty, got %q", c.MatchHash())
	}
}

func TestNewClient_DoneChannelOpen(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	select {
	case <-c.done:
		t.Error("done channel should be open on new client")
	default:
	}
}

func TestClose_BeforeConnect(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	c.Close()
	if !c.closed {
		t.Error("client should be marked closed")
	}
	select {
	case <-c.done:
	default:
		t.Error("done channel should be closed after Close()")
	}
}

func TestClose_Idempotent(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	c.Close()
	c.Close()
	c.Close()
}

func TestClose_ThreadSafe(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Close()
		}()
	}
	wg.Wait()
	if !c.closed {
		t.Error("client should be closed")
	}
}

func TestSendMessage_NotConnected(t *testing.T) {
	pub, priv := genTestKeys()
	// Need a valid 8-byte hex target hash
	target := hex.EncodeToString(make([]byte, 8))
	c := NewClient(Config{
		MatchHash: "test",
		Pub:       pub, Priv: priv,
	})
	// Set a peer key so key derivation doesn't fail first
	peerPub, _ := genTestKeys()
	c.SetPeerKey(target, peerPub)
	err := c.SendMessage(target, "hello")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestSendMessage_NoPeerKey(t *testing.T) {
	pub, priv := genTestKeys()
	target := hex.EncodeToString(make([]byte, 8))
	c := NewClient(Config{
		MatchHash: "test",
		Pub:       pub, Priv: priv,
	})
	err := c.SendMessage(target, "hello")
	if err == nil {
		t.Fatal("expected error when no peer key registered")
	}
}

func TestOnMessage_SetCallback(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.onMsg != nil {
		t.Error("onMsg should be nil before setting")
	}
	called := false
	c.OnMessage(func(senderMatchHash string, plaintext []byte) { called = true })
	if c.onMsg == nil {
		t.Error("onMsg should be set")
	}
	c.onMsg("sender", []byte("hello"))
	if !called {
		t.Error("callback should have been called")
	}
}

func TestOnControl_SetCallback(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.onCtrl != nil {
		t.Error("onCtrl should be nil before setting")
	}
	called := false
	c.OnControl(func(msg string) { called = true })
	if c.onCtrl == nil {
		t.Error("onCtrl should be set")
	}
	c.onCtrl("queued")
	if !called {
		t.Error("callback should have been called")
	}
}

func TestSetPeerKey_StoresKey(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	peerPub, _ := genTestKeys()
	c.SetPeerKey("peer_hash", peerPub)

	c.mu.Lock()
	stored, ok := c.peerKeys["peer_hash"]
	c.mu.Unlock()

	if !ok {
		t.Fatal("peer key not stored")
	}
	if string(stored) != string(peerPub) {
		t.Error("stored peer key does not match")
	}
}

func TestWsURL_Conversion(t *testing.T) {
	pub, priv := genTestKeys()
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8081", "ws://localhost:8081"},
		{"https://relay.example.com", "wss://relay.example.com"},
		{"ws://localhost:8081", "ws://localhost:8081"},
	}
	for _, tt := range tests {
		c := NewClient(Config{RelayURL: tt.input, Pub: pub, Priv: priv})
		got := c.wsURL()
		if got != tt.want {
			t.Errorf("wsURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConnect_Unreachable(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		RelayURL:  "http://127.0.0.1:1",
		IDHash:    "test_id_hash",
		MatchHash: "test_match_hash",
		Pub:       pub, Priv: priv,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if err == nil {
		c.Close()
		t.Fatal("expected error for unreachable server")
	}
}
