package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
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
		BinHash:   "abcd1234abcd1234",
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
	if c.binHash != "abcd1234abcd1234" {
		t.Errorf("binHash = %q", c.binHash)
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
}

func TestBinHash_ReturnsConfigValue(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		BinHash: "test_bin_hash123",
		Pub:     pub, Priv: priv,
	})
	if c.BinHash() != "test_bin_hash123" {
		t.Errorf("BinHash() = %q", c.BinHash())
	}
}

func TestBinHash_EmptyWhenNotSet(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.BinHash() != "" {
		t.Errorf("BinHash should be empty, got %q", c.BinHash())
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

func TestSend_NotConnected(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{
		BinHash:   "test",
		MatchHash: "test",
		Pub:       pub, Priv: priv,
	})
	err := c.SendMessagePlain("target", "hello")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestAck_NotConnected(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	err := c.AckMessage("msg-1")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestOnMessage_SetCallback(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.onMsg != nil {
		t.Error("onMsg should be nil before setting")
	}
	called := false
	c.OnMessage(func(m protocol.Message) { called = true })
	if c.onMsg == nil {
		t.Error("onMsg should be set")
	}
	c.onMsg(protocol.Message{})
	if !called {
		t.Error("callback should have been called")
	}
}

func TestOnError_SetCallback(t *testing.T) {
	pub, priv := genTestKeys()
	c := NewClient(Config{Pub: pub, Priv: priv})
	if c.onError != nil {
		t.Error("onError should be nil before setting")
	}
	called := false
	c.OnError(func(e protocol.Error) { called = true })
	if c.onError == nil {
		t.Error("onError should be set")
	}
	c.onError(protocol.Error{})
	if !called {
		t.Error("callback should have been called")
	}
}

func TestGenerateNonce_Length(t *testing.T) {
	n := generateNonce()
	if len(n) != 64 {
		t.Errorf("nonce length = %d, want 64", len(n))
	}
}

func TestGenerateNonce_ValidHex(t *testing.T) {
	n := generateNonce()
	if _, err := hex.DecodeString(n); err != nil {
		t.Errorf("nonce should be valid hex: %v", err)
	}
}

func TestGenerateNonce_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		n := generateNonce()
		if seen[n] {
			t.Fatalf("duplicate nonce on iteration %d", i)
		}
		seen[n] = true
	}
}

func TestCryptoSign_VerifiesWithEd25519(t *testing.T) {
	pub, priv := genTestKeys()
	nonce := generateNonce()
	nonceBytes, _ := hex.DecodeString(nonce)
	sig := crypto.Sign(priv, nonceBytes)
	sigBytes, _ := hex.DecodeString(sig)
	if !ed25519.Verify(pub, nonceBytes, sigBytes) {
		t.Error("crypto.Sign output should verify")
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
